package router

import (
	"github.com/gin-gonic/gin"

	"github.com/BwCloudWeGo/bw-cli/internal/gateway/handler"
)

// registerNoteRoutes registers /api/v1/notes endpoints in one business-specific file.
func registerNoteRoutes(v1 *gin.RouterGroup, noteHandler *handler.NoteHandler) {
	notes := v1.Group("/notes")
	notes.POST("", noteHandler.Create)
	notes.GET("/:id", noteHandler.Get)
	notes.POST("/publishNote", noteHandler.PublishNote)
}
