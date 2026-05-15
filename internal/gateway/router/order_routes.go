package router

import (
	"github.com/gin-gonic/gin"

	"github.com/BwCloudWeGo/bw-cli/internal/gateway/handler"
)

// registerOrderRoutes registers /api/v1/orders endpoints in one business-specific file.
func registerOrderRoutes(v1 *gin.RouterGroup, orderHandler *handler.OrderHandler) {
	routes := v1.Group("/orders")
	routes.POST("", orderHandler.Create)
	routes.GET("", orderHandler.List)
	routes.GET("/:id", orderHandler.Get)
	routes.PUT("/:id", orderHandler.Update)
	routes.DELETE("/:id", orderHandler.Delete)
}
