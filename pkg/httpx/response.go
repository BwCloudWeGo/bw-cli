package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"

	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
)

// Response is the unified JSON envelope returned by gateway handlers.
type Response struct {
	RequestID string      `json:"request_id,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Error     *ErrorBody  `json:"error,omitempty"`
}

// ErrorBody is the public error payload shape used by HTTP clients.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// OK writes a 200 response with the standard envelope.
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		RequestID: RequestID(c),
		Data:      data,
	})
}

// Created writes a 201 response with the standard envelope.
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		RequestID: RequestID(c),
		Data:      data,
	})
}

// Error maps application errors to HTTP status codes and response bodies.
func Error(c *gin.Context, err error) {
	appErr, ok := apperrors.As(err)
	if !ok {
		appErr = apperrors.Internal("internal_error", "internal error")
	}
	c.Set("error_code", appErr.Code)
	c.JSON(apperrors.HTTPStatus(appErr), Response{
		RequestID: RequestID(c),
		Error: &ErrorBody{
			Code:    appErr.Code,
			Message: appErr.Message,
		},
	})
}

// RequestID returns the request id produced by middleware.RequestID.
func RequestID(c *gin.Context) string {
	value, ok := c.Get("request_id")
	if !ok {
		return ""
	}
	requestID, _ := value.(string)
	return requestID
}
