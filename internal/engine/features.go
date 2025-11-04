package engine

// FeatureMapper describes how to map a grid position to a feature index used by value-based learners.
type FeatureMapper interface {
	NumFeatures(rows, cols int) int
	Index(env *gridworldEnv, row, col int) int
}

// DistanceBands3Mapper replicates the current three-band distance heuristic.
type DistanceBands3Mapper struct{}

// NumFeatures returns the number of feature slots required for the mapper.
func (DistanceBands3Mapper) NumFeatures(rows, cols int) int {
	_ = rows
	_ = cols
	return 3
}

// Index computes which distance band contains the provided cell.
func (DistanceBands3Mapper) Index(env *gridworldEnv, row, col int) int {
	if env == nil {
		return 0
	}
	distance := env.potential(row, col)
	switch {
	case distance <= 1:
		return 0
	case distance <= 3:
		return 1
	default:
		return 2
	}
}

// DistanceBandsMapper allows specifying custom band thresholds.
type DistanceBandsMapper struct {
	Thresholds []float64
}

// NumFeatures returns the number of configured bands (at least one).
func (m DistanceBandsMapper) NumFeatures(rows, cols int) int {
	_ = rows
	_ = cols
	if len(m.Thresholds) == 0 {
		return 1
	}
	return len(m.Thresholds) + 1
}

// Index assigns the cell to the first threshold that is at least as large as the current potential.
func (m DistanceBandsMapper) Index(env *gridworldEnv, row, col int) int {
	if env == nil {
		return 0
	}
	distance := env.potential(row, col)
	for i, limit := range m.Thresholds {
		if distance <= limit {
			return i
		}
	}
	return len(m.Thresholds)
}
