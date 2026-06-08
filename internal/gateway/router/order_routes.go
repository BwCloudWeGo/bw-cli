package router

import (
	"github.com/gin-gonic/gin"

	"github.com/BwCloudWeGo/bw-cli/internal/gateway/handler"
)

// registerOrderRoutes 在独立业务文件中注册 /api/v1/orders 接口。
func registerOrderRoutes(v1 *gin.RouterGroup, orderHandler *handler.OrderHandler) {
	routes := v1.Group("/orders")
	routes.POST("", orderHandler.Create)
	routes.GET("", orderHandler.List)
	routes.GET("/:id", orderHandler.Get)
	routes.PUT("/:id", orderHandler.Update)
	routes.DELETE("/:id", orderHandler.Delete)
}
