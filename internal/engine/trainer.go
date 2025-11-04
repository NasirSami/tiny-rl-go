package engine

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// ScaledStepPenalty downscales the per-step penalty on large grids so the default goal reward remains meaningful.
// Uses the Manhattan distance from start to goal as a proxy for the shortest path length and clamps the scaling
// factor to avoid extreme penalties on oversized or tiny boards.
func ScaledStepPenalty(rows, cols int, base float64) float64 {
	if base <= 0 {
		return 0
	}
	pathLen := float64(rows + cols - 2)
	if pathLen < 1 {
		pathLen = 1
	}
	reference := 6.0 // matches the 3+3 path of the default 4x4 board
	scale := pathLen / reference
	if scale < 0.5 {
		scale = 0.5
	}
	if scale > 3 {
		scale = 3
	}
	return base / scale
}

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

const (
	distanceRewardScale  = 0.1
	distancePenaltyScale = 0.2
)

type Config struct {
	Episodes              int
	Seed                  int64
	Epsilon               float64
	EpsilonMin            float64
	EpsilonDecay          float64
	Alpha                 float64
	Rows                  int
	Cols                  int
	StepDelayMs           int
	MaxSteps              int
	Gamma                 float64
	Algorithm             string
	Goals                 []Goal
	StepPenalty           float64
	RandomStart           bool
	DumpTrajectory        bool
	GoalCount             int
	GoalInterval          int
	SoftmaxTemperature    float64
	SoftmaxMinTemperature float64
	Lambda                float64
	WarmupEpisodes        int
	WarmupStepPenalty     float64
	FeatureMapper         FeatureMapper
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
	baseStepPenalty   float64
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
	Row    int
	Col    int
	Reward float64
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
	if cfg.MaxSteps < 0 {
		cfg.MaxSteps = 0
	}
	if cfg.Gamma <= 0 || cfg.Gamma > 1 {
		cfg.Gamma = 0.9
	}
	if cfg.StepPenalty < 0 {
		cfg.StepPenalty = 0
	}
	if cfg.Epsilon <= 0 || cfg.Epsilon > 1 {
		cfg.Epsilon = 0.1
	}
	if cfg.EpsilonMin < 0 || cfg.EpsilonMin > cfg.Epsilon {
		cfg.EpsilonMin = 0
	}
	if cfg.EpsilonDecay < 0 {
		cfg.EpsilonDecay = 0
	}
	if cfg.GoalCount < 0 {
		cfg.GoalCount = 0
	}
	if cfg.GoalInterval < 0 {
		cfg.GoalInterval = 0
	}
	if cfg.SoftmaxTemperature <= 0 {
		cfg.SoftmaxTemperature = 1
	}
	if cfg.SoftmaxMinTemperature < 0 {
		cfg.SoftmaxMinTemperature = 0
	}
	if cfg.SoftmaxMinTemperature > cfg.SoftmaxTemperature {
		cfg.SoftmaxMinTemperature = cfg.SoftmaxTemperature
	}
	if cfg.Lambda < 0 || cfg.Lambda > 1 {
		cfg.Lambda = 0.9
	}
	if cfg.WarmupEpisodes < 0 {
		cfg.WarmupEpisodes = 0
	}
	if cfg.WarmupStepPenalty < 0 {
		cfg.WarmupStepPenalty = 0
	}
	seed := cfg.Seed
	if seed == 0 {
		seed = 1
	}
	rng := rand.New(rand.NewSource(seed))
	sanitizedGoals := sanitizeGoals(cfg.Goals, cfg.Rows, cfg.Cols)
	if cfg.GoalCount > 0 {
		sanitizedGoals = autoPlaceGoals(cfg.Rows, cfg.Cols, cfg.GoalCount)
	} else if len(sanitizedGoals) == 0 {
		reward := maxFloat(1, float64(cfg.Rows+cfg.Cols-2)/2.5)
		sanitizedGoals = []Goal{{Row: 0, Col: cfg.Cols - 1, Reward: reward}}
	}
	cfg.Goals = cloneGoals(sanitizedGoals)
	effectivePenalty := ScaledStepPenalty(cfg.Rows, cfg.Cols, cfg.StepPenalty)
	cfg.StepPenalty = effectivePenalty
	env := newGridworldEnv(cfg.Rows, cfg.Cols, sanitizedGoals, effectivePenalty, cfg.MaxSteps)
	var (
		values  *valueTable
		qvalues *qTable
	)

	mapper := cfg.FeatureMapper
	if mapper == nil {
		mapper = DistanceBands3Mapper{}
	}

	qvalues = newQTable(env.rows, env.cols, 4)
	agent := newEpsilonGreedyAgent(rng, values, qvalues, cfg.Epsilon)
	return &Trainer{
		cfg:             cfg,
		baseStepPenalty: effectivePenalty,
		rng:             rng,
		env:             env,
		agent:           agent,
		values:          values,
		qvalues:         qvalues,
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

func autoPlaceGoals(rows, cols, count int) []Goal {
	if rows <= 0 || cols <= 0 {
		return nil
	}
	if count <= 0 {
		reward := maxFloat(1, float64(rows+cols-2)/2.5)
		return []Goal{{Row: 0, Col: cols - 1, Reward: reward}}
	}
	reward := maxFloat(1, float64(rows+cols-2)/2.5)
	total := rows * cols
	if count > total {
		count = total
	}
	interval := total / count
	if interval == 0 {
		interval = 1
	}
	goals := make([]Goal, 0, count)
	index := 0
	for i := 0; i < total && len(goals) < count; i++ {
		if i%interval != 0 {
			continue
		}
		row := i / cols
		col := i % cols
		goals = append(goals, Goal{Row: row, Col: col, Reward: reward})
		index++
	}
	return goals
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
			if t.cfg.EpsilonDecay > 0 {
				currentEps := clampFloat(t.cfg.Epsilon, 0, 1)
				t.agent.setEpsilon(currentEps)
			}
			t.runEpisode(ctx, episode, out)
			if t.cfg.EpsilonDecay > 0 {
				t.cfg.Epsilon = maxFloat(t.cfg.EpsilonMin, t.cfg.Epsilon*t.cfg.EpsilonDecay)
			}
		}
		out <- t.snapshot(StatusDone, t.cfg.Episodes, 0, 0, 0)
	}()
	return out
}

