package router

import (
	"github.com/gin-gonic/gin"

	"github.com/BwCloudWeGo/bw-cli/internal/gateway/handler"
)

// registerOrderReportRoutes registers /api/v1/demo_orders endpoints in one business-specific file.
func registerOrderReportRoutes(v1 *gin.RouterGroup, orderReportHandler *handler.OrderReportHandler) {
	routes := v1.Group("/demo_orders")
	routes.POST("", orderReportHandler.Create)
	routes.GET("", orderReportHandler.List)
	routes.GET("/:id", orderReportHandler.Get)
	routes.PUT("/:id", orderReportHandler.Update)
	routes.DELETE("/:id", orderReportHandler.Delete)
}
