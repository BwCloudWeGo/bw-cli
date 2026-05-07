package scaffold

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// InitOptions controls how bw-cli generates a new project from this scaffold.
type InitOptions struct {
	SourceDir   string
	TargetDir   string
	ModulePath  string
	RepoURL     string
	Branch      string
	RunTidy     bool
	IncludeDemo bool
}

// Init copies or clones the scaffold, then rewrites module paths for the target project.
func Init(opts InitOptions) error {
	if opts.TargetDir == "" {
		return errors.New("target dir is required")
	}
	if opts.ModulePath == "" {
		return errors.New("module path is required")
	}

	if opts.RepoURL != "" {
		if err := clone(opts); err != nil {
			return err
		}
	} else {
		if opts.SourceDir == "" {
			return errors.New("source dir or repo url is required")
		}
		if err := copyDir(opts.SourceDir, opts.TargetDir); err != nil {
			return err
		}
	}

	oldModule, err := readModule(filepath.Join(opts.TargetDir, "go.mod"))
	if err != nil {
		return err
	}
	if err := rewriteModule(opts.TargetDir, oldModule, opts.ModulePath); err != nil {
		return err
	}
	if err := removeScaffoldTooling(opts.TargetDir); err != nil {
		return err
	}
	if opts.IncludeDemo {
		if err := writeDemoDocs(opts.TargetDir, opts.ModulePath); err != nil {
			return err
		}
	} else {
		if err := stripDemo(opts.TargetDir, opts.ModulePath); err != nil {
			return err
		}
	}
	if opts.RunTidy {
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = opts.TargetDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("go mod tidy: %w", err)
		}
	}
	return nil
}

func clone(opts InitOptions) error {
	args := []string{"clone", "--depth", "1"}
	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}
	args = append(args, opts.RepoURL, opts.TargetDir)
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	// Generated projects should not inherit the scaffold repository remote.
	return os.RemoveAll(filepath.Join(opts.TargetDir, ".git"))
}

func copyDir(source string, target string) error {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source %s is not a directory", source)
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(target, 0o755)
		}
		if shouldSkip(rel, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		return copyFile(path, dest)
	})
}

func shouldSkip(rel string, entry os.DirEntry) bool {
	name := entry.Name()
	if name == ".git" || name == ".idea" || name == "data" || name == "logs" || name == "tmp" || name == ".DS_Store" {
		return true
	}
	if rel == filepath.Join("docs", "superpowers") {
		return true
	}
	if strings.HasSuffix(name, ".log") {
		return true
	}
	return false
}

func removeScaffoldTooling(root string) error {
	for _, rel := range []string{
		filepath.Join("cmd", "bw-cli"),
		filepath.Join("pkg", "scaffold"),
	} {
		if err := os.RemoveAll(filepath.Join(root, rel)); err != nil {
			return err
		}
	}
	return nil
}

func stripDemo(root string, module string) error {
	for _, item := range []struct {
		rel  string
		keep []string
	}{
		{rel: "cmd", keep: []string{"gateway"}},
		{rel: "internal", keep: []string{"gateway"}},
		{rel: filepath.Join("api", "proto")},
		{rel: filepath.Join("api", "gen")},
	} {
		if err := removeChildrenExcept(filepath.Join(root, item.rel), item.keep...); err != nil {
			return err
		}
	}
	for _, rel := range []string{
		filepath.Join("internal", "gateway", "client"),
		filepath.Join("internal", "gateway", "handler"),
		filepath.Join("internal", "gateway", "request"),
		filepath.Join("internal", "gateway", "router", "user_routes.go"),
		filepath.Join("internal", "gateway", "router", "note_routes.go"),
		filepath.Join("internal", "gateway", "router", "router_test.go"),
	} {
		if err := os.RemoveAll(filepath.Join(root, rel)); err != nil {
			return err
		}
	}
	if err := writeCleanGateway(root, module); err != nil {
		return err
	}
	if err := writeCleanMakefile(root); err != nil {
		return err
	}
	if err := writeCleanConfig(root); err != nil {
		return err
	}
	if err := writeCleanDocs(root, module); err != nil {
		return err
	}
	return nil
}

