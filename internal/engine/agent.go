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
		distance := env.potential(row, col)
		band := distanceBand(distance)
		score := a.values.get(row, col, band)
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

func (a *epsilonGreedyAgent) setEpsilon(value float64) {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	a.epsilon = value
}

func (a *epsilonGreedyAgent) softmaxValueAction(env *gridworldEnv, temperature float64) int {
	if a.values == nil || temperature <= 0 {
		return a.greedyValueAction(env)
	}
	var scores [4]float64
	var maxScore float64 = math.Inf(-1)
	for action := 0; action < 4; action++ {
		row, col := env.nextPosition(action)
		distance := env.potential(row, col)
		band := distanceBand(distance)
		score := a.values.get(row, col, band) / temperature
		scores[action] = score
		if score > maxScore {
			maxScore = score
		}
	}
	var sum float64
	for action := 0; action < 4; action++ {
		scores[action] = math.Exp(scores[action] - maxScore)
		sum += scores[action]
	}
	if sum == 0 {
		return a.greedyValueAction(env)
	}
	r := a.rng.Float64() * sum
	acc := 0.0
	for action := 0; action < 4; action++ {
		acc += scores[action]
		if r <= acc {
			a.recordVisit(env, action)
			return action
		}
	}
	return a.greedyValueAction(env)
}

func (a *epsilonGreedyAgent) resetVisits() {
	a.qVisits = make(map[actionKey]int)
	a.stateVisits = make(map[position]int)
}
