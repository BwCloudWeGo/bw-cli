package scaffold_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/scaffold"
)

func TestAddServiceWritesLayeredServiceStructure(t *testing.T) {
	root := t.TempDir()
	writeMinimalProject(t, root)

	err := scaffold.AddService(scaffold.ServiceOptions{
		RootDir: root,
		Name:    "order-item",
		Port:    9103,
	})

	require.NoError(t, err)
	serviceFiles := []string{
		"cmd/order_item/main.go",
		"api/proto/order_item/v1/order_item.proto",
		"internal/order_item/domain/entity.go",
		"internal/order_item/domain/repository.go",
		"internal/order_item/application/command.go",
		"internal/order_item/application/dto.go",
		"internal/order_item/application/service.go",
		"internal/order_item/application/service_test.go",
		"internal/order_item/infrastructure/persistence/gorm/model.go",
		"internal/order_item/infrastructure/persistence/gorm/mapper.go",
		"internal/order_item/infrastructure/persistence/gorm/repository.go",
		"internal/order_item/infrastructure/persistence/gorm/migrate.go",
		"internal/order_item/infrastructure/persistence/mongo/document.go",
		"internal/order_item/infrastructure/persistence/mongo/mapper.go",
		"internal/order_item/infrastructure/persistence/mongo/repository.go",
		"internal/order_item/interfaces/rpc/server.go",
		"internal/gateway/svc/context.go",
		"internal/gateway/interfaces/http/order_item/request.go",
		"internal/gateway/interfaces/http/order_item/handler.go",
		"internal/gateway/interfaces/http/order_item/routes.go",
		"docs/services/order_item.md",
	}
	for _, rel := range serviceFiles {
		require.FileExists(t, filepath.Join(root, rel), rel)
	}

	oldPaths := []string{
		"internal/order_item/model",
		"internal/order_item/dto",
		"internal/order_item/service",
		"internal/order_item/repo",
		"internal/order_item/handler",
		"internal/gateway/request/order_item_request.go",
		"internal/gateway/handler/order_item_handler.go",
		"internal/gateway/router/order_item_routes.go",
	}
	for _, rel := range oldPaths {
		requireNoPath(t, filepath.Join(root, rel))
	}

	mainFile := readString(t, filepath.Join(root, "cmd/order_item/main.go"))
	require.Contains(t, mainFile, "internal/order_item/application")
	require.Contains(t, mainFile, "internal/order_item/infrastructure/persistence/gorm")
	require.Contains(t, mainFile, "internal/order_item/interfaces/rpc")
	require.Contains(t, mainFile, "orderitemv1.RegisterOrderItemServiceServer")

	domainFile := readString(t, filepath.Join(root, "internal/order_item/domain/entity.go"))
	require.Contains(t, domainFile, "type OrderItem struct")
	require.Contains(t, domainFile, "func NewOrderItem")

	repositoryFile := readString(t, filepath.Join(root, "internal/order_item/domain/repository.go"))
	require.Contains(t, repositoryFile, "type Repository interface")
	require.Contains(t, repositoryFile, "List(ctx context.Context, offset int, limit int)")

	applicationFile := readString(t, filepath.Join(root, "internal/order_item/application/service.go"))
	require.Contains(t, applicationFile, "repo domain.Repository")
	require.Contains(t, applicationFile, "func (s *Service) Create")

	gormModel := readString(t, filepath.Join(root, "internal/order_item/infrastructure/persistence/gorm/model.go"))
	require.Contains(t, gormModel, "type OrderItemModel struct")
	require.Contains(t, gormModel, "func (OrderItemModel) TableName() string")

	gormRepo := readString(t, filepath.Join(root, "internal/order_item/infrastructure/persistence/gorm/repository.go"))
	require.Contains(t, gormRepo, "var _ domain.Repository = (*Repository)(nil)")
	require.NotContains(t, gormRepo, "type OrderItemModel struct")

	mongoDoc := readString(t, filepath.Join(root, "internal/order_item/infrastructure/persistence/mongo/document.go"))
	require.Contains(t, mongoDoc, "type OrderItemDocument struct")
	require.Contains(t, mongoDoc, "func (OrderItemDocument) MongoCollectionName() string")

	rpcFile := readString(t, filepath.Join(root, "internal/order_item/interfaces/rpc/server.go"))
	require.Contains(t, rpcFile, "package rpc")
	require.Contains(t, rpcFile, "svc *application.Service")

	gatewayRoutes := readString(t, filepath.Join(root, "internal/gateway/interfaces/http/order_item/routes.go"))
	require.Contains(t, gatewayRoutes, "func RegisterRoutes(v1 *gin.RouterGroup, ctx *svc.ServiceContext)")
	require.Contains(t, gatewayRoutes, "routes := v1.Group(\"/order_items\")")

	v1File := readString(t, filepath.Join(root, "internal/gateway/router/v1.go"))
	require.Contains(t, v1File, "func registerAPIRoutes(r *gin.Engine, ctx *svc.ServiceContext)")
	require.Contains(t, v1File, "orderitemhttp.RegisterRoutes(v1, ctx)")
}

func writeMinimalProject(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"go.mod": "module github.com/acme/app\n\ngo 1.25.0\n",
		"Makefile": `GO ?= go

.PHONY: proto test

proto:
	$(GO) run ./tools/protogen

test:
	$(GO) test ./...
`,
		"configs/config.yaml": `app:
  name: demo

grpc:
  host: 0.0.0.0

database:
  driver: sqlite
`,
		"tools/protogen/main.go": "package main\n",
		"cmd/gateway/main.go":    "package main\n",
		"internal/gateway/router/router.go": `package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/acme/app/pkg/config"
	"github.com/acme/app/pkg/middleware"
)

func New(log *zap.Logger, middlewareCfg config.MiddlewareConfig) *gin.Engine {
	r := gin.New()
	r.OPTIONS("/*path", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	registerAPIRoutes(r)
	return r
}
`,
		"internal/gateway/router/v1.go": `package router

import "github.com/gin-gonic/gin"

func registerAPIRoutes(r *gin.Engine) {
	api := r.Group("/api")
	v1 := api.Group("/v1")
	_ = v1
}
`,
	}
	for rel, content := range files {
		path := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
}

func readString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}
