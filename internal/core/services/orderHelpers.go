package services

// Assign priority: Determine the order's priority based on its total amount. The highest applicable priority is used.
func SetPriority(total float64) int {
	switch {
	case total > 100:
		return 10
	case total >= 50:
		return 5
	default:
		return 1
	}
}
