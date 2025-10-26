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
	env := newGridworldEnv()
	agent := newStubAgent(rng)

	var cumulativeReward float64
	var cumulativeSteps int
	for ep := 1; ep <= *episodes; ep++ {
		reward, steps := runEpisode(env, agent)
		fmt.Printf("episode %d: reward=%.2f steps=%d\n", ep, reward, steps)
		cumulativeReward += reward
		cumulativeSteps += steps
	}

	avgReward := cumulativeReward / float64(*episodes)
	avgSteps := float64(cumulativeSteps) / float64(*episodes)
	fmt.Printf("summary: avg_reward=%.2f avg_steps=%.2f\n", avgReward, avgSteps)

	return nil
}

type gridworldEnv struct {
	rows, cols int
	startRow   int
	startCol   int
	goalRow    int
	goalCol    int
	maxSteps   int
	currRow    int
	currCol    int
	stepsTaken int
}

func newGridworldEnv() *gridworldEnv {
	return &gridworldEnv{
		rows:      4,
		cols:      4,
		startRow:  3,
		startCol:  0,
		goalRow:   0,
		goalCol:   3,
		maxSteps:  20,
		currRow:   3,
		currCol:   0,
		stepsTaken: 0,
	}
}

func (g *gridworldEnv) reset() {
	g.currRow = g.startRow
	g.currCol = g.startCol
	g.stepsTaken = 0
}

func (g *gridworldEnv) step(action int) (float64, bool) {
	if g.stepsTaken >= g.maxSteps {
		return 0, true
	}
	row, col := g.currRow, g.currCol
	switch action {
	case 0: // up
		row--
	case 1: // right
		col++
	case 2: // down
		row++
	case 3: // left
		col--
	}
	if row < 0 {
		row = 0
	}
	if row >= g.rows {
		row = g.rows - 1
	}
	if col < 0 {
		col = 0
	}
	if col >= g.cols {
		col = g.cols - 1
	}
	g.currRow = row
	g.currCol = col
	g.stepsTaken++
	if g.currRow == g.goalRow && g.currCol == g.goalCol {
		return 1, true
	}
	if g.stepsTaken >= g.maxSteps {
		return 0, true
	}
	return 0, false
}

type stubAgent struct {
	rng *rand.Rand
}

func newStubAgent(rng *rand.Rand) *stubAgent {
	return &stubAgent{rng: rng}
}

func (a *stubAgent) act(numActions int) int {
	return a.rng.Intn(numActions)
}

func (a *stubAgent) update(reward float64) {
	_ = reward
}

func runEpisode(env *gridworldEnv, agent *stubAgent) (float64, int) {
	env.reset()
	var total float64
	steps := 0
	for {
		action := agent.act(4)
		reward, done := env.step(action)
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
