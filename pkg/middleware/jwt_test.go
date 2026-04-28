package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/middleware"
)

func TestJWTAuthAcceptsBearerTokenAndSetsClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token, err := middleware.GenerateToken(middleware.JWTConfig{
		Secret:        "unit-test-secret",
		Issuer:        "xiaolanshu",
		ExpireSeconds: 3600,
	}, middleware.JWTClaims{
		UserID: "user-1",
		Role:   "admin",
	})
	require.NoError(t, err)

	r := gin.New()
	r.Use(middleware.JWTAuth(middleware.JWTConfig{
		Secret:        "unit-test-secret",
		Issuer:        "xiaolanshu",
		ExpireSeconds: 3600,
	}))
	r.GET("/me", func(c *gin.Context) {
		claims := middleware.ClaimsFromContext(c)
		c.JSON(http.StatusOK, gin.H{"user_id": claims.UserID, "role": claims.Role})
	})

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"user_id":"user-1","role":"admin"}`, rec.Body.String())
}

func TestJWTAuthRejectsMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.JWTAuth(middleware.JWTConfig{
		Secret:        "unit-test-secret",
		Issuer:        "xiaolanshu",
		ExpireSeconds: int64(time.Hour.Seconds()),
	}))
	r.GET("/me", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/me", nil))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
