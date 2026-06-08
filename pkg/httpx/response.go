package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"

	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
)

// Response 是 gateway 处理器返回的统一 JSON 外壳。
type Response struct {
	RequestID string      `json:"request_id,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Error     *ErrorBody  `json:"error,omitempty"`
}

// ErrorBody 是 HTTP 客户端可见的错误载荷结构。
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// OK 写出带标准外壳的 200 响应。
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		RequestID: RequestID(c),
		Data:      data,
	})
}

// Created 写出带标准外壳的 201 响应。
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		RequestID: RequestID(c),
		Data:      data,
	})
}

// Error 将应用错误映射为 HTTP 状态码和响应体。
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

// RequestID 返回 middleware.RequestID 生成的请求 ID。
func RequestID(c *gin.Context) string {
	value, ok := c.Get("request_id")
	if !ok {
		return ""
	}
	requestID, _ := value.(string)
	return requestID
}
