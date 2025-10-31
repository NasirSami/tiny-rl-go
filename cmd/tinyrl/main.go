package main

import (
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
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
	alpha := fs.Float64("alpha", 0.1, "learning rate (0-1)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *episodes <= 0 {
		return fmt.Errorf("episodes must be positive (got %d)", *episodes)
	}
	if *epsilon < 0 || *epsilon > 1 {
		return fmt.Errorf("epsilon must be between 0 and 1 (got %.2f)", *epsilon)
	}
	if *alpha < 0 || *alpha > 1 {
		return fmt.Errorf("alpha must be between 0 and 1 (got %.2f)", *alpha)
	}

	fmt.Printf("train config => env=%s episodes=%d seed=%d epsilon=%.2f alpha=%.2f\n", *envName, *episodes, *seed, *epsilon, *alpha)

	rng := rand.New(rand.NewSource(normalizeSeed(*seed)))
	env := newGridworldEnv()
	values := newValueTable(env.rows, env.cols, *alpha)
	agent := newEpsilonGreedyAgent(rng, values, *epsilon)

	var cumulativeReward float64
	var cumulativeSteps int
	successCount := 0
	for ep := 1; ep <= *episodes; ep++ {
		reward, steps, path := runEpisode(env, agent)
		fmt.Printf("episode %d: reward=%.2f steps=%d\n", ep, reward, steps)
		cumulativeReward += reward
		cumulativeSteps += steps
		values.update(path, reward)
		if reward > 0 {
			successCount++
		}
	}

	avgReward := cumulativeReward / float64(*episodes)
	avgSteps := float64(cumulativeSteps) / float64(*episodes)
	successRate := float64(successCount) / float64(*episodes)
	fmt.Printf("summary: avg_reward=%.2f avg_steps=%.2f success_rate=%.2f\n", avgReward, avgSteps, successRate)
	values.print()

	return nil
}
func normalizeSeed(seed int64) int64 {
	if seed == 0 {
		return 1
	}
	return seed
}
