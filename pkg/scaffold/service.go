package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"unicode"
)

const defaultServicePort = 9100

var serviceNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

// ServiceOptions controls bw-cli service generation inside an existing project.
type ServiceOptions struct {
	RootDir  string
	Name     string
	Port     int
	RunProto bool
	RunTidy  bool
}

type serviceTemplateData struct {
	Module       string
	InputName    string
	Dir          string
	ProtoFile    string
	ProtoPackage string
	GoPackage    string
	Pascal       string
	ServiceName  string
	EnvPrefix    string
	Port         int
	TableName    string
}

// AddService creates a complete gRPC service skeleton in an existing bw-cli project.
func AddService(opts ServiceOptions) error {
	root, err := serviceRoot(opts.RootDir)
	if err != nil {
		return err
	}
	module, err := readModule(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	data, err := buildServiceTemplateData(module, opts.Name, opts.Port)
	if err != nil {
		return err
	}
	if err := ensureServiceDoesNotExist(root, data); err != nil {
		return err
	}
	if err := writeServiceFiles(root, data); err != nil {
		return err
	}
	if err := addServiceMakeTarget(root, data.Dir); err != nil {
		return err
	}
	if opts.RunProto {
		if err := runProjectCommand(root, "go", "run", "./tools/protogen"); err != nil {
			return fmt.Errorf("generate proto for %s: %w", data.Dir, err)
		}
	}
	if err := gofmtService(root, data.Dir); err != nil {
		return err
	}
	if opts.RunTidy {
		if err := runProjectCommand(root, "go", "mod", "tidy"); err != nil {
			return fmt.Errorf("go mod tidy: %w", err)
		}
	}
	return nil
}

func serviceRoot(root string) (string, error) {
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(abs, "go.mod")); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("go.mod not found in %s", abs)
		}
		return "", err
	}
	return abs, nil
}

func buildServiceTemplateData(module string, rawName string, port int) (serviceTemplateData, error) {
	parts, err := splitServiceName(rawName)
	if err != nil {
		return serviceTemplateData{}, err
	}
	if port == 0 {
		port = defaultServicePort
	}
	if port < 0 || port > 65535 {
		return serviceTemplateData{}, fmt.Errorf("port must be between 1 and 65535")
	}
	dir := strings.Join(parts, "_")
	pascal := toPascal(parts)
	return serviceTemplateData{
		Module:       module,
		InputName:    strings.TrimSpace(rawName),
		Dir:          dir,
		ProtoFile:    dir + ".proto",
		ProtoPackage: dir + ".v1",
		GoPackage:    strings.ToLower(pascal) + "v1",
		Pascal:       pascal,
		ServiceName:  strings.Join(parts, "-") + "-service",
		EnvPrefix:    strings.ToUpper(strings.Join(parts, "_")),
		Port:         port,
		TableName:    dir + "s",
	}, nil
}

func splitServiceName(rawName string) ([]string, error) {
	name := strings.TrimSpace(rawName)
	if name == "" {
		return nil, errors.New("service name is required")
	}
	if !serviceNamePattern.MatchString(name) {
		return nil, fmt.Errorf("service name %q must start with a letter and only contain letters, digits, hyphen or underscore", rawName)
	}
	rawParts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '_'
	})
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			return nil, fmt.Errorf("service name %q contains empty segment", rawName)
		}
		parts = append(parts, part)
	}
	return parts, nil
}

