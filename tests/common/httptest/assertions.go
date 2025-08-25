//go:build unit || e2e

package httptest

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
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

	var errorResponse struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
	assert.NoError(t, err, fmt.Sprintf("Failed to decode error response JSON: %s", w.Body.String()))

	if expectedErrorMsg != "" {
		assert.Contains(t, errorResponse.Error.Message, expectedErrorMsg,
			"Response error message doesn't contain expected text")
	}
}
