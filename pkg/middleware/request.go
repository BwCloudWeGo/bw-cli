package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// HeaderRequestID 是用于请求关联的公开 HTTP 头。
const HeaderRequestID = "X-Request-ID"

// RequestID 确保每个请求都有稳定的关联 ID。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(HeaderRequestID)
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Set("request_id", requestID)
		c.Writer.Header().Set(HeaderRequestID, requestID)
		c.Next()
	}
}

// RequestLogger 在每个请求完成后记录结构化 HTTP 访问日志。
func RequestLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		log.Info("http request completed",
			zap.String("request_id", requestID(c)),
			zap.String("trace_id", c.GetHeader("traceparent")),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("route", c.FullPath()),
			zap.Int("status", c.Writer.Status()),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Float64("latency_ms", float64(time.Since(start).Microseconds())/1000),
			zap.Int64("request_bytes", c.Request.ContentLength),
			zap.Int("response_bytes", c.Writer.Size()),
			zap.String("error_code", errorCode(c)),
		)
	}
}

func requestID(c *gin.Context) string {
	value, ok := c.Get("request_id")
	if !ok {
		return ""
	}
	requestID, _ := value.(string)
	return requestID
}

func errorCode(c *gin.Context) string {
	value, ok := c.Get("error_code")
	if !ok {
		return ""
	}
	errorCode, _ := value.(string)
	return errorCode
}
