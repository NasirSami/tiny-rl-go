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

func NewTrainer(cfg Config) *Trainer {
	if cfg.Algorithm == "" {
		cfg.Algorithm = AlgorithmMonteCarlo
	}
	switch cfg.Algorithm {
	case AlgorithmMonteCarlo, AlgorithmQLearning:
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
	env := newGridworldEnv(cfg.Rows, cfg.Cols)
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
	var path []position
	var rewards []float64
	if t.values != nil {
		path = make([]position, 0, t.env.maxSteps)
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
		state := position{row: t.env.currRow, col: t.env.currCol}
		action := t.agent.act(t.env)
		reward, done := t.env.step(action)
		t.agent.update(reward)
		episodeReward += reward
		steps++
		t.step++
		lastReward = reward
		nextState := position{row: t.env.currRow, col: t.env.currCol}
		if t.qvalues != nil {
			t.updateQLearning(state, action, reward, nextState, done)
		} else if path != nil {
			path = append(path, nextState)
			rewards = append(rewards, reward)
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
	}
	if episodeReward > 0 {
		t.successCount++
	}
	t.updateMonteCarloValues(path, rewards)
	t.totalReward += episodeReward
	t.totalSteps += steps
	t.episodesCompleted++
	out <- t.snapshot(StatusEpisodeComplete, episode, steps, episodeReward, lastReward)
}

func (t *Trainer) updateMonteCarloValues(path []position, rewards []float64) {
	if t.values == nil || len(path) == 0 {
		return
	}
	seen := make(map[position]bool, len(path))
	returnSoFar := 0.0
	for i := len(path) - 1; i >= 0; i-- {
		returnSoFar += rewards[i]
		state := path[i]
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
		SuccessCount:      t.successCount,
		EpisodesCompleted: t.episodesCompleted,
		TotalReward:       t.totalReward,
		TotalSteps:        t.totalSteps,
		Config:            t.cfg,
		Status:            status,
	}
}
