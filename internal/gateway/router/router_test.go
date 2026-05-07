package router_test

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	requireRoute(t, registeredRoutes, http.MethodGet, "/api/v1/users/:id")
	requireRoute(t, registeredRoutes, http.MethodPost, "/api/v1/notes")
	requireRoute(t, registeredRoutes, http.MethodGet, "/api/v1/notes/:id")
	requireRoute(t, registeredRoutes, http.MethodPost, "/api/v1/notes/publishNote")
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
	return &userv1.UserResponse{Id: "user-1", Email: "ada@example.com", DisplayName: "Ada"}, nil
}

func (fakeUserClient) Login(context.Context, *userv1.LoginRequest, ...grpc.CallOption) (*userv1.UserResponse, error) {
	return &userv1.UserResponse{Id: "user-1", Email: "ada@example.com", DisplayName: "Ada"}, nil
}

func (fakeUserClient) GetUser(context.Context, *userv1.GetUserRequest, ...grpc.CallOption) (*userv1.UserResponse, error) {
	return &userv1.UserResponse{Id: "user-1", Email: "ada@example.com", DisplayName: "Ada"}, nil
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
