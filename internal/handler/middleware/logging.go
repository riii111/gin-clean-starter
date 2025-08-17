package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"gin-clean-starter/internal/pkg/config"

	"github.com/gin-gonic/gin"
)

func (l *Logger) LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		requestID := l.generateRequestID()

		c.Set("request_id", requestID)

		userID, role := extractUserContext(c)

		logAttrs := []slog.Attr{
			slog.String("request_id", requestID),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.String("client_ip", c.ClientIP()),
		}

		if userID != "" {
			logAttrs = append(logAttrs, slog.String("user_id", userID))
		}
		if role != "" {
			logAttrs = append(logAttrs, slog.String("role", role))
		}

		if idempotencyKey := c.GetHeader("Idempotency-Key"); idempotencyKey != "" {
			logAttrs = append(logAttrs, slog.String("idempotency_key", idempotencyKey))
		}

		l.logger.LogAttrs(context.Background(), slog.LevelInfo, "Request started", logAttrs...)

		c.Next()

		duration := time.Since(startTime)
		statusCode := c.Writer.Status()

		responseAttrs := make([]slog.Attr, len(logAttrs), len(logAttrs)+3)
		copy(responseAttrs, logAttrs)
		responseAttrs = append(responseAttrs,
			slog.Int("status_code", statusCode),
			slog.Duration("duration", duration),
		)

		if responseSize := c.Writer.Size(); responseSize > 0 {
			responseAttrs = append(responseAttrs, slog.Int("response_size", responseSize))
		}

		if len(c.Errors) > 0 {
			responseAttrs = append(responseAttrs, slog.String("errors", c.Errors.String()))
		}

		logLevel := slog.LevelInfo
		if statusCode >= 500 {
			logLevel = slog.LevelError
		} else if statusCode >= 400 {
			logLevel = slog.LevelWarn
		}

		l.logger.LogAttrs(context.Background(), logLevel, "Request completed", responseAttrs...)
	}
}

func NewLogger(cfg config.LogConfig) *Logger {
	var logLevel slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	timezone := time.FixedZone(cfg.TimeZone, cfg.TimeZoneOffset)

	opts := &slog.HandlerOptions{
		Level: logLevel,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.In(timezone).Format(cfg.TimeFormat))
				}
			}
			return a
		},
	}

	var handler slog.Handler
	if gin.Mode() == gin.ReleaseMode {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return &Logger{
		logger:   logger,
		cfg:      cfg,
		timezone: timezone,
	}
}

func (l *Logger) GetSlogLogger() *slog.Logger {
	return l.logger
}

func GetRequestID(c *gin.Context) string {
	if requestID, exists := c.Get("request_id"); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}

func LoggingMiddleware(_ *slog.Logger, cfg config.LogConfig) gin.HandlerFunc {
	l := NewLogger(cfg)
	return l.LoggingMiddleware()
}

func (l *Logger) generateRequestID() string {
	timestamp := time.Now().In(l.timezone).Format("20060102150405")

	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Sprintf("%s-fallback-%d", timestamp, time.Now().UnixNano()%100000000)
	}

	randomHex := hex.EncodeToString(randomBytes)
	return fmt.Sprintf("%s-%s", timestamp, randomHex)
}

func extractUserContext(c *gin.Context) (userID, role string) {
	if claims, exists := c.Get("jwt_claims"); exists {
		if claimsMap, ok := claims.(map[string]any); ok {
			if uid, ok := claimsMap["user_id"].(string); ok {
				userID = uid
			}
			if r, ok := claimsMap["role"].(string); ok {
				role = r
			}
		}
	}

	if userID == "" {
		userID = c.GetHeader("X-User-ID")
	}
	if role == "" {
		role = c.GetHeader("X-User-Role")
	}

	return
}

type Logger struct {
	logger   *slog.Logger
	cfg      config.LogConfig
	timezone *time.Location
}
