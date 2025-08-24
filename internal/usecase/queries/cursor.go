package queries

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	MaxListLimit    = 200
	CursorVersionV1 = "v1"
)

// Uses microsecond precision to align with PostgreSQL timestamp precision
func EncodeAfterCursor(t time.Time, id uuid.UUID) string {
	cursorData := fmt.Sprintf("%s:%d-%s", CursorVersionV1, t.UnixMicro(), id.String())
	return base64.URLEncoding.EncodeToString([]byte(cursorData))
}

// Supports legacy format for backward compatibility
func DecodeAfterCursor(cursor string) (time.Time, uuid.UUID, error) {
	if cursor == "" {
		return time.Time{}, uuid.Nil, fmt.Errorf("cursor cannot be empty")
	}

	// Try to decode as base64url first (v1 format)
	if decoded, err := base64.URLEncoding.DecodeString(cursor); err == nil {
		decodedStr := string(decoded)
		if strings.HasPrefix(decodedStr, CursorVersionV1+":") {
			return parseVersionedCursor(decodedStr)
		}
	}

	// Fall back to legacy format for backward compatibility
	return parseLegacyCursor(cursor)
}

func parseVersionedCursor(cursorData string) (time.Time, uuid.UUID, error) {
	payload := strings.TrimPrefix(cursorData, CursorVersionV1+":")

	parts := strings.SplitN(payload, "-", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, fmt.Errorf("invalid cursor format: expected '<micros>-<uuid>'")
	}

	timestamp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("invalid timestamp: %w", err)
	}

	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("invalid UUID: %w", err)
	}

	return time.UnixMicro(timestamp), id, nil
}

func parseLegacyCursor(cursor string) (time.Time, uuid.UUID, error) {
	parts := strings.SplitN(cursor, "-", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, fmt.Errorf("invalid cursor format")
	}

	timestamp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("invalid timestamp: %w", err)
	}

	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("invalid UUID: %w", err)
	}

	return time.Unix(0, timestamp), id, nil
}

type Cursor struct {
	After string `json:"after,omitempty"`
}

func ValidateLimit(limit int) int {
	if limit <= 0 {
		return 20 // default limit
	}
	if limit > MaxListLimit {
		return MaxListLimit
	}
	return limit
}
