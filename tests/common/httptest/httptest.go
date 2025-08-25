//go:build unit || e2e

package httptest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// executes HTTP request with optional authorization
func PerformRequest(t *testing.T, router *gin.Engine, method, path string, body any, authToken string) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody *bytes.Buffer
	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(t, err, "Failed to encode request body to JSON")
		reqBody = bytes.NewBuffer(jsonBody)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// performs HTTP request with cookies support
func PerformRequestWithCookies(t *testing.T, router *gin.Engine, method, path string, body any, cookies []*http.Cookie, authToken string) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody *bytes.Buffer
	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(t, err, "Failed to encode request body to JSON")
		reqBody = bytes.NewBuffer(jsonBody)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	// Add cookies to the request
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// extracts all cookies from response
func ExtractCookies(w *httptest.ResponseRecorder) []*http.Cookie {
	return w.Result().Cookies()
}

// extracts specific cookie by name from response
func ExtractCookie(w *httptest.ResponseRecorder, name string) *http.Cookie {
	cookies := w.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

// decodes JSON response body into target struct
func DecodeResponseBody(t *testing.T, body *bytes.Buffer, target any) error {
	t.Helper()

	err := json.NewDecoder(body).Decode(target)
	require.NoError(t, err, "Failed to decode response body")

	return err
}
