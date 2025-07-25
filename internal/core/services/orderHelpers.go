package services

// Generate order_number: Create a unique order number using the format ORD_YYYYMMDD_NNN. The NNN sequence should reset to 001 daily (based on UTC). This must be managed transactionally to prevent race conditions
func GenerateOrderNumber() {
}

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