func removeChildrenExcept(dir string, keep ...string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	keepSet := make(map[string]struct{}, len(keep))
	for _, name := range keep {
		keepSet[name] = struct{}{}
	}
	for _, entry := range entries {
		if _, ok := keepSet[entry.Name()]; ok {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func writeCleanGateway(root string, module string) error {
	if exists(filepath.Join(root, "cmd", "gateway")) {
		if err := os.WriteFile(filepath.Join(root, "cmd", "gateway", "main.go"), []byte(cleanGatewayMain(module)), 0o644); err != nil {
			return err
		}
	}
	routerDir := filepath.Join(root, "internal", "gateway", "router")
	if exists(routerDir) {
		if err := os.WriteFile(filepath.Join(routerDir, "router.go"), []byte(cleanRouter(module)), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(routerDir, "v1.go"), []byte(cleanV1Router()), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writeCleanMakefile(root string) error {
	path := filepath.Join(root, "Makefile")
	if !exists(path) {
		return nil
	}
	return os.WriteFile(path, []byte(cleanMakefile()), 0o644)
}

func writeCleanConfig(root string) error {
	path := filepath.Join(root, "configs", "config.yaml")
	if !exists(path) {
		return nil
	}
	return os.WriteFile(path, []byte(cleanConfigYAML()), 0o644)
}

func writeCleanDocs(root string, module string) error {
	if exists(filepath.Join(root, "README.md")) {
		if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(cleanREADME(module)), 0o644); err != nil {
			return err
		}
	}
	docsDir := filepath.Join(root, "docs")
	if !exists(docsDir) {
		return nil
	}
	if err := os.WriteFile(filepath.Join(docsDir, "usage.md"), []byte(cleanUsageDoc(module)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(docsDir, "architecture.md"), []byte(cleanArchitectureDoc()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(docsDir, "toolkit.md"), []byte(generatedToolkitDoc(module)), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(docsDir, "mongodb.md"), []byte(generatedMongoDBDoc(module)), 0o644)
}

func writeDemoDocs(root string, module string) error {
	if exists(filepath.Join(root, "README.md")) {
		if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(demoREADME(module)), 0o644); err != nil {
			return err
		}
	}
	docsDir := filepath.Join(root, "docs")
	if !exists(docsDir) {
		return nil
	}
	if err := os.WriteFile(filepath.Join(docsDir, "usage.md"), []byte(demoUsageDoc(module)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(docsDir, "architecture.md"), []byte(demoArchitectureDoc()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(docsDir, "toolkit.md"), []byte(generatedToolkitDoc(module)), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(docsDir, "mongodb.md"), []byte(generatedMongoDBDoc(module)), 0o644)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func cleanGatewayMain(module string) string {
	return fmt.Sprintf(`package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"%s/internal/gateway/router"
	"%s/pkg/config"
	"%s/pkg/logger"
	"%s/pkg/observability"
)

func main() {
	// Load runtime settings from YAML/env before constructing dependencies.
	if err := config.InitGlobal("configs/config.yaml"); err != nil {
		panic(err)
	}
	cfg := config.MustGlobal()
	cfg.Log.Service = cfg.App.GatewayServiceName
	cfg.Log = logger.WithDailyFileName(cfg.Log, time.Now())

	log, err := logger.New(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer log.Sync()
	observability.Register(cfg.App.GatewayServiceName, log)

	engine := router.New(log, cfg.Middleware)
	addr := fmt.Sprintf("%%s:%%d", cfg.HTTP.Host, cfg.HTTP.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      engine,
		ReadTimeout:  time.Duration(cfg.HTTP.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.HTTP.WriteTimeoutSeconds) * time.Second,
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		printStartupFailure(addr, err)
		log.Fatal("gateway listen failed", zap.String("addr", addr), zap.Error(err))
	}
	printStartupSummary(cfg, addr)

	go func() {
		log.Info("gateway listening", zap.String("addr", addr))
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatal("gateway stopped unexpectedly", zap.Error(err))
		}
	}()

	waitForShutdown(server, log)
}

func printStartupFailure(addr string, err error) {
	fmt.Fprintf(os.Stderr, "\n[Gateway Start Failed]\n")
	fmt.Fprintf(os.Stderr, "  listen: %%s\n", addr)
	fmt.Fprintf(os.Stderr, "  error: %%v\n\n", err)
}

func printStartupSummary(cfg *config.Config, addr string) {
	host := cfg.HTTP.Host
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	baseURL := fmt.Sprintf("http://%%s:%%d", host, cfg.HTTP.Port)
	fmt.Fprintf(os.Stdout, "\n[Gateway Started]\n")
	fmt.Fprintf(os.Stdout, "  service: %%s\n", cfg.App.GatewayServiceName)
	fmt.Fprintf(os.Stdout, "  env: %%s\n", cfg.App.Env)
	fmt.Fprintf(os.Stdout, "  listen: %%s\n", addr)
	fmt.Fprintf(os.Stdout, "  http: %%s\n", baseURL)
	fmt.Fprintf(os.Stdout, "  health: %%s/healthz\n", baseURL)
	fmt.Fprintf(os.Stdout, "  api: %%s/api/v1\n\n", baseURL)
}

func waitForShutdown(server *http.Server, log *zap.Logger) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Info("gateway shutting down")
	if err := server.Shutdown(ctx); err != nil {
		log.Error("gateway shutdown failed", zap.Error(err))
	}
}
`, module, module, module, module)
}

func cleanRouter(module string) string {
	return fmt.Sprintf(`// Package router owns Gin engine construction and route registration.
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"%s/pkg/config"
	"%s/pkg/middleware"
)

// New builds the gateway Gin engine with configured middleware and versioned API routes.
func New(log *zap.Logger, middlewareCfg config.MiddlewareConfig) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(
		middleware.CORS(middlewareCfg.CORS),
		middleware.RequestID(),
		middleware.RequestLogger(log),
		gin.Recovery(),
	)
	r.OPTIONS("/*path", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	registerHealthRoutes(r)
	registerAPIRoutes(r)
	return r
}
`, module, module)
}

func cleanV1Router() string {
	return `package router

import "github.com/gin-gonic/gin"

// registerAPIRoutes creates the /api/v1 route namespace.
// Add business-specific route files beside this file as services are introduced.
func registerAPIRoutes(r *gin.Engine) {
	api := r.Group("/api")
	v1 := api.Group("/v1")
	_ = v1
}
`
}

func cleanMakefile() string {
	return `GO ?= go
PROTOC ?= protoc
PROTO_PATH ?= api/proto
PROTO_OUT ?= api/gen

export PROTOC
export PROTO_PATH
export PROTO_OUT

.PHONY: proto test tidy run-gateway tools

tools:
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

proto:
	$(GO) run ./tools/protogen

test:
	$(GO) test ./...

tidy:
	$(GO) mod tidy

run-gateway:
	$(GO) run ./cmd/gateway
`
}

func cleanConfigYAML() string {
	return `app:
  name: app
  env: local
  gateway_service_name: gateway

http:
  host: 0.0.0.0
  port: 8080
  read_timeout_seconds: 5
  write_timeout_seconds: 10

grpc:
  host: 0.0.0.0

database:
  driver: sqlite
  dsn: data/app.db

mysql:
  dsn: ""
  max_idle_conns: 10
  max_open_conns: 100
  conn_max_lifetime_seconds: 3600

postgresql:
  dsn: ""
  max_idle_conns: 10
  max_open_conns: 100
  conn_max_lifetime_seconds: 3600

mongodb:
  uri: mongodb://127.0.0.1:27017
  username: ""
  password: ""
  database: app
  app_name: bw-cli
  min_pool_size: 0
  max_pool_size: 100
  connect_timeout_seconds: 10
  server_selection_timeout_seconds: 5

file_storage:
  provider: minio
  max_size_mb: 100
  object_prefix: uploads
  public_base_url: ""
  allowed_extensions:
    - .doc
    - .docx
    - .pdf
    - .jpg
    - .jpeg
    - .png
    - .gif
    - .webp
    - .bmp
    - .svg
    - .mp4
    - .mov
    - .avi
    - .mkv
    - .webm
    - .mp3
    - .wav
    - .ogg
    - .m4a
    - .flac
    - .aac
  allowed_content_types:
    - application/msword
    - application/vnd.openxmlformats-officedocument.wordprocessingml.document
    - application/pdf
    - image/jpeg
    - image/png
    - image/gif
    - image/webp
    - image/bmp
    - image/svg+xml
    - video/mp4
    - video/quicktime
    - video/x-msvideo
    - video/x-matroska
    - video/webm
    - audio/mpeg
    - audio/wav
    - audio/x-wav
    - audio/ogg
    - audio/mp4
    - audio/flac
    - audio/aac
  minio:
    endpoint: ""
    access_key_id: ""
    secret_access_key: ""
    bucket: ""
    region: ""
    use_ssl: false
  oss:
    endpoint: ""
    access_key_id: ""
    access_key_secret: ""
    bucket: ""
  qiniu:
    access_key: ""
    secret_key: ""
    bucket: ""
    region: ""
    use_https: true
    use_cdn_domains: false
  cos:
    secret_id: ""
    secret_key: ""
    bucket: ""
    region: ""
    bucket_url: ""

redis:
  addr: 127.0.0.1:6379
  username: ""
  password: ""
  db: 0
  pool_size: 10

elasticsearch:
  addresses:
    - http://127.0.0.1:9200
  username: ""
  password: ""

kafka:
  brokers:
    - 127.0.0.1:9092
  topic: app-events
  group_id: app-consumer

middleware:
  cors:
    allow_origins:
      - "*"
    allow_methods:
      - GET
      - POST
      - PUT
      - PATCH
      - DELETE
      - OPTIONS
    allow_headers:
      - Origin
      - Content-Type
      - Authorization
      - X-Request-ID
    allow_credentials: false
  jwt:
    secret: ""
    issuer: app
    expire_seconds: 7200

log:
  service: app
  environment: local
  level: info
  encoding: json
  file:
    enabled: true
    filename: logs/app.log
    max_size_mb: 128
    max_backups: 14
    max_age_days: 7
    compress: true
`
}

func cleanREADME(module string) string {
	return fmt.Sprintf(`# Go 微服务项目

本项目由 `+"`bw-cli new`"+` 生成，默认是不带业务 demo 的干净框架。当前 module：

`+"```text"+`
%s
`+"```"+`

## 快速启动

安装依赖并验证：

`+"```bash"+`
make tidy
make proto
make test
`+"```"+`

Makefile 只调用 Go 命令，不依赖 `+"`find`"+`、`+"`sed`"+`、`+"`if [ ... ]`"+` 等 Unix shell 语法；`+"`make proto`"+` 会通过 `+"`tools/protogen`"+` 自动适配 Windows、macOS、Linux。
Windows 仍需要安装 GNU Make；如果没有 `+"`make`"+`，可以直接执行等价的 Go 命令，例如 `+"`go run ./tools/protogen`"+`、`+"`go test ./...`"+`、`+"`go run ./cmd/gateway`"+`。

启动 HTTP gateway：

`+"```bash"+`
make run-gateway
`+"```"+`

端口监听成功后，控制台会输出：

`+"```text"+`
[Gateway Started]
  service: gateway
  env: local
  listen: 0.0.0.0:8080
  http: http://127.0.0.1:8080
  health: http://127.0.0.1:8080/healthz
  api: http://127.0.0.1:8080/api/v1
`+"```"+`

如果端口被占用，控制台会输出 `+"`[Gateway Start Failed]`"+`，并显示失败的监听地址和系统错误。

健康检查：

`+"```bash"+`
curl http://localhost:8080/healthz
`+"```"+`

## 目录结构

`+"```text"+`
api/proto      # 新增 gRPC proto 时放这里
api/gen        # protoc 生成代码
cmd/gateway    # Gin HTTP 网关入口
configs        # YAML 配置
internal       # 业务代码，按服务拆分
pkg            # 公共工具包
docs           # 使用和架构文档
`+"```"+`

新增业务服务时使用脚手架命令：

`+"```bash"+`
bw-cli service order --tidy
`+"```"+`

命令会生成 `+"`cmd/order`"+`、`+"`api/proto/order`"+`、`+"`internal/order/model`"+`、`+"`internal/order/dto`"+`、`+"`internal/order/service/service.go`"+`、`+"`repo/gorm_repository.go`"+`、`+"`repo/mongo_repository.go`"+`、`+"`handler`"+`、gateway request/handler/router 和 `+"`docs/services/order.md`"+`。生成后的服务默认带 Create/Get/List/Update/Delete 基础 CRUD，端口默认值写在 `+"`cmd/order/main.go`"+`，不需要修改 `+"`configs/config.yaml`"+`。默认启动使用 Gorm，MongoDB 仓储已通过 `+"`mongox.NewDocumentStore[T]`"+` 预先接好。

需要指定端口时使用：

`+"```bash"+`
bw-cli service order --port 9103 --tidy
`+"```"+`

启动后控制台会输出服务名、环境、监听地址、gRPC 地址和端口环境变量。

如果项目包含 gateway，HTTP CRUD 路由也会自动挂载：

`+"```text"+`
POST   /api/v1/orders
GET    /api/v1/orders
GET    /api/v1/orders/:id
PUT    /api/v1/orders/:id
DELETE /api/v1/orders/:id
`+"```"+`

需要演示项目时请使用：

`+"```bash"+`
bw-cli demo demo-service --module github.com/acme/demo-service --tidy
`+"```"+`
`, module)
}

func cleanUsageDoc(module string) string {
	return fmt.Sprintf(`# 使用说明

当前项目是通过 `+"`bw-cli new`"+` 生成的干净框架，不包含业务 demo。

## 1. 初始化

`+"```bash"+`
cd <project>
make tidy
make proto
make test
`+"```"+`

如果当前还没有 proto 文件，`+"`make proto`"+` 会输出 `+"`No proto files found`"+` 并正常结束。
Makefile 只调用 Go 命令，不依赖 Unix shell 语法；`+"`make proto`"+` 会通过 `+"`tools/protogen`"+` 自动适配 Windows、macOS、Linux。
Windows 仍需要安装 GNU Make；如果没有 `+"`make`"+`，可以直接执行等价的 Go 命令，例如 `+"`go run ./tools/protogen`"+`、`+"`go test ./...`"+`、`+"`go run ./cmd/gateway`"+`。

## 2. 启动 gateway

`+"```bash"+`
make run-gateway
`+"```"+`

端口监听成功后，控制台会输出服务名、环境、监听地址、HTTP 地址、健康检查地址和 API 前缀。
如果端口被占用，控制台会输出 `+"`[Gateway Start Failed]`"+`，并显示失败的监听地址和系统错误。

默认监听：

`+"```text"+`
http://localhost:8080
`+"```"+`

健康检查：

`+"```bash"+`
curl http://localhost:8080/healthz
`+"```"+`

## 3. 配置

主配置文件：

`+"```text"+`
configs/config.yaml
`+"```"+`

环境变量覆盖规则：

`+"```text"+`
APP_ + 配置路径大写 + 下划线
`+"```"+`

示例：

`+"```bash"+`
export APP_HTTP_PORT=8081
export APP_LOG_LEVEL=debug
export APP_DATABASE_DRIVER=postgres
`+"```"+`

Windows PowerShell 使用：

`+"```powershell"+`
$env:APP_HTTP_PORT="8081"; make run-gateway
`+"```"+`

## 4. 当前 module

`+"```text"+`
%s
`+"```"+`

## 5. 查看公共工具

公共工具的详细调用流程见：

`+"```text"+`
docs/toolkit.md
`+"```"+`

MongoDB 教学文档见：

`+"```text"+`
docs/mongodb.md
`+"```"+`

## 6. 新增业务服务

进入项目根目录执行：

`+"```bash"+`
bw-cli service comment --tidy
`+"```"+`

不传 `+"`--port`"+` 时默认端口是 `+"`9100`"+`。需要指定端口时使用：

`+"```bash"+`
bw-cli service comment --port 9103 --tidy
`+"```"+`

生成后的服务不需要修改配置即可编译和启动：

- 服务名、默认 gRPC 端口和 `+"`APP_COMMENT_GRPC_PORT`"+` 环境变量写在 `+"`cmd/comment/main.go`"+`。
- 数据库继续读取当前项目已有的 `+"`database`"+`、`+"`mysql`"+`、`+"`postgresql`"+` 配置。
- 默认 SQLite 可直接本地运行，服务启动时自动执行 `+"`AutoMigrate`"+`。
- proto、handler、service、model、repo、gateway HTTP 入口和 service 单测都会同时生成。

生成结构：

`+"```text"+`
api/proto/comment/v1/comment.proto
api/gen/comment/v1
cmd/comment/main.go
internal/comment/model      # 领域实体、业务错误、Repository 接口
internal/comment/dto/command.go      # 业务用例入参命令
internal/comment/dto/comment.go      # 业务用例出参 DTO 和转换
internal/comment/service/service.go  # 业务流程编排
internal/comment/repo       # 数据库访问，默认 Gorm，同时生成 MongoDB 实现
internal/comment/handler    # gRPC 协议转换
internal/gateway/request/comment_request.go
internal/gateway/handler/comment_handler.go
internal/gateway/router/comment_routes.go
docs/services/comment.md    # 单服务详细开发说明
`+"```"+`

生成后的基础 CRUD 调用链：

`+"```text"+`
gRPC client -> proto -> handler -> service -> model.Repository -> repo(Gorm) -> database
`+"```"+`

默认启动使用 `+"`repo/gorm_repository.go`"+`。如果业务更适合 MongoDB，生成的 `+"`repo/mongo_repository.go`"+` 已经包含 `+"`MongoCollectionName()`"+`、`+"`mongox.NewDocumentStore[T]`"+` 和基础 CRUD 方法，只需要在服务 main 中改为注入 `+"`repo.NewMongoRepository`"+`。

HTTP 入口也已挂载：

`+"```text"+`
POST   /api/v1/comments
GET    /api/v1/comments
GET    /api/v1/comments/:id
PUT    /api/v1/comments/:id
DELETE /api/v1/comments/:id
`+"```"+`

默认提供：

`+"```text"+`
CreateComment
GetComment
ListComments
UpdateComment
DeleteComment
`+"```"+`

开发原则：

- `+"`model`"+` 写业务核心，不依赖 Gin、gRPC、Gorm。
- `+"`dto/command.go`"+` 写业务入参，`+"`dto/<service>.go`"+` 写业务出参，`+"`service/service.go`"+` 写业务流程。
- `+"`repo`"+` 是数据库操作唯一入口，Gorm/MongoDB/Redis 查询都放这里。
- `+"`handler`"+` 只做 gRPC request/response 转换和错误映射。
- HTTP 入参放 `+"`internal/gateway/request`"+`，路由按 `+"`/api/v1/<business>`"+` 拆分。

数据库操作示例见每次生成的 `+"`docs/services/<service>.md`"+`。
`, module)
}

func cleanArchitectureDoc() string {
	return `# 架构说明

本项目是干净微服务框架，默认只保留 HTTP gateway、配置、日志、中间件、数据库和外部组件封装，不包含业务 demo。

## 分层建议

新增服务时按以下包名组织：

~~~text
internal/<service>/model    # 实体、值对象、业务错误、仓储接口
internal/<service>/dto/command.go      # 业务用例入参命令
internal/<service>/dto/<service>.go    # 业务用例出参 DTO 和转换
internal/<service>/service/service.go  # 业务流程编排
internal/<service>/repo     # Gorm、MongoDB、Redis、外部依赖实现
internal/<service>/handler  # gRPC/HTTP 入站适配
~~~

依赖方向：

~~~text
handler -> service -> model
repo -> model
~~~

model 不依赖 Gin、gRPC、Gorm 或云 SDK，便于测试和替换基础设施。service 继续拆为 command、dto、service 三个文件：command 放业务入参，dto 放业务出参和转换，service 放业务流程。

## Gateway

Gateway 默认包含：

~~~text
cmd/gateway/main.go
internal/gateway/router/router.go
internal/gateway/router/health.go
internal/gateway/router/v1.go
~~~

路由按：

~~~text
版本 -> 业务 -> 具体接口
~~~

的方式扩展，例如：

~~~text
/api/v1/products
/api/v1/orders
~~~

## 公共能力

公共能力在 pkg 下：

~~~text
pkg/config
pkg/logger
pkg/errors
pkg/httpx
pkg/middleware
pkg/grpcx
pkg/database
pkg/mysqlx
pkg/postgresx
pkg/mongox
pkg/redisx
pkg/esx
pkg/kafkax
pkg/filex
pkg/validator
~~~
`
}

func demoREADME(module string) string {
	return fmt.Sprintf(`# Go 微服务演示项目

本项目由 `+"`bw-cli demo`"+` 生成，保留 user/note 两个示例服务，用于学习 Gin + gRPC + Gorm + DDD 的完整调用链。当前 module：

`+"```text"+`
%s
`+"```"+`

## 快速启动

初始化：

`+"```bash"+`
make tidy
make proto
make test
`+"```"+`

Makefile 只调用 Go 命令，不依赖 `+"`find`"+`、`+"`sed`"+`、`+"`if [ ... ]`"+` 等 Unix shell 语法；`+"`make proto`"+` 会通过 `+"`tools/protogen`"+` 自动适配 Windows、macOS、Linux。
Windows 仍需要安装 GNU Make；如果没有 `+"`make`"+`，可以直接执行等价的 Go 命令，例如 `+"`go run ./tools/protogen`"+`、`+"`go test ./...`"+`、`+"`go run ./cmd/gateway`"+`。

建议开三个终端启动：

`+"```bash"+`
make run-user
make run-note
make run-gateway
`+"```"+`

健康检查：

`+"```bash"+`
curl http://localhost:8080/healthz
`+"```"+`

示例接口：

`+"```bash"+`
curl -X POST http://localhost:8080/api/v1/users/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"ada@example.com","display_name":"Ada","password":"secret123"}'
`+"```"+`

## 目录结构

`+"```text"+`
api/proto      # user/note proto 源文件
api/gen        # protoc 生成代码
cmd/gateway    # Gin HTTP 网关入口
cmd/user       # user-service gRPC 入口
cmd/note       # note-service gRPC 入口
configs        # YAML 配置
internal       # user/note/gateway 业务代码
pkg            # 公共工具包
docs           # 使用和架构文档
`+"```"+`
`, module)
}

func demoUsageDoc(module string) string {
	return fmt.Sprintf(`# 使用说明

当前项目由 `+"`bw-cli demo`"+` 生成，包含 user-service、note-service 和 gateway，用于演示完整微服务调用链。

## 1. 初始化

`+"```bash"+`
cd <project>
make tidy
make proto
make test
`+"```"+`

Makefile 只调用 Go 命令，不依赖 Unix shell 语法；`+"`make proto`"+` 会通过 `+"`tools/protogen`"+` 自动适配 Windows、macOS、Linux。
Windows 仍需要安装 GNU Make；如果没有 `+"`make`"+`，可以直接执行等价的 Go 命令，例如 `+"`go run ./tools/protogen`"+`、`+"`go test ./...`"+`、`+"`go run ./cmd/gateway`"+`。

当前 module：

`+"```text"+`
%s
`+"```"+`

## 2. 启动服务

建议开三个终端：

`+"```bash"+`
make run-user
make run-note
make run-gateway
`+"```"+`

默认端口：

`+"```text"+`
gateway       http://localhost:8080
user-service  grpc://localhost:9001
note-service  grpc://localhost:9002
`+"```"+`

## 3. 调用接口

健康检查：

`+"```bash"+`
curl http://localhost:8080/healthz
`+"```"+`

注册用户：

`+"```bash"+`
curl -X POST http://localhost:8080/api/v1/users/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"ada@example.com","display_name":"Ada","password":"secret123"}'
`+"```"+`

创建笔记：

`+"```bash"+`
curl -X POST http://localhost:8080/api/v1/notes \
  -H 'Content-Type: application/json' \
  -d '{"author_id":"<user_id>","title":"DDD scaffold","content":"Gin plus gRPC demo"}'
`+"```"+`

## 4. 配置

主配置文件：

`+"```text"+`
configs/config.yaml
`+"```"+`

环境变量覆盖规则：

`+"```text"+`
APP_ + 配置路径大写 + 下划线
`+"```"+`

示例：

`+"```bash"+`
export APP_HTTP_PORT=8081
export APP_GRPC_USER_TARGET='127.0.0.1:9001'
export APP_GRPC_NOTE_TARGET='127.0.0.1:9002'
`+"```"+`

Windows PowerShell 使用：

`+"```powershell"+`
$env:APP_HTTP_PORT="8081"; make run-gateway
`+"```"+`

## 5. 公共工具

公共工具调用流程见：

`+"```text"+`
docs/toolkit.md
`+"```"+`
`, module)
}

func demoArchitectureDoc() string {
	return `# 架构说明

本项目是 bw-cli 演示工程，保留 user/note 两个示例服务。

## 总体调用链

~~~text
Client
  -> Gin Gateway
      -> UserService gRPC
          -> handler -> service -> model
                      -> repo -> Gorm
      -> NoteService gRPC
          -> handler -> service -> model
                      -> repo -> Gorm
~~~

## 服务分层

~~~text
internal/<service>/model    # 实体、值对象、业务错误、仓储接口
internal/<service>/dto/command.go      # 业务用例入参命令
internal/<service>/dto/<service>.go    # 业务用例出参 DTO 和转换
internal/<service>/service/service.go  # 业务流程编排
internal/<service>/repo     # Gorm、MongoDB、Redis、外部依赖实现
internal/<service>/handler  # gRPC/HTTP 入站适配
~~~

依赖方向：

~~~text
handler -> service -> model
repo -> model
~~~

service 目录中，command 放业务入参，dto 放业务出参和转换，service 放业务流程；handler 不直接堆字段或操作数据库。

## Gateway

~~~text
internal/gateway
  ├── client
  ├── request
  ├── handler
  └── router
~~~

路由按“版本 -> 业务 -> 具体接口”拆分：

~~~text
/api/v1/users/register
/api/v1/users/login
/api/v1/notes
/api/v1/notes/:id/publish
~~~
`
}

func generatedToolkitDoc(module string) string {
	return fmt.Sprintf(`# 工具组件总览与调用流程

当前项目由 bw-cli 生成，已移除脚手架命令源码。这里列出项目内可直接调用的公共工具包。

当前 module：

`+"```text"+`
%s
`+"```"+`

## 1. 工具列表

| 包 | 能力 |
| --- | --- |
| `+"`pkg/config`"+` | YAML 配置加载和环境变量覆盖 |
| `+"`pkg/logger`"+` | Zap 结构化日志和文件轮转 |
| `+"`pkg/errors`"+` | 统一业务错误码，HTTP/gRPC 状态映射 |
| `+"`pkg/httpx`"+` | Gin HTTP 统一响应 |
| `+"`pkg/middleware`"+` | CORS、JWT、RequestID、请求日志 |
| `+"`pkg/grpcx`"+` | gRPC request_id 透传和日志拦截器 |
| `+"`pkg/database`"+` | SQLite/MySQL/PostgreSQL Gorm 统一入口 |
| `+"`pkg/mysqlx`"+` | MySQL Gorm 初始化 |
| `+"`pkg/postgresx`"+` | PostgreSQL Gorm 初始化 |
| `+"`pkg/mongox`"+` | MongoDB 官方 driver 初始化和公共 DocumentStore CRUD 操作 |
| `+"`pkg/redisx`"+` | Redis client 初始化 |
| `+"`pkg/esx`"+` | Elasticsearch client 初始化 |
| `+"`pkg/kafkax`"+` | Kafka reader/writer 初始化 |
| `+"`pkg/filex`"+` | MinIO/OSS/Qiniu/COS 文件上传封装 |
| `+"`pkg/validator`"+` | 轻量参数校验 |

## 2. 推荐初始化顺序

`+"```text"+`
config.InitGlobal
  -> logger.New
  -> database.Open / mongox.NewClient / redisx.NewClient
  -> filex.NewUploader
  -> repo/service/handler
  -> Gin 或 gRPC server
`+"```"+`

## 3. 配置加载

`+"```go"+`
if err := config.InitGlobal("configs/config.yaml"); err != nil {
    panic(err)
}
cfg := config.MustGlobal()
`+"```"+`

环境变量覆盖规则：

`+"```text"+`
APP_ + 配置路径大写 + 下划线
`+"```"+`

## 4. 日志

`+"```go"+`
logCfg := logger.WithDailyFileName(cfg.Log, time.Now())
log, err := logger.New(logCfg)
if err != nil {
    panic(err)
}
defer log.Sync()
`+"```"+`

默认日志保留 7 天，文件名按服务名和日期生成。

## 5. Gorm 数据库

`+"```go"+`
db, err := database.Open(cfg.Database, cfg.MySQL, cfg.PostgreSQL, log)
if err != nil {
    log.Fatal("open database failed", zap.Error(err))
}
`+"```"+`

支持：

`+"```text"+`
sqlite
mysql
postgres
postgresql
pg
`+"```"+`

## 6. MongoDB

`+"```go"+`
type Document struct {
    ID string `+"`bson:\"_id\"`"+`
}

func (Document) MongoCollectionName() string {
    return "documents"
}

client, err := mongox.NewClient(cfg.MongoDB.MongoxConfig())
if err != nil {
    panic(err)
}
defer client.Disconnect(context.Background())

db := mongox.Database(client, cfg.MongoDB.Database)
documents := mongox.NewDocumentStore[Document](db)
_, err = documents.UpsertByID(context.Background(), "doc-1", &Document{ID: "doc-1"})
if err != nil {
    panic(err)
}
`+"```"+`

详细教程见 `+"`docs/mongodb.md`"+`。

## 7. 文件上传

`+"```go"+`
uploader, err := filex.NewUploader(cfg.FileStorage)
if err != nil {
    panic(err)
}

result, err := uploader.Upload(ctx, filex.UploadRequest{
    Reader:      file,
    Filename:    header.Filename,
    ContentType: header.Header.Get("Content-Type"),
    Size:        header.Size,
})
`+"```"+`

支持 provider：

`+"```text"+`
minio
oss
qiniu
cos
`+"```"+`
`, module)
}

func generatedMongoDBDoc(module string) string {
	return fmt.Sprintf(`# MongoDB 从 0 到 1 使用教程

当前项目 module：

`+"```text"+`
%s
`+"```"+`

## 1. MongoDB 是什么

MongoDB 是文档数据库，保存的是 BSON 文档。关系型数据库常见结构是 database/table/row/column，MongoDB 对应 database/collection/document/field。

适合使用 MongoDB 的场景：

- 内容草稿、富文本 JSON、扩展字段。
- 用户偏好、第三方回调原始数据。
- 操作日志、行为事件、审计记录。
- 结构变化快、字段差异大的业务文档。

不建议用 MongoDB 承担强事务账务、复杂 join 报表或必须依赖外键约束的核心关系模型。

## 2. 准备连接信息

脚手架不在文档中假设固定的 MongoDB 启动方式。你可以使用公司测试环境、本机已安装的 MongoDB 或云数据库，只需要拿到可连接的地址、账号、密码和 database 名称，然后写入 `+"`configs/config.yaml`"+`。

示例连接信息：

`+"```text"+`
uri: mongodb://127.0.0.1:27017
username: app_user
password: app_password
database: app
`+"```"+`

## 3. mongosh 入门

如果本机已经安装 `+"`mongosh`"+`，可以用配置文件中的连接信息进入数据库验证连通性：

`+"```bash"+`
mongosh 'mongodb://app_user:app_password@127.0.0.1:27017/app?authSource=app'
`+"```"+`

基础命令：

`+"```javascript"+`
db.runCommand({ ping: 1 })
db.getName()
show collections
db.documents.insertOne({ title: "hello", created_at: new Date() })
db.documents.find()
`+"```"+`

如果你的数据库没有启用账号密码，则连接串可以不带用户名和密码：

`+"```text"+`
mongodb://127.0.0.1:27017/app
`+"```"+`

## 4. 项目配置

`+"```yaml"+`
mongodb:
  uri: mongodb://127.0.0.1:27017
  username: ""
  password: ""
  database: app
  app_name: app-service
  min_pool_size: 0
  max_pool_size: 100
  connect_timeout_seconds: 10
  server_selection_timeout_seconds: 5
`+"```"+`

服务启动时只读取 `+"`configs/config.yaml`"+` 中的 `+"`mongodb.*`"+` 配置。需要账号密码时，填写 `+"`username`"+`、`+"`password`"+`；需要连接副本集或指定认证库时，把完整连接串写入 `+"`uri`"+`。

## 5. Go 初始化

`+"```go"+`
type Document struct {
    ID string `+"`bson:\"_id\"`"+`
}

func (Document) MongoCollectionName() string {
    return "documents"
}

client, err := mongox.NewClient(cfg.MongoDB.MongoxConfig())
if err != nil {
    return err
}
defer client.Disconnect(context.Background())

if err := mongox.Ping(context.Background(), client); err != nil {
    return err
}

db := mongox.Database(client, cfg.MongoDB.Database)
documents := mongox.NewDocumentStore[Document](db)
`+"```"+`

## 6. Repo 层建议

建议把 MongoDB 代码放在 `+"`internal/<service>/repo`"+`，不要在 handler 或 service 里直接操作集合。repo 层通过 `+"`mongox.NewDocumentStore[T]`"+` 复用公共 CRUD 操作，集合名称由文档结构体的 `+"`MongoCollectionName()`"+` 统一声明：

`+"```text"+`
handler -> service -> model.Repository
repo -> MongoDB collection
`+"```"+`

`+"```go"+`
_, err := documents.UpsertByID(ctx, "doc-1", &Document{ID: "doc-1"})
if err != nil {
    return err
}

document, err := documents.FindByID(ctx, "doc-1")
`+"```"+`

示例结构：

`+"```text"+`
internal/content/model/document.go
internal/content/model/repository.go
internal/content/repo/mongo_repository.go
`+"```"+`

## 7. 索引

在 repo 初始化时创建索引：

`+"```go"+`
_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
    Keys: bson.D{
        {Key: "owner_id", Value: 1},
        {Key: "created_at", Value: -1},
    },
})
`+"```"+`

常见索引策略：

- 等值查询字段放前面，例如 `+"`owner_id`"+`。
- 排序字段放后面，例如 `+"`created_at`"+`。
- 唯一业务键使用 unique index。
- 大集合分页优先使用游标条件，不要深页 skip。

## 8. 测试建议

轻量单元测试可以测 repo 的参数转换和错误处理；集成测试再连接真实 MongoDB。CI 中建议使用独立测试库或测试环境 MongoDB，连接信息统一写入测试配置文件。
`, module)
}

func copyFile(source string, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func readModule(goModPath string) (string, error) {
	file, err := os.Open(goModPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", errors.New("module directive not found")
}

func rewriteModule(root string, oldModule string, newModule string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !shouldRewrite(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		updated := strings.ReplaceAll(string(data), oldModule, newModule)
		return os.WriteFile(path, []byte(updated), 0o644)
	})
}

func shouldRewrite(path string) bool {
	if strings.HasSuffix(path, ".pb.go") {
		return false
	}
	switch filepath.Ext(path) {
	case ".go", ".mod", ".md", ".yaml", ".yml", ".proto":
		return true
	default:
		return false
	}
}
