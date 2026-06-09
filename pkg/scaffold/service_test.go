package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNextServicePortUsesMaxConfiguredServicePortPlusOne(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "configs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "configs", "config.yaml"), []byte(`
services:
  user:
    port: 9001
  note:
    port: 9002
  order:
    port: 9100
`), 0o644))

	port, err := nextServicePort(root)

	require.NoError(t, err)
	require.Equal(t, 9101, port)
}

func TestAddServiceConfigAppendsServiceBlock(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "configs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "configs", "config.yaml"), []byte(`
app:
  name: xiaolanshu

grpc:
  host: 0.0.0.0

database:
  driver: sqlite
`), 0o644))

	err := addServiceConfig(root, serviceTemplateData{
		Dir:         "comment",
		ServiceName: "comment-service",
		Port:        9101,
	})

	require.NoError(t, err)
	content, err := os.ReadFile(filepath.Join(root, "configs", "config.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(content), "services:\n  comment:\n    name: comment-service\n    port: 9101\n    target: 127.0.0.1:9101\n")
	require.Contains(t, string(content), "\ndatabase:\n  driver: sqlite")
}

func TestEnsureGatewayClientsFileCreatesClientForGeneratedService(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "gateway", "client", "clients.go")

	err := ensureGatewayClientsFile(path, serviceTemplateData{
		Module:    "example.com/app",
		Dir:       "comment",
		GoPackage: "commentv1",
		GoIdent:   "comment",
		Pascal:    "Comment",
		Port:      9101,
	})

	require.NoError(t, err)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(content)
	require.Contains(t, text, `commentv1 "example.com/app/api/gen/comment/v1"`)
	require.Contains(t, text, "Comment  commentv1.CommentServiceClient")
	require.Contains(t, text, `commentTarget := cfg.ServiceTarget("comment")`)
	require.Contains(t, text, "Comment:  commentv1.NewCommentServiceClient(commentConn)")
}

func TestEnsureGatewayClientsFilePatchesExistingClients(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "gateway", "client", "clients.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(`package client

import (
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	userv1 "example.com/app/api/gen/user/v1"
	"example.com/app/pkg/config"
)

type Clients struct {
	User   userv1.UserServiceClient
	Config *config.Config

	conns []*grpc.ClientConn
}

func New(cfg *config.Config, log *zap.Logger) (*Clients, error) {
	userTarget := cfg.ServiceTarget("user")
	userConn, err := grpc.Dial(userTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial user service: %w", err)
	}

	log.Info("grpc clients initialized",
		zap.String("user_target", userTarget),
	)
	return &Clients{
		User:   userv1.NewUserServiceClient(userConn),
		Config: cfg,
		conns:  []*grpc.ClientConn{userConn},
	}, nil
}
`), 0o644))

	err := ensureGatewayClientsFile(path, serviceTemplateData{
		Module:    "example.com/app",
		Dir:       "comment",
		GoPackage: "commentv1",
		GoIdent:   "comment",
		Pascal:    "Comment",
		Port:      9101,
	})

	require.NoError(t, err)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(content)
	require.Contains(t, text, `commentv1 "example.com/app/api/gen/comment/v1"`)
	require.Contains(t, text, "Comment  commentv1.CommentServiceClient")
	require.Contains(t, text, `commentTarget := cfg.ServiceTarget("comment")`)
	require.Contains(t, text, "userConn.Close()")
	require.Contains(t, text, "Comment:  commentv1.NewCommentServiceClient(commentConn)")
	require.Contains(t, text, "conns:  []*grpc.ClientConn{userConn, commentConn}")
}

func TestPatchGatewayRouterWiresGeneratedClient(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "cmd", "gateway"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "gateway", "router"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "cmd", "gateway", "main.go"), []byte(cleanGatewayMain("example.com/app")), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "gateway", "router", "router.go"), []byte(cleanRouter("example.com/app")), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "gateway", "router", "v1.go"), []byte(cleanV1Router()), 0o644))
	data := serviceTemplateData{
		Module:    "example.com/app",
		Dir:       "comment",
		GoPackage: "commentv1",
		GoIdent:   "comment",
		Pascal:    "Comment",
		Port:      9101,
	}

	require.NoError(t, ensureGatewayClientsFile(filepath.Join(root, "internal", "gateway", "client", "clients.go"), data))
	require.NoError(t, patchGatewayRouter(root, data))

	mainContent, err := os.ReadFile(filepath.Join(root, "cmd", "gateway", "main.go"))
	require.NoError(t, err)
	require.Contains(t, string(mainContent), `"example.com/app/internal/gateway/client"`)
	require.Contains(t, string(mainContent), "gatewayClients, err := client.New(cfg, log)")
	require.Contains(t, string(mainContent), "engine := router.New(gatewayClients, log, cfg.Middleware)")

	routerContent, err := os.ReadFile(filepath.Join(root, "internal", "gateway", "router", "router.go"))
	require.NoError(t, err)
	require.Contains(t, string(routerContent), `"example.com/app/internal/gateway/client"`)
	require.Contains(t, string(routerContent), "func New(clients *client.Clients, log *zap.Logger, middlewareCfg config.MiddlewareConfig) *gin.Engine")
	require.Contains(t, string(routerContent), "registerAPIRoutes(r, clients, log)")

	v1Content, err := os.ReadFile(filepath.Join(root, "internal", "gateway", "router", "v1.go"))
	require.NoError(t, err)
	require.Contains(t, string(v1Content), "registerCommentRoutes(v1, handler.NewCommentHandler(clients.Comment, log))")
}

func TestAddServiceWithGenerationPlanWritesRelationshipHelpers(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n\ngo 1.25\n"), 0o644))
	planPath := filepath.Join(root, "scaffold-plans", "product.json")
	require.NoError(t, SaveGenerationPlan(planPath, GenerationPlan{
		ServiceName: "product",
		RootTable:   "product_spu",
		Tables:      []string{"product_spu", "product_sku"},
		Relationships: []TableRelationship{{
			Type:       RelationshipOneToMany,
			FromTable:  "product_sku",
			FromColumn: "spu_id",
			ToTable:    "product_spu",
			ToColumn:   "id",
		}},
	}))

	err := AddService(ServiceOptions{
		RootDir:  root,
		Name:     "product",
		PlanPath: planPath,
		RunProto: false,
	})

	require.NoError(t, err)
	content, err := os.ReadFile(filepath.Join(root, "internal", "product", "repo", "relationships.go"))
	require.NoError(t, err)
	text := string(content)
	require.Contains(t, text, "func SelectedTables() []string")
	require.Contains(t, text, `"product_spu"`)
	require.Contains(t, text, `"product_sku"`)
	require.Contains(t, text, "func (r *GormRepository) JoinProductSkuToProductSpu(ctx context.Context) *gorm.DB")
	require.Contains(t, text, `LEFT JOIN product_sku ON product_sku.spu_id = product_spu.id`)
}

func TestAddServiceWithTableGeneratesModelFromDatabaseColumns(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n\ngo 1.25\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "configs"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "data"), 0o755))
	dbPath := filepath.Join(root, "data", "shop.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`create table product_spu (
		id text primary key,
		spu_no text not null,
		title text not null,
		sale_price decimal(10,2),
		stock_count integer,
		is_online boolean,
		created_at datetime,
		updated_at datetime
	)`).Error)
	require.NoError(t, os.WriteFile(filepath.Join(root, "configs", "config.yaml"), []byte(`
database:
  driver: sqlite
  dsn: data/shop.db
`), 0o644))

	err = AddService(ServiceOptions{
		RootDir:   root,
		Name:      "product",
		TableName: "product_spu",
		RunProto:  false,
	})

	require.NoError(t, err)
	modelContent, err := os.ReadFile(filepath.Join(root, "internal", "product", "model", "product.go"))
	require.NoError(t, err)
	modelText := string(modelContent)
	require.Contains(t, modelText, "SpuNo")
	require.Contains(t, modelText, "`gorm:\"column:spu_no;not null\"`")
	require.Contains(t, modelText, "SalePrice")
	require.Contains(t, modelText, "`gorm:\"column:sale_price\"`")
	require.Contains(t, modelText, "StockCount")
	require.Contains(t, modelText, "`gorm:\"column:stock_count\"`")
	require.Contains(t, modelText, "IsOnline")
	require.Contains(t, modelText, "`gorm:\"column:is_online\"`")
	require.NotContains(t, modelText, "Description string")

	repoContent, err := os.ReadFile(filepath.Join(root, "internal", "product", "repo", "gorm_repository.go"))
	require.NoError(t, err)
	repoText := string(repoContent)
	require.Contains(t, repoText, "SpuNo:")
	require.Contains(t, repoText, "item.SpuNo")
	require.Contains(t, repoText, "SalePrice:")
	require.Contains(t, repoText, "item.SalePrice")
	require.NotContains(t, repoText, "Description: item.Description")

	protoContent, err := os.ReadFile(filepath.Join(root, "api", "proto", "product", "v1", "product.proto"))
	require.NoError(t, err)
	protoText := string(protoContent)
	require.Contains(t, protoText, "string id = 1;")
	require.Contains(t, protoText, "string spu_no = 2;")
	require.Contains(t, protoText, "double sale_price = 4;")
	require.NotContains(t, protoText, "description")
}
