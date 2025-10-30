package main

import (
	"errors"
	"flag"
	"fmt"
	"math"
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

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *episodes <= 0 {
		return fmt.Errorf("episodes must be positive (got %d)", *episodes)
	}
	if *epsilon < 0 || *epsilon > 1 {
		return fmt.Errorf("epsilon must be between 0 and 1 (got %.2f)", *epsilon)
	}

	fmt.Printf("train config => env=%s episodes=%d seed=%d epsilon=%.2f\n", *envName, *episodes, *seed, *epsilon)

	rng := rand.New(rand.NewSource(normalizeSeed(*seed)))
	env := newGridworldEnv()
	values := newValueTable(env.rows, env.cols, 0.1)
	agent := newEpsilonGreedyAgent(rng, values, *epsilon)

	var cumulativeReward float64
	var cumulativeSteps int
	for ep := 1; ep <= *episodes; ep++ {
		reward, steps, path := runEpisode(env, agent)
		fmt.Printf("episode %d: reward=%.2f steps=%d\n", ep, reward, steps)
		cumulativeReward += reward
		cumulativeSteps += steps
		values.update(path, reward)
	}

	avgReward := cumulativeReward / float64(*episodes)
	avgSteps := float64(cumulativeSteps) / float64(*episodes)
	fmt.Printf("summary: avg_reward=%.2f avg_steps=%.2f\n", avgReward, avgSteps)
	values.print()

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
		rows:       4,
		cols:       4,
		startRow:   3,
		startCol:   0,
		goalRow:    0,
		goalCol:    3,
		maxSteps:   20,
		currRow:    3,
		currCol:    0,
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
	row, col := g.nextPosition(action)
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

func (g *gridworldEnv) nextPosition(action int) (int, int) {
	row, col := g.currRow, g.currCol
	switch action {
	case 0:
		row--
	case 1:
		col++
	case 2:
		row++
	case 3:
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
	return row, col
}

type position struct {
	row int
	col int
}

type epsilonGreedyAgent struct {
	rng     *rand.Rand
	values  *valueTable
	epsilon float64
}

func newEpsilonGreedyAgent(rng *rand.Rand, values *valueTable, epsilon float64) *epsilonGreedyAgent {
	return &epsilonGreedyAgent{rng: rng, values: values, epsilon: epsilon}
}

func (a *epsilonGreedyAgent) act(env *gridworldEnv) int {
	if a.rng.Float64() < a.epsilon {
		return a.rng.Intn(4)
	}
	bestAction := 0
	bestScore := math.Inf(-1)
	countBest := 0
	for action := 0; action < 4; action++ {
		row, col := env.nextPosition(action)
		score := a.values.get(row, col)
		if score > bestScore {
			bestScore = score
			bestAction = action
			countBest = 1
		} else if score == bestScore {
			countBest++
			if a.rng.Intn(countBest) == 0 {
				bestAction = action
			}
		}
	}
	return bestAction
}

func (a *epsilonGreedyAgent) update(reward float64) {
	_ = reward
}

func runEpisode(env *gridworldEnv, agent *epsilonGreedyAgent) (float64, int, []position) {
	env.reset()
	var total float64
	steps := 0
	path := make([]position, 0, env.maxSteps)
	for {
		action := agent.act(env)
		reward, done := env.step(action)
		agent.update(reward)
		total += reward
		steps++
		path = append(path, position{row: env.currRow, col: env.currCol})
		if done {
			break
		}
	}
	return total, steps, path
}

func normalizeSeed(seed int64) int64 {
	if seed == 0 {
		return 1
	}
	return seed
}

type valueTable struct {
	rows  int
	cols  int
	alpha float64
	data  [][]float64
}

func newValueTable(rows, cols int, alpha float64) *valueTable {
	data := make([][]float64, rows)
	for r := 0; r < rows; r++ {
		data[r] = make([]float64, cols)
	}
	return &valueTable{rows: rows, cols: cols, alpha: alpha, data: data}
}

func (v *valueTable) update(path []position, reward float64) {
	for _, pos := range path {
		current := v.data[pos.row][pos.col]
		v.data[pos.row][pos.col] = current + v.alpha*(reward-current)
	}
}

func (v *valueTable) print() {
	fmt.Println("value table:")
	for r := 0; r < v.rows; r++ {
		for c := 0; c < v.cols; c++ {
			fmt.Printf("%6.2f ", v.data[r][c])
		}
		fmt.Println()
	}
}

func (v *valueTable) get(row, col int) float64 {
	return v.data[row][col]
}
