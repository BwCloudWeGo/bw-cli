package router

import (
	"github.com/gin-gonic/gin"

	"github.com/BwCloudWeGo/bw-cli/internal/gateway/handler"
	"github.com/BwCloudWeGo/bw-cli/pkg/middleware"
)

// registerUserRoutes 在独立业务文件中注册 /api/v1/users 接口。
func registerUserRoutes(v1 *gin.RouterGroup, userHandler *handler.UserHandler, jwtCfg middleware.JWTConfig) {
	users := v1.Group("/users")
	users.POST("/register", userHandler.Register)
	users.POST("/login", userHandler.Login)
	users.GET("/me", middleware.JWTAuth(jwtCfg), userHandler.CurrentUser)
	users.GET("/:id", userHandler.GetUser)
}
