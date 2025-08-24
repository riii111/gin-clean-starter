package httperr

import (
	"github.com/gin-gonic/gin"
)

type Response struct {
	Status int `json:"-"`
	Error  struct {
		Message string `json:"message"`
	} `json:"error"`
	Detail any `json:"detail,omitempty"`
}

// preserves original error for future monitoring
func AbortWithError(c *gin.Context, status int, err error, msg string, detail any) {
	if err == nil {
		panic("AbortWithError: err cannot be nil")
	}

	resp := Response{Status: status}
	resp.Error.Message = msg
	resp.Detail = detail

	_ = c.Error(gin.Error{
		Err:  err,
		Type: gin.ErrorTypePublic,
		Meta: resp,
	})
	c.AbortWithStatusJSON(status, resp)
}
