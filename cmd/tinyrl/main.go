package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"tiny-rl-go/internal/engine"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "tiny-rl-go: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return errors.New("missing subcommand; try 'train'")
	}

	subcommand := os.Args[1]
	switch subcommand {
	case "train":
		return runTrain(os.Args[2:])
	default:
		return fmt.Errorf("unknown subcommand %q", subcommand)
	}
}

func runTrain(args []string) error {
	fs := flag.NewFlagSet("train", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	envName := fs.String("env", "gridworld", "environment to train in")
	episodes := fs.Int("episodes", 1, "number of training episodes")
	seed := fs.Int64("seed", 0, "deterministic seed (0 for default)")
	epsilon := fs.Float64("epsilon", 0.1, "exploration rate (0-1)")
	epsilonMin := fs.Float64("epsilon-min", 0, "minimum exploration rate")
	epsilonDecay := fs.Float64("epsilon-decay", 1, "per-episode decay multiplier")
	alpha := fs.Float64("alpha", 0.1, "learning rate (0-1)")
	gamma := fs.Float64("gamma", 0.9, "discount factor (0-1)")
	rows := fs.Int("rows", 4, "grid rows")
	cols := fs.Int("cols", 4, "grid columns")
	stepDelay := fs.Int("step-delay", 0, "per-step delay in milliseconds")
	algorithm := fs.String("algorithm", engine.AlgorithmMonteCarlo, "training algorithm (montecarlo, q-learning, sarsa)")
	var goals goalListFlag
	fs.Func("goal", "goal specification row,col,reward (repeatable)", goals.Set)
	stepPenalty := fs.Float64("step-penalty", 0.01, "per-step penalty (non-negative)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *episodes <= 0 {
		return fmt.Errorf("episodes must be positive (got %d)", *episodes)
	}
	if *epsilon < 0 || *epsilon > 1 {
		return fmt.Errorf("epsilon must be between 0 and 1 (got %.2f)", *epsilon)
	}
	if *epsilonMin < 0 || *epsilonMin > *epsilon {
		return fmt.Errorf("epsilon-min must be between 0 and epsilon (got %.2f)", *epsilonMin)
	}
	if *epsilonDecay < 0 {
		return fmt.Errorf("epsilon-decay must be non-negative (got %.2f)", *epsilonDecay)
	}
	if *alpha < 0 || *alpha > 1 {
		return fmt.Errorf("alpha must be between 0 and 1 (got %.2f)", *alpha)
	}
	if *rows <= 0 {
		return fmt.Errorf("rows must be positive (got %d)", *rows)
	}
	if *cols <= 0 {
		return fmt.Errorf("cols must be positive (got %d)", *cols)
	}
	if *stepDelay < 0 {
		return fmt.Errorf("step-delay must be non-negative (got %d)", *stepDelay)
	}
	if *algorithm != engine.AlgorithmMonteCarlo {
		if *algorithm != engine.AlgorithmQLearning && *algorithm != engine.AlgorithmSARSA {
			return fmt.Errorf("unsupported algorithm %q", *algorithm)
		}
	}
	if *gamma < 0 || *gamma > 1 {
		return fmt.Errorf("gamma must be between 0 and 1 (got %.2f)", *gamma)
	}
	if *stepPenalty < 0 {
		return fmt.Errorf("step-penalty must be non-negative (got %.4f)", *stepPenalty)
	}

	fmt.Printf("train config => env=%s episodes=%d seed=%d epsilon=%.2f epsilonMin=%.2f epsilonDecay=%.3f alpha=%.2f gamma=%.2f rows=%d cols=%d stepDelayMs=%d stepPenalty=%.3f algorithm=%s\n", *envName, *episodes, *seed, *epsilon, *epsilonMin, *epsilonDecay, *alpha, *gamma, *rows, *cols, *stepDelay, *stepPenalty, *algorithm)

	cfg := engine.Config{
		Episodes:     *episodes,
		Seed:         *seed,
		Epsilon:      *epsilon,
		EpsilonMin:   *epsilonMin,
		EpsilonDecay: *epsilonDecay,
		Alpha:        *alpha,
		Gamma:        *gamma,
		Rows:         *rows,
		Cols:         *cols,
		StepDelayMs:  *stepDelay,
		Algorithm:    *algorithm,
		Goals:        goals.Goals,
		StepPenalty:  *stepPenalty,
	}
	trainer := engine.NewTrainer(cfg)
	ctx := context.Background()
	var (
		cumulativeReward float64
		cumulativeSteps  int
		successCount     int
		valueMap         [][]float64
	)
	for snapshot := range trainer.Run(ctx) {
		switch snapshot.Status {
		case engine.StatusRunning:
			// suppress verbose step-level output in CLI mode
		case engine.StatusEpisodeComplete:
			fmt.Printf("episode %d: reward=%.2f steps=%d\n", snapshot.Episode, snapshot.EpisodeReward, snapshot.EpisodeSteps)
			cumulativeReward = snapshot.TotalReward
			cumulativeSteps = snapshot.TotalSteps
			successCount = snapshot.SuccessCount
			valueMap = snapshot.ValueMap
		case engine.StatusDone:
			cumulativeReward = snapshot.TotalReward
			cumulativeSteps = snapshot.TotalSteps
			successCount = snapshot.SuccessCount
			valueMap = snapshot.ValueMap
		case engine.StatusCancelled:
			fmt.Println("training cancelled")
			return nil
		}
	}

	avgReward := cumulativeReward / float64(*episodes)
	avgSteps := float64(cumulativeSteps) / float64(*episodes)
	successRate := float64(successCount) / float64(*episodes)
	fmt.Printf("summary: avg_reward=%.2f avg_steps=%.2f success_rate=%.2f\n", avgReward, avgSteps, successRate)
	printValueMap(valueMap)

	return nil
}

func printValueMap(data [][]float64) {
	if len(data) == 0 {
		return
	}
	fmt.Println("value table:")
	for _, row := range data {
		for _, v := range row {
			fmt.Printf("%6.2f ", v)
		}
		fmt.Println()
	}
}

type goalListFlag struct {
	Goals []engine.Goal
}

func (g *goalListFlag) String() string {
	return fmt.Sprintf("%v", g.Goals)
}

func (g *goalListFlag) Set(value string) error {
	parts := strings.Split(value, ",")
	if len(parts) != 3 {
		return fmt.Errorf("goal must be in row,col,reward format")
	}
	row, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return fmt.Errorf("invalid goal row: %w", err)
	}
	col, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return fmt.Errorf("invalid goal col: %w", err)
	}
	reward, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	if err != nil {
		return fmt.Errorf("invalid goal reward: %w", err)
	}
	g.Goals = append(g.Goals, engine.Goal{Row: row, Col: col, Reward: reward})
	return nil
}
