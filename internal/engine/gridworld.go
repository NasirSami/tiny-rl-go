package engine

type gridworldEnv struct {
	rows, cols   int
	startRow     int
	startCol     int
	maxSteps     int
	currRow      int
	currCol      int
	stepsTaken   int
	goals        []Goal
	initialGoals []Goal
	stepPenalty  float64
}

const timeoutPenaltyMultiplier = 5.0

func newGridworldEnv(rows, cols int, goals []Goal, stepPenalty float64, overrideMaxSteps int) *gridworldEnv {
	if rows <= 0 {
		rows = 1
	}
	if cols <= 0 {
		cols = 1
	}
	baseSteps := rows * cols * 5 / 2
	minSteps := rows + cols
	if baseSteps < minSteps {
		baseSteps = minSteps
	}
	maxSteps := baseSteps
	if overrideMaxSteps > 0 {
		maxSteps = overrideMaxSteps
	}
	if maxSteps < 10 {
		maxSteps = 10
	}
	startRow := rows - 1
	if startRow < 0 {
		startRow = 0
	}
	initial := cloneGoalSlice(goals)
	return &gridworldEnv{
		rows:         rows,
		cols:         cols,
		startRow:     startRow,
		startCol:     0,
		maxSteps:     maxSteps,
		currRow:      startRow,
		currCol:      0,
		stepsTaken:   0,
		goals:        cloneGoalSlice(goals),
		initialGoals: initial,
		stepPenalty:  stepPenalty,
	}
}

func (g *gridworldEnv) reset() {
	g.currRow = g.startRow
	g.currCol = g.startCol
	g.stepsTaken = 0
	g.goals = cloneGoalSlice(g.initialGoals)
}

func (g *gridworldEnv) setGoals(goals []Goal) {
	g.initialGoals = cloneGoalSlice(goals)
	g.goals = cloneGoalSlice(goals)
}

func (g *gridworldEnv) setStepPenalty(p float64) {
	if p < 0 {
		p = 0
	}
	g.stepPenalty = p
}

func (g *gridworldEnv) step(action int) (float64, bool) {
	if g.stepsTaken >= g.maxSteps {
		return 0, true
	}
	row, col := g.nextPosition(action)
	g.currRow = row
	g.currCol = col
	g.stepsTaken++
	reward := -g.stepPenalty
	collected := false
	for i, goal := range g.goals {
		if g.currRow == goal.Row && g.currCol == goal.Col {
			reward += goal.Reward
			g.goals = append(g.goals[:i], g.goals[i+1:]...)
			collected = true
			break
		}
	}
	if collected && len(g.goals) == 0 {
		return reward, true
	}
	if g.stepsTaken >= g.maxSteps {
		reward -= g.stepPenalty * timeoutPenaltyMultiplier
		return reward, true
	}
	return reward, false
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

func (g *gridworldEnv) potential(row, col int) float64 {
	if len(g.goals) == 0 {
		return 0
	}
	minDist := g.rows + g.cols
	for _, goal := range g.goals {
		d := absInt(goal.Row-row) + absInt(goal.Col-col)
		if d < minDist {
			minDist = d
		}
	}
	return float64(minDist)
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func cloneGoalSlice(goals []Goal) []Goal {
	if len(goals) == 0 {
		return nil
	}
	copied := make([]Goal, len(goals))
	copy(copied, goals)
	return copied
}
