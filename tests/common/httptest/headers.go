//go:build unit || e2e

package httptest

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func AssertHeaders(t *testing.T, w *httptest.ResponseRecorder, expected map[string]string) {
	t.Helper()
	for k, v := range expected {
		assert.Equal(t, v, w.Header().Get(k), "header %s mismatch", k)
	}
}
