package main

import "fmt"

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
