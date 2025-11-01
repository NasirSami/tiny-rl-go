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
	rng         *rand.Rand
	values      *valueTable
	qvalues     *qTable
	epsilon     float64
	qVisits     map[actionKey]int
	stateVisits map[position]int
}

func newEpsilonGreedyAgent(rng *rand.Rand, values *valueTable, qvalues *qTable, epsilon float64) *epsilonGreedyAgent {
	return &epsilonGreedyAgent{
		rng:         rng,
		values:      values,
		qvalues:     qvalues,
		epsilon:     epsilon,
		qVisits:     make(map[actionKey]int),
		stateVisits: make(map[position]int),
	}
}

func (a *epsilonGreedyAgent) act(env *gridworldEnv) int {
	var chosen int
	if a.rng.Float64() < a.epsilon {
		chosen = a.rng.Intn(4)
	} else if a.qvalues != nil {
		chosen = a.greedyQAction(env)
	} else {
		chosen = a.greedyValueAction(env)
	}
	a.recordVisit(env, chosen)
	return chosen
}

func (a *epsilonGreedyAgent) update(reward float64) {
	_ = reward
}

type actionKey struct {
	row    int
	col    int
	action int
}

type candidate struct {
	action int
	visits int
}

func (a *epsilonGreedyAgent) greedyQAction(env *gridworldEnv) int {
	row, col := env.currRow, env.currCol
	bestScore := math.Inf(-1)
	var candidates []candidate
	for action := 0; action < 4; action++ {
		score := a.qvalues.get(row, col, action)
		if score > bestScore {
			bestScore = score
			candidates = candidates[:0]
			candidates = append(candidates, candidate{action: action, visits: a.qVisits[actionKey{row, col, action}]})
		} else if score == bestScore {
			candidates = append(candidates, candidate{action: action, visits: a.qVisits[actionKey{row, col, action}]})
		}
	}
	return pickLeastVisited(candidates, a.rng)
}

func (a *epsilonGreedyAgent) greedyValueAction(env *gridworldEnv) int {
	bestScore := math.Inf(-1)
	var candidates []candidate
	for action := 0; action < 4; action++ {
		row, col := env.nextPosition(action)
		score := a.values.get(row, col)
		if score > bestScore {
			bestScore = score
			candidates = candidates[:0]
			candidates = append(candidates, candidate{action: action, visits: a.stateVisits[position{row: row, col: col}]})
		} else if score == bestScore {
			candidates = append(candidates, candidate{action: action, visits: a.stateVisits[position{row: row, col: col}]})
		}
	}
	return pickLeastVisited(candidates, a.rng)
}

func pickLeastVisited(candidates []candidate, rng *rand.Rand) int {
	if len(candidates) == 0 {
		return 0
	}
	least := int(^uint(0) >> 1)
	var options []int
	for _, candidate := range candidates {
		if candidate.visits < least {
			least = candidate.visits
			options = options[:0]
			options = append(options, candidate.action)
		} else if candidate.visits == least {
			options = append(options, candidate.action)
		}
	}
	return options[rng.Intn(len(options))]
}

func (a *epsilonGreedyAgent) recordVisit(env *gridworldEnv, action int) {
	if a.qvalues != nil {
		key := actionKey{row: env.currRow, col: env.currCol, action: action}
		a.qVisits[key]++
		return
	}
	row, col := env.nextPosition(action)
	a.stateVisits[position{row: row, col: col}]++
}
