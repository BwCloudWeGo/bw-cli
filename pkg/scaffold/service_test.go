package scaffold_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/scaffold"
)

func TestAddServiceWritesCompleteServiceStructure(t *testing.T) {
	root := t.TempDir()
	writeServiceProject(t, root)

	err := scaffold.AddService(scaffold.ServiceOptions{
		RootDir:  root,
		Name:     "order-item",
		Port:     9103,
		RunProto: false,
	})

	require.NoError(t, err)
	require.FileExists(t, filepath.Join(root, "cmd", "order_item", "main.go"))
	require.FileExists(t, filepath.Join(root, "internal", "order_item", "model", "order_item.go"))
	require.FileExists(t, filepath.Join(root, "internal", "order_item", "model", "repository.go"))
	require.FileExists(t, filepath.Join(root, "internal", "order_item", "dto", "command.go"))
	require.FileExists(t, filepath.Join(root, "internal", "order_item", "dto", "order_item.go"))
	require.FileExists(t, filepath.Join(root, "internal", "order_item", "service", "service.go"))
	require.FileExists(t, filepath.Join(root, "internal", "order_item", "service", "service_test.go"))
	require.FileExists(t, filepath.Join(root, "internal", "order_item", "repo", "gorm_repository.go"))
	require.FileExists(t, filepath.Join(root, "internal", "order_item", "repo", "mongo_repository.go"))
	require.FileExists(t, filepath.Join(root, "internal", "order_item", "handler", "server.go"))
	require.FileExists(t, filepath.Join(root, "internal", "gateway", "request", "order_item_request.go"))
	require.FileExists(t, filepath.Join(root, "internal", "gateway", "handler", "order_item_handler.go"))
	require.FileExists(t, filepath.Join(root, "internal", "gateway", "handler", "common.go"))
	require.FileExists(t, filepath.Join(root, "internal", "gateway", "router", "order_item_routes.go"))
	require.FileExists(t, filepath.Join(root, "api", "proto", "order_item", "v1", "order_item.proto"))
	require.FileExists(t, filepath.Join(root, "docs", "services", "order_item.md"))

	proto := readString(t, filepath.Join(root, "api", "proto", "order_item", "v1", "order_item.proto"))
	require.Contains(t, proto, "package order_item.v1;")
	require.Contains(t, proto, "option go_package = \"github.com/acme/app/api/gen/order_item/v1;orderitemv1\";")
	require.Contains(t, proto, "service OrderItemService")
	require.Contains(t, proto, "rpc CreateOrderItem")
	require.Contains(t, proto, "rpc GetOrderItem")
	require.Contains(t, proto, "rpc ListOrderItems")
	require.Contains(t, proto, "rpc UpdateOrderItem")
	require.Contains(t, proto, "rpc DeleteOrderItem")
	require.Contains(t, proto, "message OrderItemResponse")

	mainFile := readString(t, filepath.Join(root, "cmd", "order_item", "main.go"))
	require.Contains(t, mainFile, "const serviceName = \"order-item-service\"")
	require.Contains(t, mainFile, "const defaultGRPCPort = 9103")
	require.Contains(t, mainFile, "const grpcPortEnv = \"APP_ORDER_ITEM_GRPC_PORT\"")
	require.Contains(t, mainFile, "orderitemv1.RegisterOrderItemServiceServer")
	require.Contains(t, mainFile, "internal/order_item/service")

	modelFile := readString(t, filepath.Join(root, "internal", "order_item", "model", "order_item.go"))
	require.Contains(t, modelFile, "Name        string")
	require.Contains(t, modelFile, "Description string")
	require.Contains(t, modelFile, "func NewOrderItem")
	require.Contains(t, modelFile, "func (item *OrderItem) Update")

	repositoryFile := readString(t, filepath.Join(root, "internal", "order_item", "model", "repository.go"))
	require.Contains(t, repositoryFile, "Save(ctx context.Context, item *OrderItem) error")
	require.Contains(t, repositoryFile, "FindByID(ctx context.Context, id string) (*OrderItem, error)")
	require.Contains(t, repositoryFile, "List(ctx context.Context, offset int, limit int) ([]*OrderItem, int64, error)")
	require.Contains(t, repositoryFile, "Delete(ctx context.Context, id string) error")

	commandFile := readString(t, filepath.Join(root, "internal", "order_item", "dto", "command.go"))
	require.Contains(t, commandFile, "type CreateCommand struct")
	require.Contains(t, commandFile, "type UpdateCommand struct")
	require.Contains(t, commandFile, "type ListCommand struct")

	dtoFile := readString(t, filepath.Join(root, "internal", "order_item", "dto", "order_item.go"))
	require.Contains(t, dtoFile, "type OrderItemDTO struct")
	require.Contains(t, dtoFile, "type ListOrderItemDTO struct")
	require.Contains(t, dtoFile, "func FromOrderItem")

	serviceFile := readString(t, filepath.Join(root, "internal", "order_item", "service", "service.go"))
	require.NotContains(t, serviceFile, "type CreateCommand struct")
	require.NotContains(t, serviceFile, "type OrderItemDTO struct")
	require.NotContains(t, serviceFile, "func FromOrderItem")
	require.Contains(t, serviceFile, "func (s *Service) Create")
	require.Contains(t, serviceFile, "func (s *Service) Get")
	require.Contains(t, serviceFile, "func (s *Service) List")
	require.Contains(t, serviceFile, "func (s *Service) Update")
	require.Contains(t, serviceFile, "func (s *Service) Delete")

	repoFile := readString(t, filepath.Join(root, "internal", "order_item", "repo", "gorm_repository.go"))
	require.Contains(t, repoFile, "func (r *GormRepository) Save")
	require.Contains(t, repoFile, "func (r *GormRepository) List")
	require.Contains(t, repoFile, "func (r *GormRepository) Delete")

	mongoRepoFile := readString(t, filepath.Join(root, "internal", "order_item", "repo", "mongo_repository.go"))
	require.Contains(t, mongoRepoFile, "func (OrderItemDocument) MongoCollectionName() string")
	require.Contains(t, mongoRepoFile, "mongox.NewDocumentStore[OrderItemDocument](db, log)")
	require.Contains(t, mongoRepoFile, "documents *mongox.DocumentStore[OrderItemDocument]")
	require.Contains(t, mongoRepoFile, "func (r *MongoRepository) Save")
	require.Contains(t, mongoRepoFile, "func (r *MongoRepository) List")
	require.Contains(t, mongoRepoFile, "func (r *MongoRepository) Delete")

	handlerFile := readString(t, filepath.Join(root, "internal", "order_item", "handler", "server.go"))
	require.Contains(t, handlerFile, "func (s *Server) CreateOrderItem")
	require.Contains(t, handlerFile, "func (s *Server) GetOrderItem")
	require.Contains(t, handlerFile, "func (s *Server) ListOrderItems")
	require.Contains(t, handlerFile, "func (s *Server) UpdateOrderItem")
	require.Contains(t, handlerFile, "func (s *Server) DeleteOrderItem")

	gatewayRequestFile := readString(t, filepath.Join(root, "internal", "gateway", "request", "order_item_request.go"))
	require.Contains(t, gatewayRequestFile, "type CreateOrderItemRequest struct")
	require.Contains(t, gatewayRequestFile, "type UpdateOrderItemRequest struct")
	require.Contains(t, gatewayRequestFile, "type ListOrderItemRequest struct")

	gatewayHandlerFile := readString(t, filepath.Join(root, "internal", "gateway", "handler", "order_item_handler.go"))
	require.Contains(t, gatewayHandlerFile, "const orderItemGatewayTargetEnv = \"APP_ORDER_ITEM_GRPC_TARGET\"")
	require.Contains(t, gatewayHandlerFile, "func NewOrderItemHandler")
	require.Contains(t, gatewayHandlerFile, "func (h *OrderItemHandler) Create")
	require.Contains(t, gatewayHandlerFile, "func (h *OrderItemHandler) List")
	require.Contains(t, gatewayHandlerFile, "orderitemv1.NewOrderItemServiceClient")

	gatewayRoutesFile := readString(t, filepath.Join(root, "internal", "gateway", "router", "order_item_routes.go"))
	require.Contains(t, gatewayRoutesFile, "func registerOrderItemRoutes")
	require.Contains(t, gatewayRoutesFile, "routes := v1.Group(\"/order_items\")")
	require.Contains(t, gatewayRoutesFile, "routes.POST")
	require.Contains(t, gatewayRoutesFile, "routes.DELETE")

	routerFile := readString(t, filepath.Join(root, "internal", "gateway", "router", "router.go"))
	require.Contains(t, routerFile, "registerAPIRoutes(r, log)")

	v1File := readString(t, filepath.Join(root, "internal", "gateway", "router", "v1.go"))
	require.Contains(t, v1File, "func registerAPIRoutes(r *gin.Engine, log *zap.Logger)")
	require.Contains(t, v1File, "registerOrderItemRoutes(v1, handler.NewOrderItemHandler(log))")

	makefile := readString(t, filepath.Join(root, "Makefile"))
	require.Contains(t, makefile, ".PHONY: proto test run-order_item")
	require.Contains(t, makefile, "run-order_item:")
	require.Contains(t, makefile, "$(GO) run ./cmd/order_item")

	doc := readString(t, filepath.Join(root, "docs", "services", "order_item.md"))
	require.Contains(t, doc, "bw-cli service order-item --port 9103")
	require.Contains(t, doc, "APP_ORDER_ITEM_GRPC_PORT")
	require.Contains(t, doc, "APP_ORDER_ITEM_GRPC_TARGET")
	require.Contains(t, doc, "基础 CRUD")
	require.Contains(t, doc, "数据库操作只放在 `internal/order_item/repo`")
	require.Contains(t, doc, "repo/mongo_repository.go")
	require.Contains(t, doc, "mongox.NewDocumentStore[OrderItemDocument]")
	require.Contains(t, doc, "/api/v1/order_items")
}

