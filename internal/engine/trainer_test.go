package engine

import (
	"context"
	"testing"
)

func TestMonteCarloControlSmoke(t *testing.T) {
	cfg := Config{
		Episodes:     50,
		Seed:         7,
		Algorithm:    AlgorithmMonteCarlo,
		Rows:         4,
		Cols:         4,
		GoalCount:    1,
		GoalInterval: 0,
		StepPenalty:  0.02,
		Epsilon:      0.5,
		EpsilonMin:   0.05,
		EpsilonDecay: 0.998,
		Alpha:        0.2,
		Gamma:        0.9,
	}

	trainer := NewTrainer(cfg)
	ctx := context.Background()

	var final Snapshot
	for snapshot := range trainer.Run(ctx) {
		final = snapshot
	}

	if final.EpisodesCompleted != cfg.Episodes {
		t.Fatalf("expected %d episodes completed, got %d", cfg.Episodes, final.EpisodesCompleted)
	}
	if final.SuccessCount < 1 {
		t.Fatalf("expected at least one successful episode, got %d", final.SuccessCount)
	}
	avgSteps := float64(final.TotalSteps) / float64(cfg.Episodes)
	if avgSteps >= 200 {
		t.Fatalf("expected average steps < 200, got %.2f", avgSteps)
	}
}

func TestQLearningParitySmoke(t *testing.T) {
	cfg := Config{
		Episodes:     50,
		Seed:         7,
		Algorithm:    AlgorithmQLearning,
		Rows:         4,
		Cols:         4,
		GoalCount:    1,
		GoalInterval: 0,
		StepPenalty:  0.02,
		Epsilon:      0.5,
		EpsilonMin:   0.05,
		EpsilonDecay: 0.998,
		Alpha:        0.2,
		Gamma:        0.9,
	}

	trainer := NewTrainer(cfg)
	ctx := context.Background()

	var final Snapshot
	for snapshot := range trainer.Run(ctx) {
		final = snapshot
	}

	if final.EpisodesCompleted != cfg.Episodes {
		t.Fatalf("expected %d episodes completed, got %d", cfg.Episodes, final.EpisodesCompleted)
	}
	if final.SuccessCount < 1 {
		t.Fatalf("expected at least one successful episode, got %d", final.SuccessCount)
	}
}

func TestGoalReshuffleSmoke(t *testing.T) {
	cfg := Config{
		Episodes:     12,
		Seed:         3,
		Algorithm:    AlgorithmQLearning,
		Rows:         5,
		Cols:         5,
		GoalCount:    3,
		GoalInterval: 5,
		StepPenalty:  0.01,
		Epsilon:      0.3,
		Alpha:        0.3,
		Gamma:        0.9,
	}

	trainer := NewTrainer(cfg)
	ctx := context.Background()

	reshuffles := make(map[int][]Goal)
	for snapshot := range trainer.Run(ctx) {
		if snapshot.Status == StatusEpisodeComplete {
			if snapshot.Episode == 1 || (snapshot.Episode-1)%cfg.GoalInterval == 0 {
				reshuffles[snapshot.Episode] = cloneGoals(snapshot.Goals)
			}
		}
	}

	if len(reshuffles) == 0 {
		t.Fatalf("expected at least one reshuffle recorded")
	}

	var previous []Goal
	for episode := 1; episode <= cfg.Episodes; episode++ {
		goals, ok := reshuffles[episode]
		if !ok {
			continue
		}
		if previous != nil && sameGoals(previous, goals) {
			t.Fatalf("expected goals to change at episode %d", episode)
		}
		previous = goals
	}
}

func sameGoals(a, b []Goal) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Row != b[i].Row || a[i].Col != b[i].Col || a[i].Reward != b[i].Reward {
			return false
		}
	}
	return true
}
