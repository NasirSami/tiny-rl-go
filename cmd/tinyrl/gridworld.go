package main

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
