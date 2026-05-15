package router

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/internal/gateway/client"
	"github.com/BwCloudWeGo/bw-cli/internal/gateway/handler"
)

// registerAPIRoutes creates the /api/v1 route namespace before delegating by business module.
func registerAPIRoutes(r *gin.Engine, clients *client.Clients, log *zap.Logger) {
	api := r.Group("/api")
	v1 := api.Group("/v1")

	registerUserRoutes(v1, handler.NewUserHandler(clients.User, log))
	registerNoteRoutes(v1, handler.NewNoteHandler(clients.Note, log))
}
