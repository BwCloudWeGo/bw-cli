package router

import (
	"github.com/gin-gonic/gin"

	"github.com/BwCloudWeGo/bw-cli/internal/gateway/handler"
	"github.com/BwCloudWeGo/bw-cli/pkg/middleware"
)

// registerUserRoutes registers /api/v1/users endpoints in one business-specific file.
func registerUserRoutes(v1 *gin.RouterGroup, userHandler *handler.UserHandler, jwtCfg middleware.JWTConfig) {
	users := v1.Group("/users")
	users.POST("/register", userHandler.Register)
	users.POST("/login", userHandler.Login)
	users.GET("/me", middleware.JWTAuth(jwtCfg), userHandler.CurrentUser)
	users.GET("/:id", userHandler.GetUser)
}
