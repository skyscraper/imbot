package model

func pctReturn(p1, p2 float64) float64 {
	return (p2 / p1) - 1
}

func ewmStep(previous, observed, alpha float64) float64 {
	if previous == 0 {
		return observed
	}
	return (alpha * observed) + ((1 - alpha) * previous)
}
