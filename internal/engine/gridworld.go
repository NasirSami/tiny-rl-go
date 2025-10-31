package engine

type gridworldEnv struct {
	rows, cols int
	startRow   int
	startCol   int
	maxSteps   int
	currRow    int
	currCol    int
	stepsTaken int
	goals      []Goal
}

func newGridworldEnv(rows, cols int, goals []Goal) *gridworldEnv {
	if rows <= 0 {
		rows = 1
	}
	if cols <= 0 {
		cols = 1
	}
	maxSteps := rows * cols * 4
	if maxSteps < 10 {
		maxSteps = 10
	}
	startRow := rows - 1
	if startRow < 0 {
		startRow = 0
	}
	return &gridworldEnv{
		rows:       rows,
		cols:       cols,
		startRow:   startRow,
		startCol:   0,
		maxSteps:   maxSteps,
		currRow:    startRow,
		currCol:    0,
		stepsTaken: 0,
		goals:      goals,
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
	for _, goal := range g.goals {
		if g.currRow == goal.Row && g.currCol == goal.Col {
			return goal.Reward, true
		}
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
