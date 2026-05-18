package router_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	notev1 "github.com/BwCloudWeGo/bw-cli/api/gen/note/v1"
	userv1 "github.com/BwCloudWeGo/bw-cli/api/gen/user/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/gateway/client"
	"github.com/BwCloudWeGo/bw-cli/internal/gateway/router"
	"github.com/BwCloudWeGo/bw-cli/pkg/config"
	"github.com/BwCloudWeGo/bw-cli/pkg/middleware"
)

func TestRouterUsesConfiguredCORSAndVersionedBusinessRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := router.New(&client.Clients{
		User: fakeUserClient{},
		Note: fakeNoteClient{},
	}, zap.NewNop(), config.MiddlewareConfig{
		CORS: middleware.CORSConfig{
			AllowOrigins: []string{"http://console.example.com"},
			AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
			AllowHeaders: []string{"Authorization", "Content-Type"},
		},
		JWT: middleware.JWTConfig{
			Secret:        "test-secret",
			Issuer:        "xiaolanshu",
			ExpireSeconds: 7200,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-1", nil)
	req.Header.Set("Origin", "http://console.example.com")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "http://console.example.com", rec.Header().Get("Access-Control-Allow-Origin"))

	registeredRoutes := engine.Routes()
	requireRoute(t, registeredRoutes, http.MethodPost, "/api/v1/users/register")
	requireRoute(t, registeredRoutes, http.MethodPost, "/api/v1/users/login")
	requireRoute(t, registeredRoutes, http.MethodGet, "/api/v1/users/me")
	requireRoute(t, registeredRoutes, http.MethodGet, "/api/v1/users/:id")
	requireRoute(t, registeredRoutes, http.MethodPost, "/api/v1/notes")
	requireRoute(t, registeredRoutes, http.MethodGet, "/api/v1/notes/:id")
	requireRoute(t, registeredRoutes, http.MethodPost, "/api/v1/notes/publishNote")
}

func TestRouterProtectsCurrentUserRouteWithJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := router.New(&client.Clients{
		User: fakeUserClient{},
		Note: fakeNoteClient{},
	}, zap.NewNop(), config.MiddlewareConfig{
		JWT: middleware.JWTConfig{
			Secret:        "test-secret",
			Issuer:        "xiaolanshu",
			ExpireSeconds: 7200,
		},
	})
	token, err := middleware.GenerateToken(middleware.JWTConfig{
		Secret:        "test-secret",
		Issuer:        "xiaolanshu",
		ExpireSeconds: 7200,
	}, middleware.JWTClaims{UserID: "user-1"})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "user-1", data["id"])
	require.Equal(t, "ada", data["account"])
}

func TestRouterRejectsCurrentUserRouteWithoutJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := router.New(&client.Clients{
		User: fakeUserClient{},
		Note: fakeNoteClient{},
	}, zap.NewNop(), config.MiddlewareConfig{
		JWT: middleware.JWTConfig{
			Secret:        "test-secret",
			Issuer:        "xiaolanshu",
			ExpireSeconds: 7200,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRouterLoginIssuesJWTAtGateway(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := router.New(&client.Clients{
		User: fakeUserClient{},
		Note: fakeNoteClient{},
	}, zap.NewNop(), config.MiddlewareConfig{
		JWT: middleware.JWTConfig{
			Secret:        "test-secret",
			Issuer:        "xiaolanshu",
			ExpireSeconds: 7200,
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", strings.NewReader(`{"account":"ada","password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	token, ok := data["token"].(string)
	require.True(t, ok)
	require.NotEmpty(t, token)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func requireRoute(t *testing.T, routes gin.RoutesInfo, method string, path string) {
	t.Helper()
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return
		}
	}
	require.Failf(t, "route not registered", "%s %s", method, path)
}

type fakeUserClient struct{}

func (fakeUserClient) Register(context.Context, *userv1.RegisterRequest, ...grpc.CallOption) (*userv1.UserResponse, error) {
	return &userv1.UserResponse{Id: "user-1", Account: "ada", DisplayName: "Ada"}, nil
}

func (fakeUserClient) Login(context.Context, *userv1.LoginRequest, ...grpc.CallOption) (*userv1.UserResponse, error) {
	return &userv1.UserResponse{Id: "user-1", Account: "ada", DisplayName: "Ada"}, nil
}

func (fakeUserClient) GetUser(context.Context, *userv1.GetUserRequest, ...grpc.CallOption) (*userv1.UserResponse, error) {
	return &userv1.UserResponse{Id: "user-1", Account: "ada", DisplayName: "Ada"}, nil
}

type fakeNoteClient struct{}

func (fakeNoteClient) CreateNote(context.Context, *notev1.CreateNoteRequest, ...grpc.CallOption) (*notev1.NoteResponse, error) {
	return &notev1.NoteResponse{Id: "note-1", AuthorId: "user-1", Title: "Title", Content: "Content", Status: "DRAFT"}, nil
}

func (fakeNoteClient) GetNote(context.Context, *notev1.GetNoteRequest, ...grpc.CallOption) (*notev1.NoteResponse, error) {
	return &notev1.NoteResponse{Id: "note-1", AuthorId: "user-1", Title: "Title", Content: "Content", Status: "DRAFT"}, nil
}

func (fakeNoteClient) PublishNote(context.Context, *notev1.PublishNoteRequest, ...grpc.CallOption) (*notev1.NoteResponse, error) {
	return &notev1.NoteResponse{Id: "note-1", AuthorId: "user-1", Title: "Title", Content: "Content", Status: "PUBLISHED"}, nil
}
