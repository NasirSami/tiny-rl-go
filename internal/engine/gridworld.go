package engine

import "math/rand"

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
	tiles        map[position]tile
	rng          *rand.Rand
}

type tileKind int

const (
	tileEmpty tileKind = iota
	tileWall
	tileSlip
)

type tile struct {
	kind     tileKind
	slipProb float64
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
		tiles:        make(map[position]tile),
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

func (g *gridworldEnv) setRandomSource(r *rand.Rand) {
	g.rng = r
}

func (g *gridworldEnv) setWall(row, col int) {
	if row < 0 || row >= g.rows || col < 0 || col >= g.cols {
		return
	}
	if g.tiles == nil {
		g.tiles = make(map[position]tile)
	}
	g.tiles[position{row: row, col: col}] = tile{kind: tileWall}
}

func (g *gridworldEnv) setSlipTile(row, col int, probability float64) {
	if row < 0 || row >= g.rows || col < 0 || col >= g.cols {
		return
	}
	if probability < 0 {
		probability = 0
	}
	if probability > 1 {
		probability = 1
	}
	if g.tiles == nil {
		g.tiles = make(map[position]tile)
	}
	g.tiles[position{row: row, col: col}] = tile{kind: tileSlip, slipProb: probability}
}

func (g *gridworldEnv) tileAt(row, col int) tile {
	if g.tiles == nil {
		return tile{kind: tileEmpty}
	}
	if t, ok := g.tiles[position{row: row, col: col}]; ok {
		return t
	}
	return tile{kind: tileEmpty}
}

func (g *gridworldEnv) wallPositions() []Position {
	if len(g.tiles) == 0 {
		return nil
	}
	positions := make([]Position, 0, len(g.tiles))
	for pos, t := range g.tiles {
		if t.kind != tileWall {
			continue
		}
		positions = append(positions, Position{Row: pos.row, Col: pos.col})
	}
	if len(positions) == 0 {
		return nil
	}
	return positions
}

func (g *gridworldEnv) slipTiles() []SlipTile {
	if len(g.tiles) == 0 {
		return nil
	}
	tiles := make([]SlipTile, 0, len(g.tiles))
	for pos, t := range g.tiles {
		if t.kind != tileSlip {
			continue
		}
		tiles = append(tiles, SlipTile{Row: pos.row, Col: pos.col, Probability: t.slipProb})
	}
	if len(tiles) == 0 {
		return nil
	}
	return tiles
}

func (g *gridworldEnv) step(action int) (float64, bool) {
	if g.stepsTaken >= g.maxSteps {
		return 0, true
	}
	actual := g.resolveAction(action)
	row, col := g.nextPosition(actual)
	if g.tileAt(row, col).kind == tileWall {
		row, col = g.currRow, g.currCol
	}
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

func (g *gridworldEnv) resolveAction(action int) int {
	tile := g.tileAt(g.currRow, g.currCol)
	if tile.kind != tileSlip {
		return action
	}
	if g.rng == nil {
		return action
	}
	if tile.slipProb <= 0 {
		return action
	}
	if tile.slipProb >= 1 {
		return g.rng.Intn(4)
	}
	if g.rng.Float64() < tile.slipProb {
		return g.rng.Intn(4)
	}
	return action
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
