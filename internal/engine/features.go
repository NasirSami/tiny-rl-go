package engine

const distanceBands = 3

func distanceBand(distance float64) int {
	if distance <= 1 {
		return 0
	}
	if distance <= 3 {
		return 1
	}
	return 2
}
