package main

import (
	"math"
	"math/rand"
)

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
