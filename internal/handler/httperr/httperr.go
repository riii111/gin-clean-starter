package httperr

import (
	"github.com/gin-gonic/gin"
)

type ErrorType string

const (
	TypeValidation ErrorType = "Validation failed"
	TypeAuth       ErrorType = "Authentication failed"
	TypeNotFound   ErrorType = "Resource not found"
	TypeBadRequest ErrorType = "Bad request"
	TypeConflict   ErrorType = "Data conflict occurred"
	TypeInternal   ErrorType = "Internal server error"
)

type Response struct {
	Status int `json:"-"`
	Error  struct {
		Type    ErrorType `json:"type"`
		Message string    `json:"message"`
	} `json:"error"`
	Detail any `json:"detail,omitempty"`
}

// preserves original error for future monitoring
func AbortWithError(c *gin.Context, status int, err error, errType ErrorType, msg string, detail any) {
	if err == nil {
		panic("AbortWithError: err cannot be nil")
	}

	resp := Response{Status: status}
	resp.Error.Type = errType
	resp.Error.Message = msg
	resp.Detail = detail

	_ = c.Error(gin.Error{
		Err:  err,
		Type: gin.ErrorTypePublic,
		Meta: resp,
	})
	c.AbortWithStatusJSON(status, resp)
}
