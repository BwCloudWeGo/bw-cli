package router

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/internal/gateway/client"
	"github.com/BwCloudWeGo/bw-cli/internal/gateway/handler"
	"github.com/BwCloudWeGo/bw-cli/pkg/config"
)

// registerAPIRoutes 创建 /api/v1 路由命名空间，再按业务模块分发。
func registerAPIRoutes(r *gin.Engine, clients *client.Clients, log *zap.Logger, middlewareCfg config.MiddlewareConfig) {
	api := r.Group("/api")
	v1 := api.Group("/v1")

	registerUserRoutes(v1, handler.NewUserHandler(clients.User, middlewareCfg.JWT, log), middlewareCfg.JWT)
	registerNoteRoutes(v1, handler.NewNoteHandler(clients.Note, log))
	registerOrderRoutes(v1, handler.NewOrderHandler(clients.Order, log))
}
