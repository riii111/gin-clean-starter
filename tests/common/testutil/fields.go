//go:build unit || e2e

package testutil

// a helper function for dynamically modifying map fields in tests
func Field(key string, value any) func(m map[string]any) {
	return func(m map[string]any) {
		if value == nil {
			delete(m, key)
		} else {
			m[key] = value
		}
	}
}
