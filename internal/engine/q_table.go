package engine

type qTable struct {
	rows    int
	cols    int
	actions int
	data    [][][]float64
}

func newQTable(rows, cols, actions int) *qTable {
	data := make([][][]float64, rows)
	for r := 0; r < rows; r++ {
		data[r] = make([][]float64, cols)
		for c := 0; c < cols; c++ {
			data[r][c] = make([]float64, actions)
		}
	}
	return &qTable{rows: rows, cols: cols, actions: actions, data: data}
}

func (q *qTable) get(row, col, action int) float64 {
	return q.data[row][col][action]
}

func (q *qTable) set(row, col, action int, value float64) {
	q.data[row][col][action] = value
}

func (q *qTable) maxValue(row, col int) float64 {
	max := q.data[row][col][0]
	for a := 1; a < q.actions; a++ {
		if q.data[row][col][a] > max {
			max = q.data[row][col][a]
		}
	}
	return max
}

func (q *qTable) stateValues() [][]float64 {
	values := make([][]float64, q.rows)
	for r := 0; r < q.rows; r++ {
		values[r] = make([]float64, q.cols)
		for c := 0; c < q.cols; c++ {
			values[r][c] = q.maxValue(r, c)
		}
	}
	return values
}
