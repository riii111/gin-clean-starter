//go:build unit || e2e

package testutil

import (
	"encoding/json"
	"testing"
)

func DtoMap(t *testing.T, v any, muts ...func(map[string]any)) map[string]any {
	t.Helper()
	b, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	for _, f := range muts {
		f(m)
	}
	return m
}
