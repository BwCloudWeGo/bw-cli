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
	if !opts.IncludeDemo {
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
	for _, rel := range []string{
		filepath.Join("api", "proto", "user"),
		filepath.Join("api", "proto", "note"),
		filepath.Join("api", "gen", "user"),
		filepath.Join("api", "gen", "note"),
		filepath.Join("cmd", "user"),
		filepath.Join("cmd", "note"),
		filepath.Join("internal", "user"),
		filepath.Join("internal", "note"),
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
	return os.WriteFile(filepath.Join(docsDir, "architecture.md"), []byte(cleanArchitectureDoc()), 0o644)
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
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		panic(err)
	}
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

	go func() {
		log.Info("gateway listening", zap.String("addr", addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("gateway stopped unexpectedly", zap.Error(err))
		}
	}()

	waitForShutdown(server, log)
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
PROTO_PATH := api/proto
PROTO_OUT := api/gen
PROTO_PLUGIN_PATH := $(shell go env GOPATH)/bin
PROTO_FILES := $(shell if [ -d "$(PROTO_PATH)" ]; then cd $(PROTO_PATH) && find . -name '*.proto' | sed 's,^\./,,'; fi)

.PHONY: proto test tidy run-gateway install-tools

tools:
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

proto:
	@if [ -z "$(PROTO_FILES)" ]; then \
		echo "No proto files found"; \
	else \
		PATH="$(PROTO_PLUGIN_PATH):$$PATH" $(PROTOC) \
			--proto_path=$(PROTO_PATH) \
			--go_out=$(PROTO_OUT) --go_opt=paths=source_relative \
			--go-grpc_out=$(PROTO_OUT) --go-grpc_opt=paths=source_relative \
			$(PROTO_FILES); \
	fi

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

启动 HTTP gateway：

`+"```bash"+`
make run-gateway
`+"```"+`

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

新增业务服务时建议使用：

`+"```text"+`
internal/<service>/model
internal/<service>/service
internal/<service>/repo
internal/<service>/handler
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

## 2. 启动 gateway

`+"```bash"+`
make run-gateway
`+"```"+`

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
`, module)
}

func cleanArchitectureDoc() string {
	return `# 架构说明

本项目是干净微服务框架，默认只保留 HTTP gateway、配置、日志、中间件、数据库和外部组件封装，不包含业务 demo。

## 分层建议

新增服务时按以下包名组织：

~~~text
internal/<service>/model    # 实体、值对象、业务错误、仓储接口
internal/<service>/service  # 业务用例编排
internal/<service>/repo     # Gorm、MongoDB、Redis、外部依赖实现
internal/<service>/handler  # gRPC/HTTP 入站适配
~~~

依赖方向：

~~~text
handler -> service -> model
repo -> model
~~~

model 不依赖 Gin、gRPC、Gorm 或云 SDK，便于测试和替换基础设施。

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
