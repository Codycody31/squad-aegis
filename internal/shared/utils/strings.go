package utils

func ReturnOldIfEmpty(oldValue, newValue string) string {
	if newValue == "" {
		return oldValue
	}
	return newValue
}
