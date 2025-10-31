package engine

import (
	"context"
	"math/rand"
	"time"
)

const (
	StatusRunning         = "running"
	StatusEpisodeComplete = "episode_complete"
	StatusDone            = "done"
	StatusCancelled       = "cancelled"
)

const (
	AlgorithmMonteCarlo = "montecarlo"
	AlgorithmQLearning  = "q-learning"
	AlgorithmSARSA      = "sarsa"
)

type Config struct {
	Episodes    int
	Seed        int64
	Epsilon     float64
	Alpha       float64
	Rows        int
	Cols        int
	StepDelayMs int
	Gamma       float64
	Algorithm   string
	Goals       []Goal
}

type Position struct {
	Row int
	Col int
}

type Snapshot struct {
	Step              int
	Episode           int
	EpisodeSteps      int
	EpisodeReward     float64
	Reward            float64
	Position          Position
	ValueMap          [][]float64
	Goals             []Goal
	SuccessCount      int
	EpisodesCompleted int
	TotalReward       float64
	TotalSteps        int
	Config            Config
	Status            string
}

type Trainer struct {
	cfg               Config
	rng               *rand.Rand
	env               *gridworldEnv
	agent             *epsilonGreedyAgent
	values            *valueTable
	qvalues           *qTable
	step              int
	successCount      int
	episodesCompleted int
	totalReward       float64
	totalSteps        int
}

type Goal struct {
	Row     int
	Col     int
	Reward  float64
}

func NewTrainer(cfg Config) *Trainer {
	if cfg.Algorithm == "" {
		cfg.Algorithm = AlgorithmMonteCarlo
	}
	switch cfg.Algorithm {
	case AlgorithmMonteCarlo, AlgorithmQLearning, AlgorithmSARSA:
		// allowed
	default:
		cfg.Algorithm = AlgorithmMonteCarlo
	}
	if cfg.Rows <= 0 {
		cfg.Rows = 4
	}
	if cfg.Cols <= 0 {
		cfg.Cols = 4
	}
	if cfg.StepDelayMs < 0 {
		cfg.StepDelayMs = 0
	}
	if cfg.Gamma <= 0 || cfg.Gamma > 1 {
		cfg.Gamma = 0.9
	}
	seed := cfg.Seed
	if seed == 0 {
		seed = 1
	}
	rng := rand.New(rand.NewSource(seed))
	sanitizedGoals := sanitizeGoals(cfg.Goals, cfg.Rows, cfg.Cols)
	if len(sanitizedGoals) == 0 {
		sanitizedGoals = []Goal{{Row: 0, Col: cfg.Cols - 1, Reward: 1}}
	}
	cfg.Goals = cloneGoals(sanitizedGoals)
	env := newGridworldEnv(cfg.Rows, cfg.Cols, sanitizedGoals)
	var (
		values  *valueTable
		qvalues *qTable
	)
	if cfg.Algorithm == AlgorithmMonteCarlo {
		values = newValueTable(env.rows, env.cols, cfg.Alpha)
	} else {
		qvalues = newQTable(env.rows, env.cols, 4)
	}
	agent := newEpsilonGreedyAgent(rng, values, qvalues, cfg.Epsilon)
	return &Trainer{
		cfg:     cfg,
		rng:     rng,
		env:     env,
		agent:   agent,
		values:  values,
		qvalues: qvalues,
	}
}

func sanitizeGoals(goals []Goal, rows, cols int) []Goal {
	result := make([]Goal, 0, len(goals))
	for _, g := range goals {
		if g.Reward == 0 {
			continue
		}
		row := g.Row
		col := g.Col
		if row < 0 || row >= rows || col < 0 || col >= cols {
			continue
		}
		result = append(result, Goal{Row: row, Col: col, Reward: g.Reward})
	}
	return result
}

func (t *Trainer) Run(ctx context.Context) <-chan Snapshot {
	out := make(chan Snapshot)
	go func() {
		defer close(out)
		if t.cfg.Episodes <= 0 {
			return
		}
		for episode := 1; episode <= t.cfg.Episodes; episode++ {
			select {
			case <-ctx.Done():
				out <- t.snapshot(StatusCancelled, episode, 0, 0, 0)
				return
			default:
			}
			t.runEpisode(ctx, episode, out)
		}
		out <- t.snapshot(StatusDone, t.cfg.Episodes, 0, 0, 0)
	}()
	return out
}

