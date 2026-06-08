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

// InitOptions 控制 bw-cli 如何基于当前脚手架生成新项目。
type InitOptions struct {
	SourceDir   string
	TargetDir   string
	ModulePath  string
	RepoURL     string
	Branch      string
	RunTidy     bool
	IncludeDemo bool
}

// Init 复制或克隆脚手架，并为目标项目重写 module 路径。
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
	// 生成项目不应继承脚手架仓库的远程地址。
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
		filepath.Join("internal", "gateway", "router", "router_test.go"),
	} {
		if err := os.RemoveAll(filepath.Join(root, rel)); err != nil {
			return err
		}
	}
	if err := removeGeneratedGatewayRoutes(filepath.Join(root, "internal", "gateway", "router")); err != nil {
		return err
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

func removeGeneratedGatewayRoutes(routerDir string) error {
	entries, err := os.ReadDir(routerDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_routes.go") {
			continue
		}
		if err := os.RemoveAll(filepath.Join(routerDir, entry.Name())); err != nil {
			return err
		}
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
	if err := os.WriteFile(path, []byte(cleanConfigYAML()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "configs", "nacos.yaml"), []byte(cleanNacosYAML()), 0o644)
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
	if err := os.WriteFile(filepath.Join(docsDir, "mongodb.md"), []byte(generatedMongoDBDoc(module)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(docsDir, "alipay.md"), []byte(generatedAlipayDoc()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(docsDir, "elasticsearch.md"), []byte(generatedElasticsearchDoc()), 0o644)
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
	if err := os.WriteFile(filepath.Join(docsDir, "mongodb.md"), []byte(generatedMongoDBDoc(module)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(docsDir, "alipay.md"), []byte(generatedAlipayDoc()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(docsDir, "elasticsearch.md"), []byte(generatedElasticsearchDoc()), 0o644)
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
	// 构建依赖前先加载运行时配置。
	if err := config.InitGlobal("configs/config.yaml"); err != nil {
		panic(err)
	}
	cfg := config.MustGlobal()
	gatewayServiceName := cfg.ServiceName("gateway")
	cfg.Log.Service = gatewayServiceName
	cfg.Log = logger.WithDailyFileName(cfg.Log, time.Now())

	log, err := logger.New(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer log.Sync()
	observability.Register(gatewayServiceName, log)
	config.PrintSourceNotice(cfg, os.Stdout)

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
	fmt.Fprintf(os.Stdout, "  service: %%s\n", cfg.ServiceName("gateway"))
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
	return fmt.Sprintf(`// router 包负责 Gin 引擎构建和路由注册。
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"%s/pkg/config"
	"%s/pkg/middleware"
)

// New 使用配置好的中间件和版本化 API 路由构建网关 Gin 引擎。
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

// registerAPIRoutes 创建 /api/v1 路由命名空间。
// 新增服务时，在该文件旁边增加对应业务路由文件。
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

http:
  host: 0.0.0.0
  port: 8080
  read_timeout_seconds: 5
  write_timeout_seconds: 10

grpc:
  host: 0.0.0.0

services:
  gateway:
    name: gateway

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
  dial_timeout: 5s
  read_timeout: 3s
  write_timeout: 3s
  lock:
    key_prefix: app
    default_ttl: 30s

elasticsearch:
  addresses:
    - http://127.0.0.1:9200
  username: ""
  password: ""
  cloud_id: ""
  api_key: ""

kafka:
  brokers:
    - 127.0.0.1:9092
  topic: app-events
  group_id: app-consumer
  client_id: app
  required_acks: all
  dial_timeout: 5s
  producer:
    max_attempts: 10
    batch_size: 100
    batch_bytes: 1048576
    batch_timeout: 10ms
    read_timeout: 10s
    write_timeout: 10s
    async: false
    compression: none
    allow_auto_topic_creation: false
  consumer:
    queue_capacity: 100
    min_bytes: 1
    max_bytes: 10485760
    max_wait: 10s
    read_batch_timeout: 10s
    commit_interval: 0s
    heartbeat_interval: 3s
    session_timeout: 30s
    rebalance_timeout: 30s
    start_offset: first
    watch_partition_changes: true
    max_attempts: 3
  sasl:
    enable: false
    mechanism: plain
    username: ""
    password: ""
  tls:
    enable: false
    insecure_skip_verify: false
    server_name: ""

alipay:
  app_id: ""
  private_key: ""
  alipay_public_key: ""
  production: false
  notify_url: ""
  return_url: ""
  encrypt_key: ""
  app_cert_public_key_path: ""
  alipay_root_cert_path: ""
  alipay_cert_public_key_path: ""

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

func cleanNacosYAML() string {
	return `enabled: false
server_addr: 127.0.0.1
server_port: 8848
namespace_id: ""
group: DEFAULT_GROUP
data_id: xiaolanshu.yaml
username: ""
password: ""
timeout_ms: 5000
log_dir: logs/nacos
cache_dir: data/nacos/cache
log_level: info
fail_fast: false
watch: false
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

命令会生成 `+"`cmd/order`"+`、`+"`api/proto/order`"+`、`+"`internal/order/entity`"+`、`+"`internal/order/model`"+`、`+"`internal/order/dto`"+`、`+"`internal/order/service/service.go`"+`、`+"`repo/gorm_repository.go`"+`、`+"`repo/mongo_repository.go`"+`、`+"`handler`"+`、gateway request/handler/router 和 `+"`docs/services/order.md`"+`。生成后的服务默认带 Create/Get/List/Update/Delete 基础 CRUD，并会把服务名、gRPC 端口和 gateway target 写入 `+"`configs/config.yaml`"+`。默认启动使用 Gorm，MongoDB 仓储已通过 `+"`mongox.NewDocumentStore[T]`"+` 预先接好。

需要指定端口时使用：

`+"```bash"+`
bw-cli service order --port 9103 --tidy
`+"```"+`

启动后控制台会输出服务名、环境、监听地址、gRPC 地址和配置项。

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

需要调整端口、日志级别或数据库类型时，修改 `+"`configs/config.yaml`"+`。

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

Alipay、Elasticsearch、Kafka 和 JWT 的接入示例见：

`+"```text"+`
docs/alipay.md
docs/elasticsearch.md
docs/toolkit.md
`+"```"+`

## 6. 新增业务服务

进入项目根目录执行：

`+"```bash"+`
bw-cli service comment --tidy
`+"```"+`

不传 `+"`--port`"+` 时会根据 `+"`configs/config.yaml`"+` 中已有服务端口自动递增。需要指定端口时使用：

`+"```bash"+`
bw-cli service comment --port 9103 --tidy
`+"```"+`

如果要绑定已有数据库表，可以使用 `+"`--table`"+`：

`+"```bash"+`
bw-cli service comment --table comments --tidy
`+"```"+`

当前 `+"`--table`"+` 会校验表存在，并要求字段包含 `+"`id`"+`、`+"`name`"+`、`+"`description`"+`、`+"`created_at`"+`、`+"`updated_at`"+`，其中 `+"`id`"+` 必须是主键。生成后的服务会跳过 `+"`AutoMigrate`"+`，避免修改既有表结构。MySQL 或 PostgreSQL 需要指定 schema 时，可以传 `+"`--schema`"+`。

生成后的服务会自动追加 `+"`configs/config.yaml`"+`：

- 服务名、gRPC 端口和 gateway target 写在 `+"`services.comment`"+`。
- 数据库继续读取当前项目已有的 `+"`database`"+`、`+"`mysql`"+`、`+"`postgresql`"+` 配置。
- 默认 SQLite 可直接本地运行，服务启动时自动执行 `+"`AutoMigrate`"+`。
- proto、handler、entity、model、service、repo、gateway HTTP 入口和 service 单测都会同时生成。
- 如果启用了 Nacos，命令行会提示把本地新增配置同步到 Nacos。

生成结构：

`+"```text"+`
api/proto/comment/v1/comment.proto
api/gen/comment/v1
cmd/comment/main.go
internal/comment/entity     # 业务实体、业务错误、Repository 接口
internal/comment/model      # 数据库表结构和文档结构
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

删除脚手架生成的服务：

`+"```bash"+`
bw-cli delete-service comment --tidy
`+"```"+`

删除命令会清理服务目录、proto/gen、gateway request/handler/router、`+"`docs/services/comment.md`"+`、`+"`Makefile`"+` 目标和 `+"`configs/config.yaml`"+` 中的服务配置。如果启用了 Nacos，命令行会提示把本地配置删除结果同步到 Nacos。

生成后的基础 CRUD 调用链：

`+"```text"+`
gRPC client -> proto -> handler -> service -> entity.Repository -> repo(Gorm) -> database
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

- `+"`entity`"+` 写业务核心，不依赖 Gin、gRPC、Gorm，也不写数据库 tag。
- `+"`model`"+` 只写数据库表结构、文档结构、`+"`TableName()`"+` 和 `+"`MongoCollectionName()`"+`，不写查询逻辑。
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
internal/<service>/entity   # 业务实体、业务错误、仓储接口
internal/<service>/model    # 数据库表结构和文档结构
internal/<service>/dto/command.go      # 业务用例入参命令
internal/<service>/dto/<service>.go    # 业务用例出参 DTO 和转换
internal/<service>/service/service.go  # 业务流程编排
internal/<service>/repo     # Gorm、MongoDB、Redis、外部依赖实现
internal/<service>/handler  # gRPC/HTTP 入站适配
~~~

依赖方向：

~~~text
handler -> service -> entity
repo -> entity
repo -> model
~~~

entity 不依赖 Gin、gRPC、Gorm 或云 SDK，便于测试和替换基础设施。model 只维护数据库结构，不写查询逻辑。service 继续拆为 command、dto、service 三个文件：command 放业务入参，dto 放业务出参和转换，service 放业务流程。

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
  -d '{"email":"ada@example.com","display_name":"Ada","password":"<password>"}'
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
  -d '{"email":"ada@example.com","display_name":"Ada","password":"<password>"}'
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

需要调整 HTTP 端口或 gRPC target 时，修改 `+"`configs/config.yaml`"+` 中的 `+"`http`"+` 和 `+"`services`"+` 配置。

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
          -> handler -> service -> entity
                      -> repo -> model -> Gorm
      -> NoteService gRPC
          -> handler -> service -> entity
                      -> repo -> model -> Gorm
~~~

## 服务分层

~~~text
internal/<service>/entity   # 业务实体、业务错误、仓储接口
internal/<service>/model    # 数据库表结构和文档结构
internal/<service>/dto/command.go      # 业务用例入参命令
internal/<service>/dto/<service>.go    # 业务用例出参 DTO 和转换
internal/<service>/service/service.go  # 业务流程编排
internal/<service>/repo     # Gorm、MongoDB、Redis、外部依赖实现
internal/<service>/handler  # gRPC/HTTP 入站适配
~~~

依赖方向：

~~~text
handler -> service -> entity
repo -> entity
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
| `+"`pkg/config`"+` | YAML 配置加载和默认值 |
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
| `+"`pkg/esx`"+` | Elasticsearch v7 client、模糊搜索、高亮和聚合封装 |
| `+"`pkg/kafkax`"+` | Kafka producer/consumer 和原生 reader/writer 初始化 |
| `+"`pkg/filex`"+` | MinIO/OSS/Qiniu/COS 文件上传封装 |
| `+"`pkg/alipayx`"+` | 支付宝支付、回调验签和退款封装 |
| `+"`pkg/timex`"+` | 周岁计算、中文相对时间和日期时间格式化 |
| `+"`pkg/validator`"+` | 轻量参数校验 |

## 2. 推荐初始化顺序

`+"```text"+`
config.InitGlobal
  -> logger.New
  -> database.Open / mongox.NewClient / redisx.NewClient
  -> esx.NewClient / kafkax.NewProducer / alipayx.NewClient
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

## 5. 时间处理

`+"```go"+`
age := timex.Age(user.Birthday)
display := timex.RelativeTime(note.CreatedAt)
`+"```"+`

`+"`RelativeTime`"+` 会按时间差返回 `+"`N秒前`"+`、`+"`N分钟前`"+`、`+"`N小时前`"+`、`+"`N天前`"+`、`+"`N月前`"+`，超过一年返回 `+"`2006-01-02 15:04:05`"+` 格式的具体日期时间。需要固定当前时间时使用 `+"`AgeAt`"+` 或 `+"`RelativeTimeAt`"+`。

## 6. HTTP 中间件和 JWT

`+"```go"+`
r.Use(middleware.RequestID())
r.Use(middleware.RequestLogger(log))
r.Use(middleware.CORS(cfg.Middleware.CORS))

auth := r.Group("/api/v1")
auth.Use(middleware.JWTAuth(cfg.Middleware.JWT))
`+"```"+`

签发 token：

`+"```go"+`
token, err := middleware.GenerateToken(cfg.Middleware.JWT, middleware.JWTClaims{
    UserID: userID,
    Role:   "user",
})
if err != nil {
    return err
}
`+"```"+`

`+"`middleware.jwt.secret`"+` 必须在 `+"`configs/config.yaml`"+` 中设置。

## 6. Gorm 数据库

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

## 7. MongoDB

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

## 8. Elasticsearch

本地或无认证集群只需要配置 `+"`elasticsearch.addresses`"+`。其它字段保留为云服务或认证集群使用。

`+"```go"+`
client, err := esx.NewClient(cfg.Elasticsearch)
if err != nil {
    return err
}
searcher := esx.NewSearcherFromClient(client)

result, err := searcher.FuzzySearch(ctx, esx.FuzzySearchRequest{
    Index:   "documents",
    Keyword: "golang",
    Fields:  []string{"title^2", "content"},
    Highlight: esx.HighlightConfig{
        Fields:   []string{"title", "content"},
        PreTags:  []string{"<mark>"},
        PostTags: []string{"</mark>"},
    },
})
if err != nil {
    return err
}
_ = result
`+"```"+`

聚合查询：

`+"```go"+`
result, err := searcher.Aggregate(ctx, esx.AggregationRequest{
    Index: "documents",
    Aggregations: map[string]esx.Aggregation{
        "by_author": esx.TermsAggregation("author_id", 10),
        "by_day":    esx.DateHistogramAggregation("created_at", "day"),
    },
})
if err != nil {
    return err
}
fmt.Println(string(result.Aggregations))
`+"```"+`

MySQL 同步 ES 的增量扫描、bulk 写入和删除同步示例见 `+"`docs/elasticsearch.md`"+`。

## 9. Kafka

进程入口初始化一次 producer，然后注入业务 service：

`+"```go"+`
producer, err := kafkax.NewProducer(cfg.Kafka)
if err != nil {
    return err
}
defer producer.Close()

noteSvc := noteservice.NewService(repo, producer, log)
`+"```"+`

业务 service 直接发布事件。`+"`kafkax.NewProducer(cfg.Kafka)`"+` 创建的 writer 已经绑定 `+"`cfg.Kafka.Topic`"+`，不要再给 `+"`kafkax.Message.Topic`"+` 赋值。

`+"```go"+`
func (s *Service) PublishNoteCreated(ctx context.Context, noteID string, authorID string) error {
    payload, err := json.Marshal(map[string]string{
        "event":     "note.created",
        "note_id":   noteID,
        "author_id": authorID,
    })
    if err != nil {
        return err
    }

    return s.kafka.Publish(ctx, kafkax.Message{
        Key:   noteID,
        Value: payload,
        Headers: map[string]string{
            "event": "note.created",
        },
    })
}
`+"```"+`

如果出现 `+"`[3] Unknown Topic Or Partition`"+`，说明 broker 上没有配置里的 topic。生产环境建议提前创建 topic：

`+"```bash"+`
kafka-topics.sh --bootstrap-server 127.0.0.1:9092 \
  --create \
  --topic xiaolanshu-events \
  --partitions 3 \
  --replication-factor 1
`+"```"+`

开发环境也可以临时打开：

`+"```yaml"+`
kafka:
  producer:
    allow_auto_topic_creation: true
`+"```"+`

消费者：

`+"```go"+`
consumer, err := kafkax.NewConsumer(cfg.Kafka)
if err != nil {
    return err
}
defer consumer.Close()

return consumer.Run(ctx, func(ctx context.Context, msg kafka.Message) error {
    return handleEvent(ctx, msg.Key, msg.Value)
})
`+"```"+`

如果业务需要直接使用 `+"`segmentio/kafka-go`"+` 的完整能力，也可以使用 `+"`kafkax.NewWriter`"+` 和 `+"`kafkax.NewReader`"+`。

## 10. 支付宝支付

`+"```go"+`
payClient, err := alipayx.NewClient(cfg.Alipay)
if err != nil {
    return err
}

payURL, err := payClient.PagePay(alipayx.PayRequest{
    OutTradeNo:  orderID,
    Subject:     "订单支付",
    TotalAmount: "9.90",
})
if err != nil {
    return err
}
_ = payURL.String()
`+"```"+`

同步回调使用 `+"`VerifyReturn`"+` 验签，异步通知使用 `+"`DecodeNotification`"+` 解析并验签，退款使用 `+"`Refund`"+`。Gin handler 和 service 接入示例见 `+"`docs/alipay.md`"+`。

## 11. 文件上传

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

脚手架不在文档中假设固定的 MongoDB 启动方式。你可以使用公司测试环境、本机已安装的 MongoDB 或云数据库，只需要把真实连接地址、认证信息和 database 名称写入 `+"`configs/config.yaml`"+`。

本地无认证示例：

`+"```text"+`
uri: mongodb://127.0.0.1:27017
username: ""
password: ""
database: app
`+"```"+`

## 3. mongosh 入门

如果本机已经安装 `+"`mongosh`"+`，可以用配置文件中的连接信息进入数据库验证连通性：

`+"```bash"+`
mongosh 'mongodb://127.0.0.1:27017/app'
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

建议把 MongoDB 查询代码放在 `+"`internal/<service>/repo`"+`，不要在 handler 或 service 里直接操作集合。MongoDB 文档结构放在 `+"`internal/<service>/model`"+`，写 `+"`bson`"+` tag 和 `+"`MongoCollectionName()`"+`；repo 层通过 `+"`mongox.NewDocumentStore[T]`"+` 复用公共 CRUD 操作，并负责 entity/model 转换：

`+"```text"+`
handler -> service -> entity.Repository
repo -> model -> MongoDB collection
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
internal/content/entity/content.go
internal/content/entity/repository.go
internal/content/model/content.go
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

func generatedAlipayDoc() string {
	return `# 支付宝支付调用示例

本文档说明如何使用 ` + "`pkg/alipayx`" + ` 接入支付宝支付、同步回调验签、异步通知和退款。

## 配置

` + "```yaml" + `
alipay:
  app_id: ""
  private_key: ""
  alipay_public_key: ""
  production: false
  notify_url: "https://api.example.com/payments/alipay/notify"
  return_url: "https://www.example.com/orders/alipay/return"
  encrypt_key: ""
  app_cert_public_key_path: ""
  alipay_root_cert_path: ""
  alipay_cert_public_key_path: ""
` + "```" + `

普通公钥模式和证书模式二选一。普通公钥模式配置 ` + "`alipay_public_key`" + `；证书模式配置三个证书路径。

## 初始化

` + "```go" + `
if err := config.InitGlobal("configs/config.yaml"); err != nil {
    panic(err)
}
cfg := config.MustGlobal()

payClient, err := alipayx.NewClient(cfg.Alipay)
if err != nil {
    panic(err)
}

orderSvc := orderservice.NewService(repo, payClient, log)
` + "```" + `

推荐在入口初始化一次，然后注入业务 service：

` + "```text" + `
handler -> service -> alipayx.Client
` + "```" + `

## 创建支付

实际业务 service 中直接调用 ` + "`pkg/alipayx`" + `，不要在业务项目里再复制一层支付宝工具类。

` + "```go" + `
func (s *Service) CreateAlipayPagePayment(ctx context.Context, orderID string) (string, error) {
    order, err := s.repo.FindByID(ctx, orderID)
    if err != nil {
        return "", err
    }
    if order.PaidAt != nil {
        return "", apperrors.FailedPrecondition("ORDER_ALREADY_PAID", "order already paid")
    }

    return s.alipay.PagePayURL(alipayx.PayRequest{
        OutTradeNo:     order.PayNo,
        Subject:        "订单支付",
        TotalAmount:    order.PayAmount.StringFixed(2),
        TimeoutExpress: "15m",
    })
}
` + "```" + `

Gin handler 只处理协议入参和出参：

` + "```go" + `
func CreateAlipayPagePayment(svc *orderservice.Service) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req struct {
            OrderID string ` + "`json:\"order_id\" binding:\"required\"`" + `
        }
        if err := c.ShouldBindJSON(&req); err != nil {
            httpx.Error(c, apperrors.InvalidArgument("INVALID_REQUEST", err.Error()))
            return
        }

        payURL, err := svc.CreateAlipayPagePayment(c.Request.Context(), req.OrderID)
        if err != nil {
            httpx.Error(c, err)
            return
        }
        httpx.OK(c, gin.H{"pay_url": payURL})
    }
}
` + "```" + `

## 同步回调验签

` + "```go" + `
func AlipayReturn(payClient *alipayx.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        if err := c.Request.ParseForm(); err != nil {
            httpx.Error(c, apperrors.InvalidArgument("INVALID_REQUEST", err.Error()))
            return
        }
        if err := payClient.VerifyReturn(c.Request.Context(), c.Request.Form); err != nil {
            httpx.Error(c, apperrors.InvalidArgument("ALIPAY_SIGN_INVALID", err.Error()))
            return
        }

        httpx.OK(c, gin.H{
            "out_trade_no": c.Request.Form.Get("out_trade_no"),
            "trade_no":     c.Request.Form.Get("trade_no"),
        })
    }
}
` + "```" + `

## 异步通知回调

支付宝异步通知必须验签、校验订单号和金额、幂等更新订单状态。处理成功后返回纯文本 ` + "`success`" + `。

` + "```go" + `
func AlipayNotify(payClient *alipayx.Client, svc *orderservice.Service) gin.HandlerFunc {
    return func(c *gin.Context) {
        if err := c.Request.ParseForm(); err != nil {
            c.String(http.StatusBadRequest, "fail")
            return
        }

        notification, err := payClient.DecodeNotification(c.Request.Context(), c.Request.Form)
        if err != nil {
            c.String(http.StatusBadRequest, "fail")
            return
        }

        err = svc.MarkAlipayPaid(
            c.Request.Context(),
            notification.OutTradeNo,
            notification.TradeNo,
            notification.TotalAmount,
            notification.TradeStatus,
        )
        if err != nil {
            c.String(http.StatusInternalServerError, "fail")
            return
        }

        c.String(http.StatusOK, "success")
    }
}
` + "```" + `

## 退款

` + "```go" + `
func (s *Service) RefundAlipayOrder(ctx context.Context, orderID string, reason string) error {
    order, err := s.repo.FindByID(ctx, orderID)
    if err != nil {
        return err
    }
    if order.RefundedAt != nil {
        return nil
    }

    return s.alipay.RefundOK(ctx, alipayx.RefundRequest{
        OutTradeNo:   order.PayNo,
        RefundAmount: order.PayAmount.StringFixed(2),
        RefundReason: reason,
        OutRequestNo: order.PayNo + "-refund-001",
    })
}
` + "```" + `
`
}

func generatedElasticsearchDoc() string {
	return `# Elasticsearch 调用示例

本文档说明如何使用 ` + "`pkg/esx`" + ` 创建 Elasticsearch v7 client，并调用模糊搜索、高亮和聚合查询。本地或无认证集群只需要配置 ` + "`addresses`" + `，其它认证参数保留为可选项。

## 配置

` + "```yaml" + `
elasticsearch:
  addresses:
    - http://127.0.0.1:9200
  username: ""
  password: ""
  cloud_id: ""
  api_key: ""
` + "```" + `

最小配置只需要：

` + "```yaml" + `
elasticsearch:
  addresses:
    - http://127.0.0.1:9200
` + "```" + `

## 初始化

` + "```go" + `
if err := config.InitGlobal("configs/config.yaml"); err != nil {
    panic(err)
}
cfg := config.MustGlobal()

client, err := esx.NewClient(cfg.Elasticsearch)
if err != nil {
    panic(err)
}
searcher := esx.NewSearcherFromClient(client)
` + "```" + `

## 模糊搜索和高亮

` + "```go" + `
result, err := searcher.FuzzySearch(ctx, esx.FuzzySearchRequest{
    Index:   "documents",
    Keyword: "golang",
    Fields:  []string{"title^2", "content"},
    Filters: []esx.Filter{
        esx.TermFilter("status", "published"),
    },
    Highlight: esx.HighlightConfig{
        Fields:   []string{"title", "content"},
        PreTags:  []string{"<mark>"},
        PostTags: []string{"</mark>"},
    },
})
if err != nil {
    return err
}
_ = result
` + "```" + `

## 聚合查询

` + "```go" + `
result, err := searcher.Aggregate(ctx, esx.AggregationRequest{
    Index: "documents",
    Aggregations: map[string]esx.Aggregation{
        "by_author": esx.TermsAggregation("author_id", 10),
        "by_day":    esx.DateHistogramAggregation("created_at", "day"),
    },
})
if err != nil {
    return err
}
fmt.Println(string(result.Aggregations))
` + "```" + `

## MySQL 同步到 ES

常见同步方式有两种：

1. 事件同步：MySQL 写入成功后发布 Kafka 事件，由消费者写 ES。
2. 增量轮询：定时按 ` + "`updated_at`" + ` 和 ` + "`id`" + ` 游标扫描 MySQL，批量写 ES。

Bulk 写入示例：

` + "```go" + `
func SyncDocumentsToES(ctx context.Context, client *elasticsearch.Client, docs []Document) error {
    var buf bytes.Buffer
    encoder := json.NewEncoder(&buf)

    for _, doc := range docs {
        if err := encoder.Encode(map[string]any{
            "index": map[string]any{"_index": "documents", "_id": doc.ID},
        }); err != nil {
            return err
        }
        if err := encoder.Encode(doc); err != nil {
            return err
        }
    }

    res, err := client.Bulk(bytes.NewReader(buf.Bytes()), client.Bulk.WithContext(ctx))
    if err != nil {
        return err
    }
    defer res.Body.Close()
    if res.IsError() {
        data, _ := io.ReadAll(res.Body)
        return fmt.Errorf("bulk index documents failed: %s", data)
    }
    return nil
}
` + "```" + `
`
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