func (t *Trainer) runEpisode(ctx context.Context, episode int, out chan<- Snapshot) {
	if t.cfg.Algorithm == AlgorithmMonteCarlo {
		t.applyWarmupPenalty(episode)
		t.agent.resetVisits()
	}
	if t.cfg.GoalCount > 0 && t.cfg.GoalInterval > 0 {
		shouldShuffle := episode == 1 || (episode-1)%t.cfg.GoalInterval == 0
		if shouldShuffle {
			newGoals := autoPlaceGoals(t.env.rows, t.env.cols, t.cfg.GoalCount)
			t.env.setGoals(newGoals)
			t.cfg.Goals = cloneGoals(newGoals)
		}
	}
	t.env.reset()
	if t.cfg.RandomStart {
		t.applyRandomStart()
	}
	state := position{row: t.env.currRow, col: t.env.currCol}
	action := t.agent.act(t.env)
	var mcStates []position
	var mcActions []int
	var mcRewards []float64
	if t.cfg.Algorithm == AlgorithmMonteCarlo {
		mcStates = append(mcStates, state)
		mcActions = append(mcActions, action)
		mcRewards = make([]float64, 0, t.env.maxSteps)
	}
	visits := make(map[position]int, t.env.rows*t.env.cols)
	visits[state]++
	steps := 0
	episodeReward := 0.0
	var lastReward float64
	goalReached := false
	for {
		select {
		case <-ctx.Done():
			out <- t.snapshot(StatusCancelled, episode, steps, episodeReward, lastReward)
			return
		default:
		}
		prevDistance := t.env.potential(state.row, state.col)
		baseReward, done := t.env.step(action)
		nextState := position{row: t.env.currRow, col: t.env.currCol}
		newDistance := t.env.potential(nextState.row, nextState.col)
		reward := baseReward
		delta := prevDistance - newDistance
		reward += 0.1 * delta
		if done && len(t.env.goals) == 0 {
			goalReached = true
		}
		t.agent.update(reward)
		episodeReward += reward
		steps++
		t.step++
		lastReward = reward
		var nextAction int
		switch t.cfg.Algorithm {
		case AlgorithmQLearning:
			t.updateQLearning(state, action, reward, nextState, done)
		case AlgorithmSARSA:
			if !done {
				nextAction = t.agent.act(t.env)
			}
			t.updateSARSA(state, action, reward, nextState, nextAction, done)
		case AlgorithmMonteCarlo:
			mcRewards = append(mcRewards, reward)
			if !done {
				nextAction = t.agent.act(t.env)
				mcStates = append(mcStates, nextState)
				mcActions = append(mcActions, nextAction)
			}
		}
		visits[nextState]++
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
		switch t.cfg.Algorithm {
		case AlgorithmQLearning:
			action = t.agent.act(t.env)
		case AlgorithmSARSA:
			action = nextAction
		case AlgorithmMonteCarlo:
			action = nextAction
		default:
			action = t.agent.act(t.env)
		}
	}
	if goalReached {
		t.successCount++
	}
	if t.cfg.Algorithm == AlgorithmMonteCarlo {
		t.updateMonteCarloQ(mcStates, mcActions, mcRewards)
	}
	t.totalReward += episodeReward
	t.totalSteps += steps
	t.episodesCompleted++
	t.printVisitHeatmap(episode, visits)
	out <- t.snapshot(StatusEpisodeComplete, episode, steps, episodeReward, lastReward)
}

