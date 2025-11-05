package engine

import (
	"context"
	"testing"
)

func benchmarkEpisodes(b *testing.B, cfg Config) {
	for i := 0; i < b.N; i++ {
		trainer := NewTrainer(cfg)
		ctx := context.Background()
		for range trainer.Run(ctx) {
		}
	}
}

func BenchmarkEpisodeMonteCarlo(b *testing.B) {
	cfg := Config{
		Episodes:     1,
		Seed:         99,
		Algorithm:    AlgorithmMonteCarlo,
		Rows:         6,
		Cols:         6,
		GoalCount:    1,
		StepPenalty:  0.02,
		Epsilon:      0.2,
		EpsilonMin:   0.05,
		EpsilonDecay: 0.999,
		Alpha:        0.2,
		Gamma:        0.9,
	}
	benchmarkEpisodes(b, cfg)
}

func BenchmarkEpisodeQLearning(b *testing.B) {
	cfg := Config{
		Episodes:     1,
		Seed:         99,
		Algorithm:    AlgorithmQLearning,
		Rows:         6,
		Cols:         6,
		GoalCount:    1,
		StepPenalty:  0.02,
		Epsilon:      0.2,
		EpsilonMin:   0.05,
		EpsilonDecay: 0.999,
		Alpha:        0.2,
		Gamma:        0.9,
	}
	benchmarkEpisodes(b, cfg)
}
