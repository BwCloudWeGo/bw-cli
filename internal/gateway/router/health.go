package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// registerHealthRoutes 注册不属于 API 版本命名空间的进程级健康检查接口。
func registerHealthRoutes(r *gin.Engine) {
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
