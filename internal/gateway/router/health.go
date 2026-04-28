package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// registerHealthRoutes registers process-level health endpoints outside API versions.
func registerHealthRoutes(r *gin.Engine) {
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
