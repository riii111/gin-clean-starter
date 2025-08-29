package middleware

import (
	"log/slog"
	"net/http"

	"gin-clean-starter/internal/handler/httperr"

	"github.com/gin-gonic/gin"
)

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if c.Writer.Written() {
			return
		}
		// Search backward through the error stack
		for i := len(c.Errors) - 1; i >= 0; i-- {
			err := c.Errors[i]

			if err.IsType(gin.ErrorTypePublic) {
				// Public: Meta ⇒ Return as is
				if resp, ok := err.Meta.(httperr.Response); ok {
					c.JSON(resp.Status, resp)
					return
				}
			}
		}
		if status := c.Writer.Status(); status != http.StatusOK {
			c.Status(status)
			c.Writer.WriteHeaderNow()
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "Internal server error"}})
	}
}

func CustomRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("recovered from panic", "error", err, "path", c.Request.URL.Path)

				resp := httperr.Response{Status: http.StatusInternalServerError}
				resp.Error.Message = "Internal server error"

				c.JSON(http.StatusInternalServerError, resp)
				c.Abort()
			}
		}()
		c.Next()
	}
}
