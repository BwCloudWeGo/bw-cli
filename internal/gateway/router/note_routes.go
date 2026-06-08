package router

import (
	"github.com/gin-gonic/gin"

	"github.com/BwCloudWeGo/bw-cli/internal/gateway/handler"
)

// registerNoteRoutes 在独立业务文件中注册 /api/v1/notes 接口。
func registerNoteRoutes(v1 *gin.RouterGroup, noteHandler *handler.NoteHandler) {
	notes := v1.Group("/notes")
	notes.POST("", noteHandler.Create)
	notes.GET("/:id", noteHandler.Get)
	notes.POST("/publishNote", noteHandler.PublishNote)
}