func toPascal(parts []string) string {
	var b strings.Builder
	for _, part := range parts {
		runes := []rune(part)
		for i, r := range runes {
			if i == 0 {
				b.WriteRune(unicode.ToUpper(r))
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}

func ensureServiceDoesNotExist(root string, data serviceTemplateData) error {
	for _, rel := range []string{
		filepath.Join("cmd", data.Dir),
		filepath.Join("internal", data.Dir),
		filepath.Join("api", "proto", data.Dir),
		filepath.Join("api", "gen", data.Dir),
	} {
		if exists(filepath.Join(root, rel)) {
			return fmt.Errorf("service %s already exists: %s", data.Dir, rel)
		}
	}
	return nil
}

func writeServiceFiles(root string, data serviceTemplateData) error {
	files := map[string]string{
		filepath.Join("api", "proto", data.Dir, "v1", data.ProtoFile):     renderServiceTemplate(serviceProtoTemplate, data),
		filepath.Join("cmd", data.Dir, "main.go"):                         renderServiceTemplate(serviceMainTemplate, data),
		filepath.Join("internal", data.Dir, "model", data.Dir+".go"):      renderServiceTemplate(serviceModelTemplate, data),
		filepath.Join("internal", data.Dir, "model", "repository.go"):     renderServiceTemplate(serviceRepositoryTemplate, data),
		filepath.Join("internal", data.Dir, "service", "service.go"):      renderServiceTemplate(serviceUseCaseTemplate, data),
		filepath.Join("internal", data.Dir, "service", "service_test.go"): renderServiceTemplate(serviceUseCaseTestTemplate, data),
		filepath.Join("internal", data.Dir, "repo", "gorm_repository.go"): renderServiceTemplate(serviceGormRepoTemplate, data),
		filepath.Join("internal", data.Dir, "handler", "server.go"):       renderServiceTemplate(serviceHandlerTemplate, data),
		filepath.Join("docs", "services", data.Dir+".md"):                 renderServiceTemplate(serviceDocTemplate, data),
	}
	for rel, content := range files {
		if err := writeNewFile(filepath.Join(root, rel), []byte(content)); err != nil {
			return err
		}
	}
	return nil
}

func writeNewFile(path string, data []byte) error {
	if exists(path) {
		return fmt.Errorf("file already exists: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func renderServiceTemplate(body string, data serviceTemplateData) string {
	tpl := template.Must(template.New("service").Parse(body))
	var out bytes.Buffer
	if err := tpl.Execute(&out, data); err != nil {
		panic(err)
	}
	return out.String()
}

func addServiceMakeTarget(root string, serviceDir string) error {
	path := filepath.Join(root, "Makefile")
	if !exists(path) {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(data)
	target := "run-" + serviceDir
	if strings.Contains(text, "\n"+target+":") || strings.HasPrefix(text, target+":") {
		return nil
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, ".PHONY:") && !strings.Contains(line, " "+target) {
			lines[i] = strings.TrimRight(line, " ") + " " + target
			break
		}
	}
	text = strings.TrimRight(strings.Join(lines, "\n"), "\n")
	text += "\n\n" + target + ":\n\t$(GO) run ./cmd/" + serviceDir + "\n"
	return os.WriteFile(path, []byte(text), 0o644)
}

func gofmtService(root string, serviceDir string) error {
	args := []string{
		filepath.Join("cmd", serviceDir, "main.go"),
		filepath.Join("internal", serviceDir, "model", serviceDir+".go"),
		filepath.Join("internal", serviceDir, "model", "repository.go"),
		filepath.Join("internal", serviceDir, "service", "service.go"),
		filepath.Join("internal", serviceDir, "service", "service_test.go"),
		filepath.Join("internal", serviceDir, "repo", "gorm_repository.go"),
		filepath.Join("internal", serviceDir, "handler", "server.go"),
	}
	return runProjectCommand(root, "gofmt", append([]string{"-w"}, args...)...)
}

func runProjectCommand(root string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

const serviceProtoTemplate = `syntax = "proto3";

package {{ .ProtoPackage }};

option go_package = "{{ .Module }}/api/gen/{{ .Dir }}/v1;{{ .GoPackage }}";

// {{ .Pascal }}Service is the gRPC boundary for the {{ .Dir }} business service.
// Add RPC methods here, then run make proto.
service {{ .Pascal }}Service {
}
`

const serviceMainTemplate = `package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	{{ .GoPackage }} "{{ .Module }}/api/gen/{{ .Dir }}/v1"
	{{ .Dir }}handler "{{ .Module }}/internal/{{ .Dir }}/handler"
	{{ .Dir }}repo "{{ .Module }}/internal/{{ .Dir }}/repo"
	{{ .Dir }}service "{{ .Module }}/internal/{{ .Dir }}/service"
	"{{ .Module }}/pkg/config"
	"{{ .Module }}/pkg/database"
	"{{ .Module }}/pkg/grpcx"
	"{{ .Module }}/pkg/logger"
)

const serviceName = "{{ .ServiceName }}"
const defaultGRPCPort = {{ .Port }}
const grpcPortEnv = "APP_{{ .EnvPrefix }}_GRPC_PORT"

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		panic(err)
	}
	cfg.Log.Service = serviceName
	cfg.Log = logger.WithDailyFileName(cfg.Log, time.Now())

	log, err := logger.New(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	db, err := database.Open(cfg.Database, cfg.MySQL, cfg.PostgreSQL, log)
	if err != nil {
		log.Fatal("open database failed", zap.Error(err))
	}
	if err := {{ .Dir }}repo.AutoMigrate(db); err != nil {
		log.Fatal("migrate {{ .Dir }} database failed", zap.Error(err))
	}

	repo := {{ .Dir }}repo.NewGormRepository(db, log)
	svc := {{ .Dir }}service.NewService(repo, log)
	server := grpc.NewServer(grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor(log)))
	{{ .GoPackage }}.Register{{ .Pascal }}ServiceServer(server, {{ .Dir }}handler.NewServer(svc, log))

	port := grpcPort(defaultGRPCPort, log)
	addr := fmt.Sprintf("%s:%d", cfg.GRPC.Host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n[Service Start Failed]\n  service: %s\n  listen: %s\n  error: %v\n\n", serviceName, addr, err)
		log.Fatal("listen failed", zap.String("addr", addr), zap.Error(err))
	}

	printStartupSummary(cfg.App.Env, addr, port)
	go shutdownOnSignal(server, log)
	if err := server.Serve(listener); err != nil {
		log.Fatal("service stopped unexpectedly", zap.Error(err))
	}
}

func grpcPort(fallback int, log *zap.Logger) int {
	value := strings.TrimSpace(os.Getenv(grpcPortEnv))
	if value == "" {
		return fallback
	}
	port, err := strconv.Atoi(value)
	if err != nil || port <= 0 || port > 65535 {
		log.Warn("invalid grpc port env, using fallback", zap.String("env", grpcPortEnv), zap.String("value", value), zap.Int("fallback", fallback))
		return fallback
	}
	return port
}

func printStartupSummary(env string, addr string, port int) {
	host := strings.Split(addr, ":")[0]
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	fmt.Fprintf(os.Stdout, "\n[Service Started]\n")
	fmt.Fprintf(os.Stdout, "  service: %s\n", serviceName)
	fmt.Fprintf(os.Stdout, "  env: %s\n", env)
	fmt.Fprintf(os.Stdout, "  listen: %s\n", addr)
	fmt.Fprintf(os.Stdout, "  grpc: %s:%d\n", host, port)
	fmt.Fprintf(os.Stdout, "  port_env: %s\n\n", grpcPortEnv)
}

func shutdownOnSignal(server *grpc.Server, log *zap.Logger) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Info("service shutting down", zap.String("service", serviceName))
	server.GracefulStop()
}
`

const serviceModelTemplate = `package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	Err{{ .Pascal }}NotFound = errors.New("{{ .Dir }} not found")
	ErrInvalid{{ .Pascal }} = errors.New("invalid {{ .Dir }}")
)

// {{ .Pascal }} is the aggregate root for the {{ .Dir }} business service.
// Add domain fields and behavior here before exposing them through service use cases.
type {{ .Pascal }} struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// New{{ .Pascal }} creates a new aggregate with framework-managed identity fields.
func New{{ .Pascal }}() *{{ .Pascal }} {
	now := time.Now().UTC()
	return &{{ .Pascal }}{
		ID:        uuid.NewString(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}
`

const serviceRepositoryTemplate = `package model

// Repository defines persistence behavior required by the {{ .Dir }} service layer.
// Add methods here as business use cases are introduced.
type Repository interface {
}
`

const serviceUseCaseTemplate = `package service

import (
	"go.uber.org/zap"

	"{{ .Module }}/internal/{{ .Dir }}/model"
)

// Service orchestrates {{ .Dir }} use cases.
type Service struct {
	repo model.Repository
	log  *zap.Logger
}

// NewService constructs the {{ .Dir }} use-case service.
func NewService(repo model.Repository, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	return &Service{repo: repo, log: log}
}
`

const serviceUseCaseTestTemplate = `package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewService(t *testing.T) {
	svc := NewService(nil, zap.NewNop())

	require.NotNil(t, svc)
}
`

const serviceGormRepoTemplate = `package repo

import (
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// {{ .Pascal }}Model is the Gorm persistence model for the {{ .TableName }} table.
// Add storage fields after the domain model is defined.
type {{ .Pascal }}Model struct {
	ID        string ` + "`gorm:\"primaryKey;size:64\"`" + `
	CreatedAt time.Time
	UpdatedAt time.Time
}

func ({{ .Pascal }}Model) TableName() string {
	return "{{ .TableName }}"
}

// GormRepository persists {{ .Dir }} aggregates with Gorm.
type GormRepository struct {
	db  *gorm.DB
	log *zap.Logger
}

// NewGormRepository constructs a {{ .Dir }} repository with optional structured logging.
func NewGormRepository(db *gorm.DB, loggers ...*zap.Logger) *GormRepository {
	log := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}
	return &GormRepository{db: db, log: log}
}

// AutoMigrate creates or updates the {{ .TableName }} table schema.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&{{ .Pascal }}Model{})
}
`

const serviceHandlerTemplate = `package handler

import (
	"go.uber.org/zap"

	{{ .GoPackage }} "{{ .Module }}/api/gen/{{ .Dir }}/v1"
	"{{ .Module }}/internal/{{ .Dir }}/service"
)

// Server adapts {{ .Dir }} gRPC requests to service use cases.
type Server struct {
	{{ .GoPackage }}.Unimplemented{{ .Pascal }}ServiceServer
	svc *service.Service
	log *zap.Logger
}

// NewServer constructs the {{ .Dir }} gRPC server adapter.
func NewServer(svc *service.Service, log *zap.Logger) *Server {
	if log == nil {
		log = zap.NewNop()
	}
	return &Server{svc: svc, log: log}
}
`

const serviceDocTemplate = `# {{ .Pascal }} 服务开发说明

本服务由以下命令生成：

~~~bash
bw-cli service {{ .InputName }} --port {{ .Port }}
~~~

## 目录结构

~~~text
api/proto/{{ .Dir }}/v1/{{ .ProtoFile }}       # gRPC 协议定义
api/gen/{{ .Dir }}/v1                          # make proto 生成代码
cmd/{{ .Dir }}/main.go                         # gRPC 服务启动入口
internal/{{ .Dir }}/model                      # 领域实体和仓储接口
internal/{{ .Dir }}/service                    # 业务用例
internal/{{ .Dir }}/repo                       # Gorm 仓储实现
internal/{{ .Dir }}/handler                    # gRPC 入站适配器
~~~

## 启动

~~~bash
make proto
make run-{{ .Dir }}
~~~

默认端口是 ` + "`{{ .Port }}`" + `，可以通过环境变量覆盖：

~~~bash
export APP_{{ .EnvPrefix }}_GRPC_PORT={{ .Port }}
~~~

Windows PowerShell：

~~~powershell
$env:APP_{{ .EnvPrefix }}_GRPC_PORT="{{ .Port }}"; make run-{{ .Dir }}
~~~

## 开发顺序

1. 在 ` + "`api/proto/{{ .Dir }}/v1/{{ .ProtoFile }}`" + ` 中定义 RPC、Request、Response。
2. 执行 ` + "`make proto`" + ` 生成 ` + "`api/gen/{{ .Dir }}/v1`" + `。
3. 在 ` + "`internal/{{ .Dir }}/model`" + ` 补充领域实体、业务错误和仓储接口。
4. 在 ` + "`internal/{{ .Dir }}/service`" + ` 编写业务用例。
5. 在 ` + "`internal/{{ .Dir }}/repo`" + ` 实现数据库访问。
6. 在 ` + "`internal/{{ .Dir }}/handler`" + ` 将 gRPC 请求转成业务命令。

## 每一层怎么写，为什么这么写

| 层级 | 写什么 | 为什么 |
| --- | --- | --- |
| ` + "`api/proto/{{ .Dir }}/v1`" + ` | RPC、Request、Response、` + "`go_package`" + ` | 先稳定外部契约，避免内部模型直接暴露 |
| ` + "`api/gen/{{ .Dir }}/v1`" + ` | ` + "`make proto`" + ` 生成代码 | 保持 proto 与 Go 类型一致，不手写 |
| ` + "`cmd/{{ .Dir }}`" + ` | 配置、日志、数据库、gRPC server 组装 | main 只负责依赖装配，不写业务 |
| ` + "`internal/{{ .Dir }}/model`" + ` | 领域实体、业务错误、Repository 接口 | 业务核心不依赖 Gin、gRPC、Gorm |
| ` + "`internal/{{ .Dir }}/service`" + ` | 用例编排、事务意图、DTO | 表达业务流程，依赖接口而不是数据库实现 |
| ` + "`internal/{{ .Dir }}/repo`" + ` | Gorm/MongoDB/Redis 等实现 | 数据库访问集中管理，方便替换和测试 |
| ` + "`internal/{{ .Dir }}/handler`" + ` | gRPC request/response 适配 | 协议转换和错误映射，不写数据库逻辑 |

## model 层

` + "`model`" + ` 放业务核心，不引入 Gorm、Gin、gRPC SDK。

~~~go
package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	Err{{ .Pascal }}NotFound = errors.New("{{ .Dir }} not found")
	ErrInvalid{{ .Pascal }} = errors.New("invalid {{ .Dir }}")
)

type {{ .Pascal }} struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func New{{ .Pascal }}() *{{ .Pascal }} {
	now := time.Now().UTC()
	return &{{ .Pascal }}{
		ID:        uuid.NewString(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}
~~~

仓储接口也放在 ` + "`model`" + `：

~~~go
package model

type Repository interface {
	// Add methods after use cases are clear.
}
~~~

这样写的原因是：` + "`service`" + ` 只关心业务需要的能力，不关心底层用 MySQL、PostgreSQL、MongoDB 还是测试 fake。

## service 层

` + "`service`" + ` 编排业务流程，只依赖 ` + "`model.Repository`" + `。

~~~go
package service

import (
	"go.uber.org/zap"

	"{{ .Module }}/internal/{{ .Dir }}/model"
)

type Service struct {
	repo model.Repository
	log  *zap.Logger
}

func NewService(repo model.Repository, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	return &Service{repo: repo, log: log}
}
~~~

新增业务时建议先定义命令对象，例如 ` + "`CreateCommand`" + `，再在方法里调用领域模型和仓储接口。这样 handler 不会堆业务判断，service 也更容易写单元测试。

## repo 层：数据库在哪里操作，如何操作

数据库操作只放在 ` + "`internal/{{ .Dir }}/repo`" + `。启动入口 ` + "`cmd/{{ .Dir }}/main.go`" + ` 打开数据库并注入 repo：

~~~go
db, err := database.Open(cfg.Database, cfg.MySQL, cfg.PostgreSQL, log)
repo := {{ .Dir }}repo.NewGormRepository(db, log)
svc := {{ .Dir }}service.NewService(repo, log)
~~~

Gorm 仓储示例：

~~~go
type {{ .Pascal }}Model struct {
	ID        string ` + "`gorm:\"primaryKey;size:64\"`" + `
	CreatedAt time.Time
	UpdatedAt time.Time
}

func ({{ .Pascal }}Model) TableName() string {
	return "{{ .TableName }}"
}

type GormRepository struct {
	db  *gorm.DB
	log *zap.Logger
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&{{ .Pascal }}Model{})
}
~~~

数据库操作规则：

- ` + "`handler`" + ` 不直接操作数据库。
- ` + "`service`" + ` 不直接使用 ` + "`*gorm.DB`" + `。
- ` + "`model`" + ` 不写 Gorm tag，避免领域模型和数据库实现耦合。
- 查询、分页、事务、锁、索引相关实现都放在 ` + "`repo`" + `。
- 需要事务时，在 repo 层内部使用 ` + "`db.Transaction(func(tx *gorm.DB) error { ... })`" + `。
- 多数据源时保持接口不变，例如 ` + "`GormRepository`" + `、` + "`MongoRepository`" + ` 都实现 ` + "`model.Repository`" + `。

## handler 层

` + "`handler`" + ` 只做 gRPC 协议转换：

1. 从 proto request 取字段。
2. 组装 service command。
3. 调用 service。
4. 把 DTO 转成 proto response。
5. 把业务错误转成统一错误。

不要在 handler 中写 SQL、Gorm 查询、复杂业务判断。
`
