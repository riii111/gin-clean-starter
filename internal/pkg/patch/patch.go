package patch

// Coalesce returns the value pointed to by ptr if it's not nil, otherwise returns fallback
func Coalesce[T any](ptr *T, fallback T) T {
	if ptr != nil {
		return *ptr
	}
	return fallback
}