func TestAddServiceRejectsExistingService(t *testing.T) {
	root := t.TempDir()
	writeServiceProject(t, root)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "order"), 0o755))

	err := scaffold.AddService(scaffold.ServiceOptions{
		RootDir:  root,
		Name:     "order",
		RunProto: false,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}

func TestAddServicePatchesGatewayForMultipleServices(t *testing.T) {
	root := t.TempDir()
	writeServiceProject(t, root)

	require.NoError(t, scaffold.AddService(scaffold.ServiceOptions{
		RootDir:  root,
		Name:     "comment",
		Port:     9103,
		RunProto: false,
	}))
	require.NoError(t, scaffold.AddService(scaffold.ServiceOptions{
		RootDir:  root,
		Name:     "tag",
		Port:     9104,
		RunProto: false,
	}))

	v1File := readString(t, filepath.Join(root, "internal", "gateway", "router", "v1.go"))
	require.Contains(t, v1File, "registerCommentRoutes(v1, handler.NewCommentHandler(log))")
	require.Contains(t, v1File, "registerTagRoutes(v1, handler.NewTagHandler(log))")

	commonFile := readString(t, filepath.Join(root, "internal", "gateway", "handler", "common.go"))
	require.Contains(t, commonFile, "func gatewayGRPCTarget")

	commentHandler := readString(t, filepath.Join(root, "internal", "gateway", "handler", "comment_handler.go"))
	tagHandler := readString(t, filepath.Join(root, "internal", "gateway", "handler", "tag_handler.go"))
	require.NotContains(t, commentHandler, "func gatewayGRPCTarget")
	require.NotContains(t, tagHandler, "func gatewayGRPCTarget")
	require.Contains(t, tagHandler, "const tagGatewayTargetEnv = \"APP_TAG_GRPC_TARGET\"")
}

func TestAddServiceExtendsExistingGatewayCommon(t *testing.T) {
	root := t.TempDir()
	writeServiceProject(t, root)
	commonPath := filepath.Join(root, "internal", "gateway", "handler", "common.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(commonPath), 0o755))
	require.NoError(t, os.WriteFile(commonPath, []byte(`package handler

import (
	"context"

	"github.com/gin-gonic/gin"
)

func outgoingContext(c *gin.Context) context.Context {
	return c.Request.Context()
}
`), 0o644))

	require.NoError(t, scaffold.AddService(scaffold.ServiceOptions{
		RootDir:  root,
		Name:     "comment",
		Port:     9103,
		RunProto: false,
	}))

	commonFile := readString(t, commonPath)
	require.Contains(t, commonFile, "\"os\"")
	require.Contains(t, commonFile, "\"strings\"")
	require.Contains(t, commonFile, "func gatewayGRPCTarget")
}

func TestAddServiceRejectsInvalidName(t *testing.T) {
	root := t.TempDir()
	writeServiceProject(t, root)

	err := scaffold.AddService(scaffold.ServiceOptions{
		RootDir:  root,
		Name:     "../bad",
		RunProto: false,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "service name")
}

func writeServiceProject(t *testing.T, root string) {
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
		"tools/protogen/main.go": "package main\n",
		"internal/gateway/router/router.go": `package router

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/acme/app/pkg/config"
)

func New(log *zap.Logger, middlewareCfg config.MiddlewareConfig) *gin.Engine {
	r := gin.New()
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
