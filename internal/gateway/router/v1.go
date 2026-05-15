package router

import "github.com/gin-gonic/gin"

// registerAPIRoutes creates the /api/v1 route namespace.
// Add business-specific route files beside this file as services are introduced.
func registerAPIRoutes(r *gin.Engine) {
	api := r.Group("/api")
	v1 := api.Group("/v1")
	_ = v1
}
