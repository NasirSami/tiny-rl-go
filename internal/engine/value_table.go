package engine

import "fmt"

type valueTable struct {
	rows     int
	cols     int
	features int
	alpha    float64
	mapper   FeatureMapper
	data     [][]float64
}

func newValueTable(rows, cols int, alpha float64) *valueTable {
	return newValueTableWithMapper(rows, cols, alpha, DistanceBands3Mapper{})
}

func newValueTableWithMapper(rows, cols int, alpha float64, mapper FeatureMapper) *valueTable {
	if mapper == nil {
		mapper = DistanceBands3Mapper{}
	}
	featureCount := mapper.NumFeatures(rows, cols)
	if featureCount <= 0 {
		featureCount = 1
	}
	totalRows := rows * featureCount
	data := make([][]float64, totalRows)
	for r := 0; r < totalRows; r++ {
		data[r] = make([]float64, cols)
	}
	return &valueTable{
		rows:     rows,
		cols:     cols,
		features: featureCount,
		alpha:    alpha,
		mapper:   mapper,
		data:     data,
	}
}

func (v *valueTable) update(path []position, reward float64) {
	for _, pos := range path {
		current := v.data[pos.row][pos.col]
		v.data[pos.row][pos.col] = current + v.alpha*(reward-current)
	}
}

func (v *valueTable) cloneData() [][]float64 {
	copyData := make([][]float64, v.rows)
	for r := 0; r < v.rows; r++ {
		copyData[r] = make([]float64, v.cols)
		for c := 0; c < v.cols; c++ {
			best := v.data[v.flatIndex(r, 0)][c]
			for f := 1; f < v.features; f++ {
				val := v.data[v.flatIndex(r, f)][c]
				if val > best {
					best = val
				}
			}
			copyData[r][c] = best
		}
	}
	return copyData
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

func (v *valueTable) flatIndex(row, feature int) int {
	if row < 0 {
		row = 0
	}
	if row >= v.rows {
		row = v.rows - 1
	}
	if feature < 0 {
		feature = 0
	}
	if feature >= v.features {
		feature = v.features - 1
	}
	return row*v.features + feature
}

func (v *valueTable) get(row, col, feature int) float64 {
	return v.data[v.flatIndex(row, feature)][col]
}

func (v *valueTable) add(row, col, feature int, delta float64) {
	idx := v.flatIndex(row, feature)
	v.data[idx][col] += delta
}

func (v *valueTable) addFlat(flatRow, col int, delta float64) {
	if flatRow < 0 || flatRow >= len(v.data) {
		return
	}
	v.data[flatRow][col] += delta
}

func (v *valueTable) bandRows() int {
	return len(v.data)
}
