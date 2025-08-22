//go:build unit || e2e

package helper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func AssertSuccessResponse(t *testing.T, w *httptest.ResponseRecorder, expectedStatus int, targetStruct any) {
	t.Helper()

	if !assert.Equal(t, expectedStatus, w.Code,
		fmt.Sprintf("Expected status %d, got %d. Response: %s", expectedStatus, w.Code, w.Body.String())) {
		return
	}

	if expectedStatus >= 200 && expectedStatus < 300 && targetStruct != nil {
		err := json.Unmarshal(w.Body.Bytes(), targetStruct)
		assert.NoError(t, err, fmt.Sprintf("Failed to decode response JSON: %s", w.Body.String()))
	}
}

func AssertErrorResponse(t *testing.T, w *httptest.ResponseRecorder, expectedStatus int, expectedErrorMsg string) {
	t.Helper()

	assert.Equal(t, expectedStatus, w.Code,
		fmt.Sprintf("Expected status %d, got %d", expectedStatus, w.Code))

	var errorResponse map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
	assert.NoError(t, err, fmt.Sprintf("Failed to decode error response JSON: %s", w.Body.String()))

	if expectedErrorMsg != "" {
		assert.Contains(t, errorResponse["error"], expectedErrorMsg,
			"Response error message doesn't contain expected text")
	}
}

// DecodeResponseBody decodes JSON response body into target struct
func DecodeResponseBody(t *testing.T, body io.Reader, target any) error {
	t.Helper()

	err := json.NewDecoder(body).Decode(target)
	require.NoError(t, err, "Failed to decode response body")

	return err
}
