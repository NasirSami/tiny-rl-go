package engine

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
	qvalues *qTable
	epsilon float64
}

func newEpsilonGreedyAgent(rng *rand.Rand, values *valueTable, qvalues *qTable, epsilon float64) *epsilonGreedyAgent {
	return &epsilonGreedyAgent{rng: rng, values: values, qvalues: qvalues, epsilon: epsilon}
}

func (a *epsilonGreedyAgent) act(env *gridworldEnv) int {
	if a.rng.Float64() < a.epsilon {
		return a.rng.Intn(4)
	}
	if a.qvalues != nil {
		row, col := env.currRow, env.currCol
		bestAction := 0
		bestScore := math.Inf(-1)
		countBest := 0
		for action := 0; action < 4; action++ {
			score := a.qvalues.get(row, col, action)
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