func (t *Trainer) runEpisode(ctx context.Context, episode int, out chan<- Snapshot) {
	t.env.reset()
	state := position{row: t.env.currRow, col: t.env.currCol}
	action := t.agent.act(t.env)
	var states []position
	var rewards []float64
	if t.values != nil {
		states = append(states, state)
		rewards = make([]float64, 0, t.env.maxSteps)
	}
	steps := 0
	episodeReward := 0.0
	var lastReward float64
	for {
		select {
		case <-ctx.Done():
			out <- t.snapshot(StatusCancelled, episode, steps, episodeReward, lastReward)
			return
		default:
		}
		reward, done := t.env.step(action)
		t.agent.update(reward)
		episodeReward += reward
		steps++
		t.step++
		lastReward = reward
		nextState := position{row: t.env.currRow, col: t.env.currCol}
		var nextAction int
		if t.qvalues != nil {
			if t.cfg.Algorithm == AlgorithmQLearning {
				t.updateQLearning(state, action, reward, nextState, done)
			} else if t.cfg.Algorithm == AlgorithmSARSA {
				if !done {
					nextAction = t.agent.act(t.env)
				}
				t.updateSARSA(state, action, reward, nextState, nextAction, done)
			}
		} else {
			rewards = append(rewards, reward)
			states = append(states, nextState)
		}
		out <- t.snapshot(StatusRunning, episode, steps, episodeReward, reward)
		if t.cfg.StepDelayMs > 0 {
			select {
			case <-ctx.Done():
				out <- t.snapshot(StatusCancelled, episode, steps, episodeReward, reward)
				return
			case <-time.After(time.Duration(t.cfg.StepDelayMs) * time.Millisecond):
			}
		}
		if done {
			break
		}
		state = nextState
		if t.qvalues != nil {
			if t.cfg.Algorithm == AlgorithmQLearning {
				action = t.agent.act(t.env)
			} else {
				action = nextAction
			}
		} else {
			action = t.agent.act(t.env)
		}
	}
	if episodeReward > 0 {
		t.successCount++
	}
	t.updateMonteCarloValues(states, rewards)
	t.totalReward += episodeReward
	t.totalSteps += steps
	t.episodesCompleted++
	out <- t.snapshot(StatusEpisodeComplete, episode, steps, episodeReward, lastReward)
}

func (t *Trainer) updateMonteCarloValues(states []position, rewards []float64) {
	if t.values == nil || len(states) == 0 || len(rewards) == 0 {
		return
	}
	seen := make(map[position]bool, len(states))
	returnSoFar := 0.0
	for i := len(rewards) - 1; i >= 0; i-- {
		returnSoFar += rewards[i]
		state := states[i]
		if seen[state] {
			continue
		}
		seen[state] = true
		current := t.values.data[state.row][state.col]
		t.values.data[state.row][state.col] = current + t.cfg.Alpha*(returnSoFar-current)
	}
}

func (t *Trainer) updateQLearning(state position, action int, reward float64, next position, done bool) {
	if t.qvalues == nil {
		return
	}
	current := t.qvalues.get(state.row, state.col, action)
	var nextValue float64
	if !done {
		nextValue = t.qvalues.maxValue(next.row, next.col)
	}
	target := reward + t.cfg.Gamma*nextValue
	updated := current + t.cfg.Alpha*(target-current)
	t.qvalues.set(state.row, state.col, action, updated)
}

func (t *Trainer) updateSARSA(state position, action int, reward float64, next position, nextAction int, done bool) {
	if t.qvalues == nil {
		return
	}
	current := t.qvalues.get(state.row, state.col, action)
	var nextValue float64
	if !done {
		nextValue = t.qvalues.get(next.row, next.col, nextAction)
	}
	target := reward + t.cfg.Gamma*nextValue
	updated := current + t.cfg.Alpha*(target-current)
	t.qvalues.set(state.row, state.col, action, updated)
}

func (t *Trainer) snapshot(status string, episode, episodeSteps int, episodeReward, reward float64) Snapshot {
	var valueMap [][]float64
	if t.values != nil {
		valueMap = t.values.cloneData()
	} else if t.qvalues != nil {
		valueMap = t.qvalues.stateValues()
	}
	return Snapshot{
		Step:              t.step,
		Episode:           episode,
		EpisodeSteps:      episodeSteps,
		EpisodeReward:     episodeReward,
		Reward:            reward,
		Position:          Position{Row: t.env.currRow, Col: t.env.currCol},
		ValueMap:          valueMap,
		Goals:             cloneGoals(t.env.goals),
		SuccessCount:      t.successCount,
		EpisodesCompleted: t.episodesCompleted,
		TotalReward:       t.totalReward,
		TotalSteps:        t.totalSteps,
		Config:            t.cfg,
		Status:            status,
	}
}

func cloneGoals(goals []Goal) []Goal {
	if len(goals) == 0 {
		return nil
	}
	copyGoals := make([]Goal, len(goals))
	copy(copyGoals, goals)
	return copyGoals
}