func (t *Trainer) updateMonteCarloQ(states []position, actions []int, rewards []float64) {
	if t.qvalues == nil {
		return
	}
	if len(states) == 0 || len(actions) == 0 || len(rewards) == 0 {
		return
	}
	if len(states) != len(actions) {
		return
	}
	if len(rewards) != len(actions) {
		return
	}
	type visitKey struct {
		row    int
		col    int
		action int
	}
	seen := make(map[visitKey]bool, len(actions))
	G := 0.0
	for i := len(rewards) - 1; i >= 0; i-- {
		G = rewards[i] + t.cfg.Gamma*G
		state := states[i]
		action := actions[i]
		key := visitKey{row: state.row, col: state.col, action: action}
		if seen[key] {
			continue
		}
		seen[key] = true
		current := t.qvalues.get(state.row, state.col, action)
		updated := current + t.cfg.Alpha*(G-current)
		t.qvalues.set(state.row, state.col, action, updated)
	}
}

func (t *Trainer) applyWarmupPenalty(episode int) {
	if t.env == nil {
		return
	}
	penalty := t.baseStepPenalty
	if t.cfg.WarmupEpisodes > 0 && episode <= t.cfg.WarmupEpisodes {
		if t.cfg.WarmupStepPenalty > 0 {
			penalty = t.cfg.WarmupStepPenalty
		}
	}
	if penalty < 0 {
		penalty = 0
	}
	t.env.setStepPenalty(penalty)
}

func (t *Trainer) applyRandomStart() {
	if t.env == nil || t.rng == nil {
		return
	}
	if t.env.rows <= 0 || t.env.cols <= 0 {
		return
	}
	t.env.currRow = t.rng.Intn(t.env.rows)
	t.env.currCol = t.rng.Intn(t.env.cols)
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

func (t *Trainer) printVisitHeatmap(episode int, visits map[position]int) {
	if visits == nil {
		return
	}
	fmt.Printf("visit heatmap (episode %d)\n", episode)
	for r := 0; r < t.env.rows; r++ {
		for c := 0; c < t.env.cols; c++ {
			count := visits[position{row: r, col: c}]
			if count == 0 {
				fmt.Printf("  . ")
			} else {
				fmt.Printf("%3d ", count)
			}
		}
		fmt.Println()
	}
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
