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

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *episodes <= 0 {
		return fmt.Errorf("episodes must be positive (got %d)", *episodes)
	}

	fmt.Printf("train config => env=%s episodes=%d seed=%d\n", *envName, *episodes, *seed)

	rng := rand.New(rand.NewSource(normalizeSeed(*seed)))
	env := newStubEnv(rng)
	agent := newStubAgent(rng)

	for ep := 1; ep <= *episodes; ep++ {
		reward, steps := runEpisode(env, agent)
		fmt.Printf("episode %d: reward=%.2f steps=%d\n", ep, reward, steps)
	}

	return nil
}

type stubEnv struct {
	rng      *rand.Rand
	maxSteps int
	current  int
}

func newStubEnv(rng *rand.Rand) *stubEnv {
	return &stubEnv{rng: rng, maxSteps: 5}
}

func (e *stubEnv) reset() {
	e.current = 0
}

func (e *stubEnv) step() (reward float64, done bool) {
	if e.current >= e.maxSteps {
		return 0, true
	}
	e.current++
	reward = e.rng.Float64() - 0.5
	return reward, e.current == e.maxSteps
}

type stubAgent struct {
	rng *rand.Rand
}

func newStubAgent(rng *rand.Rand) *stubAgent {
	return &stubAgent{rng: rng}
}

func (a *stubAgent) act() int {
	return a.rng.Intn(2)
}

func (a *stubAgent) update(reward float64) {
	_ = reward
}

func runEpisode(env *stubEnv, agent *stubAgent) (float64, int) {
	env.reset()
	var total float64
	steps := 0
	for {
		_ = agent.act()
		reward, done := env.step()
		agent.update(reward)
		total += reward
		steps++
		if done {
			break
		}
	}
	return total, steps
}

func normalizeSeed(seed int64) int64 {
	if seed == 0 {
		return 1
	}
	return seed
}
