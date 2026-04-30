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
	GoIdent      string
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
	if err := writeGatewayServiceFiles(root, data); err != nil {
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
		GoIdent:      lowerFirst(pascal),
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

func lowerFirst(value string) string {
	if value == "" {
		return value
	}
	runes := []rune(value)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
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

func writeGatewayServiceFiles(root string, data serviceTemplateData) error {
	routerDir := filepath.Join(root, "internal", "gateway", "router")
	if !exists(routerDir) {
		return nil
	}
	commonPath := filepath.Join(root, "internal", "gateway", "handler", "common.go")
	if err := ensureGatewayCommonFile(commonPath, data); err != nil {
		return err
	}
	files := map[string]string{
		filepath.Join("internal", "gateway", "request", data.Dir+"_request.go"): renderServiceTemplate(gatewayRequestTemplate, data),
		filepath.Join("internal", "gateway", "handler", data.Dir+"_handler.go"): renderServiceTemplate(gatewayHandlerTemplate, data),
		filepath.Join("internal", "gateway", "router", data.Dir+"_routes.go"):   renderServiceTemplate(gatewayRoutesTemplate, data),
	}
	for rel, content := range files {
		if err := writeNewFile(filepath.Join(root, rel), []byte(content)); err != nil {
			return err
		}
	}
	if err := patchGatewayRouter(root, data); err != nil {
		return err
	}
	return nil
}

func ensureGatewayCommonFile(path string, data serviceTemplateData) error {
	if !exists(path) {
		return writeNewFile(path, []byte(renderServiceTemplate(gatewayCommonTemplate, data)))
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(content)
	if strings.Contains(text, "func gatewayGRPCTarget") {
		return nil
	}
	text = ensureImport(text, "\"os\"")
	text = ensureImport(text, "\"strings\"")
	text = strings.TrimRight(text, "\n") + "\n\n" + gatewayTargetFunction
	return os.WriteFile(path, []byte(text), 0o644)
}

func ensureImport(text string, quotedPackage string) string {
	if strings.Contains(text, quotedPackage) {
		return text
	}
	if strings.Contains(text, "import (\n") {
		return strings.Replace(text, "import (\n", "import (\n\t"+quotedPackage+"\n", 1)
	}
	return text
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
	for _, rel := range []string{
		filepath.Join("internal", "gateway", "handler", "common.go"),
		filepath.Join("internal", "gateway", "request", serviceDir+"_request.go"),
		filepath.Join("internal", "gateway", "handler", serviceDir+"_handler.go"),
		filepath.Join("internal", "gateway", "router", serviceDir+"_routes.go"),
		filepath.Join("internal", "gateway", "router", "router.go"),
		filepath.Join("internal", "gateway", "router", "v1.go"),
	} {
		if exists(filepath.Join(root, rel)) {
			args = append(args, rel)
		}
	}
	return runProjectCommand(root, "gofmt", append([]string{"-w"}, args...)...)
}

func patchGatewayRouter(root string, data serviceTemplateData) error {
	routerPath := filepath.Join(root, "internal", "gateway", "router", "router.go")
	if exists(routerPath) {
		routerBytes, err := os.ReadFile(routerPath)
		if err != nil {
			return err
		}
		routerText := string(routerBytes)
		if strings.Contains(routerText, "registerAPIRoutes(r)") {
			routerText = strings.Replace(routerText, "registerAPIRoutes(r)", "registerAPIRoutes(r, log)", 1)
			if err := os.WriteFile(routerPath, []byte(routerText), 0o644); err != nil {
				return err
			}
		}
	}

	v1Path := filepath.Join(root, "internal", "gateway", "router", "v1.go")
	if !exists(v1Path) {
		return nil
	}
	v1Bytes, err := os.ReadFile(v1Path)
	if err != nil {
		return err
	}
	v1Text := string(v1Bytes)
	registration := fmt.Sprintf("register%sRoutes(v1, handler.New%sHandler(log))", data.Pascal, data.Pascal)
	if strings.Contains(v1Text, registration) {
		return nil
	}
	if strings.Contains(v1Text, "func registerAPIRoutes(r *gin.Engine)") {
		return os.WriteFile(v1Path, []byte(renderServiceTemplate(cleanGatewayV1WithServiceTemplate, data)), 0o644)
	}
	if !strings.Contains(v1Text, "func registerAPIRoutes(r *gin.Engine, log *zap.Logger)") {
		return nil
	}
	if strings.Contains(v1Text, "\n\t_ = v1\n") {
		v1Text = strings.Replace(v1Text, "\n\t_ = v1\n", "\n\t"+registration+"\n", 1)
		return os.WriteFile(v1Path, []byte(v1Text), 0o644)
	}
	index := strings.LastIndex(v1Text, "\n}")
	if index == -1 {
		return nil
	}
	v1Text = v1Text[:index] + "\n\t" + registration + v1Text[index:]
	return os.WriteFile(v1Path, []byte(v1Text), 0o644)
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
// The default CRUD contract is ready to call. Extend messages and RPCs as business grows.
service {{ .Pascal }}Service {
  rpc Create{{ .Pascal }}(Create{{ .Pascal }}Request) returns ({{ .Pascal }}Response);
  rpc Get{{ .Pascal }}(Get{{ .Pascal }}Request) returns ({{ .Pascal }}Response);
  rpc List{{ .Pascal }}s(List{{ .Pascal }}sRequest) returns (List{{ .Pascal }}sResponse);
  rpc Update{{ .Pascal }}(Update{{ .Pascal }}Request) returns ({{ .Pascal }}Response);
  rpc Delete{{ .Pascal }}(Delete{{ .Pascal }}Request) returns (Delete{{ .Pascal }}Response);
}

message Create{{ .Pascal }}Request {
  string name = 1;
  string description = 2;
}

message Get{{ .Pascal }}Request {
  string id = 1;
}

message List{{ .Pascal }}sRequest {
  int32 page = 1;
  int32 page_size = 2;
}

message Update{{ .Pascal }}Request {
  string id = 1;
  string name = 2;
  string description = 3;
}

message Delete{{ .Pascal }}Request {
  string id = 1;
}

message {{ .Pascal }}Response {
  string id = 1;
  string name = 2;
  string description = 3;
  string created_at = 4;
  string updated_at = 5;
}

message List{{ .Pascal }}sResponse {
  repeated {{ .Pascal }}Response items = 1;
  int64 total = 2;
}

message Delete{{ .Pascal }}Response {
  bool success = 1;
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
	{{ .GoIdent }}handler "{{ .Module }}/internal/{{ .Dir }}/handler"
	{{ .GoIdent }}repo "{{ .Module }}/internal/{{ .Dir }}/repo"
	{{ .GoIdent }}service "{{ .Module }}/internal/{{ .Dir }}/service"
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
	if err := {{ .GoIdent }}repo.AutoMigrate(db); err != nil {
		log.Fatal("migrate {{ .Dir }} database failed", zap.Error(err))
	}

	repo := {{ .GoIdent }}repo.NewGormRepository(db, log)
	svc := {{ .GoIdent }}service.NewService(repo, log)
	server := grpc.NewServer(grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor(log)))
	{{ .GoPackage }}.Register{{ .Pascal }}ServiceServer(server, {{ .GoIdent }}handler.NewServer(svc, log))

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
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	Err{{ .Pascal }}NotFound = errors.New("{{ .Dir }} not found")
	ErrInvalid{{ .Pascal }} = errors.New("invalid {{ .Dir }}")
)

// {{ .Pascal }} is the aggregate root for the {{ .Dir }} business service.
// Replace Name and Description with real business fields when the domain is clear.
type {{ .Pascal }} struct {
	ID        string
	Name        string
	Description string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// New{{ .Pascal }} validates input and creates an aggregate with framework-managed identity fields.
func New{{ .Pascal }}(name string, description string) (*{{ .Pascal }}, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return nil, ErrInvalid{{ .Pascal }}
	}
	now := time.Now().UTC()
	return &{{ .Pascal }}{
		ID:          uuid.NewString(),
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// Update changes mutable fields while keeping validation inside the domain model.
func (item *{{ .Pascal }}) Update(name string, description string) error {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if item == nil || item.ID == "" || name == "" {
		return ErrInvalid{{ .Pascal }}
	}
	item.Name = name
	item.Description = description
	item.UpdatedAt = time.Now().UTC()
	return nil
}
`

const serviceRepositoryTemplate = `package model

import "context"

// Repository defines persistence behavior required by the {{ .Dir }} service layer.
type Repository interface {
	Save(ctx context.Context, item *{{ .Pascal }}) error
	FindByID(ctx context.Context, id string) (*{{ .Pascal }}, error)
	List(ctx context.Context, offset int, limit int) ([]*{{ .Pascal }}, int64, error)
	Delete(ctx context.Context, id string) error
}
`

const serviceUseCaseTemplate = `package service

import (
	"context"
	"time"

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

// CreateCommand contains input for creating a {{ .Dir }} record.
type CreateCommand struct {
	Name        string
	Description string
}

// UpdateCommand contains input for updating a {{ .Dir }} record.
type UpdateCommand struct {
	ID          string
	Name        string
	Description string
}

// ListCommand contains pagination input for listing {{ .Dir }} records.
type ListCommand struct {
	Page     int32
	PageSize int32
}

// {{ .Pascal }}DTO is returned by use cases and converted by handlers.
type {{ .Pascal }}DTO struct {
	ID          string
	Name        string
	Description string
	CreatedAt   string
	UpdatedAt   string
}

// List{{ .Pascal }}DTO contains paginated list output.
type List{{ .Pascal }}DTO struct {
	Items []*{{ .Pascal }}DTO
	Total int64
}

// Create creates a {{ .Dir }} record.
func (s *Service) Create(ctx context.Context, cmd CreateCommand) (*{{ .Pascal }}DTO, error) {
	item, err := model.New{{ .Pascal }}(cmd.Name, cmd.Description)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, item); err != nil {
		return nil, err
	}
	s.log.Info("{{ .Dir }} created", zap.String("aggregate_id", item.ID), zap.String("use_case", "Create{{ .Pascal }}"))
	return toDTO(item), nil
}

// Get returns one {{ .Dir }} record by id.
func (s *Service) Get(ctx context.Context, id string) (*{{ .Pascal }}DTO, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toDTO(item), nil
}

// List returns paginated {{ .Dir }} records.
func (s *Service) List(ctx context.Context, cmd ListCommand) (*List{{ .Pascal }}DTO, error) {
	offset, limit := normalizePagination(cmd.Page, cmd.PageSize)
	items, total, err := s.repo.List(ctx, offset, limit)
	if err != nil {
		return nil, err
	}
	output := &List{{ .Pascal }}DTO{Items: make([]*{{ .Pascal }}DTO, 0, len(items)), Total: total}
	for _, item := range items {
		output.Items = append(output.Items, toDTO(item))
	}
	return output, nil
}

// Update changes one {{ .Dir }} record by id.
func (s *Service) Update(ctx context.Context, cmd UpdateCommand) (*{{ .Pascal }}DTO, error) {
	item, err := s.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	if err := item.Update(cmd.Name, cmd.Description); err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, item); err != nil {
		return nil, err
	}
	s.log.Info("{{ .Dir }} updated", zap.String("aggregate_id", item.ID), zap.String("use_case", "Update{{ .Pascal }}"))
	return toDTO(item), nil
}

// Delete removes one {{ .Dir }} record by id.
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.log.Info("{{ .Dir }} deleted", zap.String("aggregate_id", id), zap.String("use_case", "Delete{{ .Pascal }}"))
	return nil
}

func normalizePagination(page int32, pageSize int32) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return int((page - 1) * pageSize), int(pageSize)
}

func toDTO(item *model.{{ .Pascal }}) *{{ .Pascal }}DTO {
	return &{{ .Pascal }}DTO{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   formatTime(item.CreatedAt),
		UpdatedAt:   formatTime(item.UpdatedAt),
	}
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}
`

const serviceUseCaseTestTemplate = `package service

import (
	"context"
	"testing"
	"time"

	"{{ .Module }}/internal/{{ .Dir }}/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewService(t *testing.T) {
	svc := NewService(nil, zap.NewNop())

	require.NotNil(t, svc)
}

func TestServiceCRUD(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	svc := NewService(repo, zap.NewNop())

	created, err := svc.Create(ctx, CreateCommand{Name: "first", Description: "created from service test"})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "first", created.Name)

	got, err := svc.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)

	list, err := svc.List(ctx, ListCommand{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), list.Total)
	require.Len(t, list.Items, 1)

	updated, err := svc.Update(ctx, UpdateCommand{ID: created.ID, Name: "updated", Description: "updated from service test"})
	require.NoError(t, err)
	require.Equal(t, "updated", updated.Name)

	require.NoError(t, svc.Delete(ctx, created.ID))
	_, err = svc.Get(ctx, created.ID)
	require.ErrorIs(t, err, model.Err{{ .Pascal }}NotFound)
}

type fakeRepository struct {
	items map[string]*model.{{ .Pascal }}
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{items: make(map[string]*model.{{ .Pascal }})}
}

func (r *fakeRepository) Save(ctx context.Context, item *model.{{ .Pascal }}) error {
	copy := *item
	r.items[item.ID] = &copy
	return nil
}

func (r *fakeRepository) FindByID(ctx context.Context, id string) (*model.{{ .Pascal }}, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, model.Err{{ .Pascal }}NotFound
	}
	copy := *item
	return &copy, nil
}

func (r *fakeRepository) List(ctx context.Context, offset int, limit int) ([]*model.{{ .Pascal }}, int64, error) {
	items := make([]*model.{{ .Pascal }}, 0, len(r.items))
	for _, item := range r.items {
		copy := *item
		items = append(items, &copy)
	}
	if offset > len(items) {
		return nil, int64(len(items)), nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], int64(len(items)), nil
}

func (r *fakeRepository) Delete(ctx context.Context, id string) error {
	if _, ok := r.items[id]; !ok {
		return model.Err{{ .Pascal }}NotFound
	}
	delete(r.items, id)
	return nil
}

var _ model.Repository = (*fakeRepository)(nil)
var _ = time.RFC3339Nano
`

const serviceGormRepoTemplate = `package repo

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"{{ .Module }}/internal/{{ .Dir }}/model"
)

// {{ .Pascal }}Model is the Gorm persistence model for the {{ .TableName }} table.
type {{ .Pascal }}Model struct {
	ID          string ` + "`gorm:\"primaryKey;size:64\"`" + `
	Name        string ` + "`gorm:\"size:128;not null\"`" + `
	Description string ` + "`gorm:\"type:text\"`" + `
	CreatedAt   time.Time
	UpdatedAt   time.Time
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

// Save inserts or updates a {{ .Dir }} aggregate.
func (r *GormRepository) Save(ctx context.Context, item *model.{{ .Pascal }}) error {
	start := time.Now()
	tx := r.db.WithContext(ctx).Save(toRecord(item))
	r.logOperation("Save", tx.RowsAffected, start, tx.Error)
	return tx.Error
}

// FindByID loads a {{ .Dir }} aggregate by id.
func (r *GormRepository) FindByID(ctx context.Context, id string) (*model.{{ .Pascal }}, error) {
	start := time.Now()
	var record {{ .Pascal }}Model
	tx := r.db.WithContext(ctx).Where("id = ?", id).First(&record)
	err := tx.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = model.Err{{ .Pascal }}NotFound
	}
	if err != nil {
		r.logOperation("FindByID", tx.RowsAffected, start, err)
		return nil, err
	}
	r.logOperation("FindByID", tx.RowsAffected, start, nil)
	return toDomain(&record), nil
}

// List loads paginated {{ .Dir }} aggregates.
func (r *GormRepository) List(ctx context.Context, offset int, limit int) ([]*model.{{ .Pascal }}, int64, error) {
	start := time.Now()
	var total int64
	countTx := r.db.WithContext(ctx).Model(&{{ .Pascal }}Model{}).Count(&total)
	if countTx.Error != nil {
		r.logOperation("Count", countTx.RowsAffected, start, countTx.Error)
		return nil, 0, countTx.Error
	}
	var records []{{ .Pascal }}Model
	tx := r.db.WithContext(ctx).
		Order("created_at desc").
		Offset(offset).
		Limit(limit).
		Find(&records)
	if tx.Error != nil {
		r.logOperation("List", tx.RowsAffected, start, tx.Error)
		return nil, 0, tx.Error
	}
	items := make([]*model.{{ .Pascal }}, 0, len(records))
	for i := range records {
		items = append(items, toDomain(&records[i]))
	}
	r.logOperation("List", tx.RowsAffected, start, nil)
	return items, total, nil
}

// Delete removes a {{ .Dir }} aggregate by id.
func (r *GormRepository) Delete(ctx context.Context, id string) error {
	start := time.Now()
	tx := r.db.WithContext(ctx).Where("id = ?", id).Delete(&{{ .Pascal }}Model{})
	err := tx.Error
	if err == nil && tx.RowsAffected == 0 {
		err = model.Err{{ .Pascal }}NotFound
	}
	r.logOperation("Delete", tx.RowsAffected, start, err)
	return err
}

func (r *GormRepository) logOperation(operation string, rows int64, start time.Time, err error) {
	fields := []zap.Field{
		zap.String("repository", "{{ .Dir }}"),
		zap.String("operation", operation),
		zap.Int64("rows_affected", rows),
		zap.Float64("latency_ms", float64(time.Since(start).Microseconds())/1000),
	}
	if err != nil {
		fields = append(fields, zap.Error(err))
		r.log.Warn("repository operation completed with error", fields...)
		return
	}
	r.log.Info("repository operation completed", fields...)
}

func toRecord(item *model.{{ .Pascal }}) *{{ .Pascal }}Model {
	return &{{ .Pascal }}Model{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func toDomain(record *{{ .Pascal }}Model) *model.{{ .Pascal }} {
	return &model.{{ .Pascal }}{
		ID:          record.ID,
		Name:        record.Name,
		Description: record.Description,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}

var _ model.Repository = (*GormRepository)(nil)
`

const serviceHandlerTemplate = `package handler

import (
	"context"
	stderrors "errors"

	"go.uber.org/zap"

	{{ .GoPackage }} "{{ .Module }}/api/gen/{{ .Dir }}/v1"
	"{{ .Module }}/internal/{{ .Dir }}/model"
	"{{ .Module }}/internal/{{ .Dir }}/service"
	apperrors "{{ .Module }}/pkg/errors"
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

// Create{{ .Pascal }} handles the create RPC.
func (s *Server) Create{{ .Pascal }}(ctx context.Context, req *{{ .GoPackage }}.Create{{ .Pascal }}Request) (*{{ .GoPackage }}.{{ .Pascal }}Response, error) {
	item, err := s.svc.Create(ctx, service.CreateCommand{
		Name:        req.GetName(),
		Description: req.GetDescription(),
	})
	if err != nil {
		return nil, map{{ .Pascal }}Error(err)
	}
	return toProto(item), nil
}

// Get{{ .Pascal }} handles lookup by id.
func (s *Server) Get{{ .Pascal }}(ctx context.Context, req *{{ .GoPackage }}.Get{{ .Pascal }}Request) (*{{ .GoPackage }}.{{ .Pascal }}Response, error) {
	item, err := s.svc.Get(ctx, req.GetId())
	if err != nil {
		return nil, map{{ .Pascal }}Error(err)
	}
	return toProto(item), nil
}

// List{{ .Pascal }}s handles paginated listing.
func (s *Server) List{{ .Pascal }}s(ctx context.Context, req *{{ .GoPackage }}.List{{ .Pascal }}sRequest) (*{{ .GoPackage }}.List{{ .Pascal }}sResponse, error) {
	list, err := s.svc.List(ctx, service.ListCommand{
		Page:     req.GetPage(),
		PageSize: req.GetPageSize(),
	})
	if err != nil {
		return nil, map{{ .Pascal }}Error(err)
	}
	resp := &{{ .GoPackage }}.List{{ .Pascal }}sResponse{
		Items: make([]*{{ .GoPackage }}.{{ .Pascal }}Response, 0, len(list.Items)),
		Total: list.Total,
	}
	for _, item := range list.Items {
		resp.Items = append(resp.Items, toProto(item))
	}
	return resp, nil
}

// Update{{ .Pascal }} handles updates by id.
func (s *Server) Update{{ .Pascal }}(ctx context.Context, req *{{ .GoPackage }}.Update{{ .Pascal }}Request) (*{{ .GoPackage }}.{{ .Pascal }}Response, error) {
	item, err := s.svc.Update(ctx, service.UpdateCommand{
		ID:          req.GetId(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
	})
	if err != nil {
		return nil, map{{ .Pascal }}Error(err)
	}
	return toProto(item), nil
}

// Delete{{ .Pascal }} handles deletion by id.
func (s *Server) Delete{{ .Pascal }}(ctx context.Context, req *{{ .GoPackage }}.Delete{{ .Pascal }}Request) (*{{ .GoPackage }}.Delete{{ .Pascal }}Response, error) {
	if err := s.svc.Delete(ctx, req.GetId()); err != nil {
		return nil, map{{ .Pascal }}Error(err)
	}
	return &{{ .GoPackage }}.Delete{{ .Pascal }}Response{Success: true}, nil
}

func toProto(item *service.{{ .Pascal }}DTO) *{{ .GoPackage }}.{{ .Pascal }}Response {
	return &{{ .GoPackage }}.{{ .Pascal }}Response{
		Id:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func map{{ .Pascal }}Error(err error) error {
	switch {
	case stderrors.Is(err, model.ErrInvalid{{ .Pascal }}):
		return apperrors.InvalidArgument("invalid_{{ .Dir }}", "invalid {{ .Dir }} input")
	case stderrors.Is(err, model.Err{{ .Pascal }}NotFound):
		return apperrors.NotFound("{{ .Dir }}_not_found", "{{ .Dir }} not found")
	default:
		return apperrors.Wrap(apperrors.KindInternal, "{{ .Dir }}_service_error", "{{ .Dir }} service error", err)
	}
}
`

const gatewayCommonTemplate = `package handler

import (
	"context"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/metadata"

	"{{ .Module }}/pkg/grpcx"
	"{{ .Module }}/pkg/httpx"
)

// outgoingContext forwards gateway metadata such as request id to downstream gRPC calls.
func outgoingContext(c *gin.Context) context.Context {
	return metadata.AppendToOutgoingContext(c.Request.Context(), grpcx.MetadataRequestID, httpx.RequestID(c))
}

func gatewayGRPCTarget(envName string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(envName))
	if value == "" {
		return fallback
	}
	return value
}
`

const gatewayTargetFunction = `func gatewayGRPCTarget(envName string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(envName))
	if value == "" {
		return fallback
	}
	return value
}
`

const gatewayRequestTemplate = `package request

// Create{{ .Pascal }}Request is the JSON payload used by POST /api/v1/{{ .TableName }}.
type Create{{ .Pascal }}Request struct {
	Name        string ` + "`json:\"name\" binding:\"required\"`" + `
	Description string ` + "`json:\"description\"`" + `
}

// Update{{ .Pascal }}Request is the JSON payload used by PUT /api/v1/{{ .TableName }}/:id.
type Update{{ .Pascal }}Request struct {
	Name        string ` + "`json:\"name\" binding:\"required\"`" + `
	Description string ` + "`json:\"description\"`" + `
}

// List{{ .Pascal }}Request is the query string payload used by GET /api/v1/{{ .TableName }}.
type List{{ .Pascal }}Request struct {
	Page     int32 ` + "`form:\"page\"`" + `
	PageSize int32 ` + "`form:\"page_size\"`" + `
}
`

const gatewayHandlerTemplate = `package handler

import (
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	{{ .GoPackage }} "{{ .Module }}/api/gen/{{ .Dir }}/v1"
	"{{ .Module }}/internal/gateway/request"
	apperrors "{{ .Module }}/pkg/errors"
	"{{ .Module }}/pkg/httpx"
)

const {{ .GoIdent }}GatewayTargetEnv = "APP_{{ .EnvPrefix }}_GRPC_TARGET"
const {{ .GoIdent }}GatewayDefaultTarget = "127.0.0.1:{{ .Port }}"

// {{ .Pascal }}Handler adapts {{ .Dir }} HTTP endpoints to the generated gRPC client.
type {{ .Pascal }}Handler struct {
	target string
	client {{ .GoPackage }}.{{ .Pascal }}ServiceClient
	conn   *grpc.ClientConn
	once   sync.Once
	err    error
	log    *zap.Logger
}

// New{{ .Pascal }}Handler builds a gateway handler with a default target that needs no config changes.
func New{{ .Pascal }}Handler(log *zap.Logger) *{{ .Pascal }}Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &{{ .Pascal }}Handler{
		target: gatewayGRPCTarget({{ .GoIdent }}GatewayTargetEnv, {{ .GoIdent }}GatewayDefaultTarget),
		log:    log,
	}
}

// Create proxies POST /api/v1/{{ .TableName }} to Create{{ .Pascal }}.
func (h *{{ .Pascal }}Handler) Create(c *gin.Context) {
	var req request.Create{{ .Pascal }}Request
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	client, err := h.grpcClient()
	if err != nil {
		httpx.Error(c, apperrors.Wrap(apperrors.KindInternal, "{{ .Dir }}_grpc_client_error", "{{ .Dir }} grpc client error", err))
		return
	}
	resp, err := client.Create{{ .Pascal }}(outgoingContext(c), &{{ .GoPackage }}.Create{{ .Pascal }}Request{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	h.log.Info("gateway {{ .Dir }} create proxied", zap.String("request_id", httpx.RequestID(c)), zap.String("aggregate_id", resp.GetId()))
	httpx.Created(c, resp)
}

// Get proxies GET /api/v1/{{ .TableName }}/:id to Get{{ .Pascal }}.
func (h *{{ .Pascal }}Handler) Get(c *gin.Context) {
	client, err := h.grpcClient()
	if err != nil {
		httpx.Error(c, apperrors.Wrap(apperrors.KindInternal, "{{ .Dir }}_grpc_client_error", "{{ .Dir }} grpc client error", err))
		return
	}
	resp, err := client.Get{{ .Pascal }}(outgoingContext(c), &{{ .GoPackage }}.Get{{ .Pascal }}Request{Id: c.Param("id")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// List proxies GET /api/v1/{{ .TableName }} to List{{ .Pascal }}s.
func (h *{{ .Pascal }}Handler) List(c *gin.Context) {
	var req request.List{{ .Pascal }}Request
	if err := c.ShouldBindQuery(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	client, err := h.grpcClient()
	if err != nil {
		httpx.Error(c, apperrors.Wrap(apperrors.KindInternal, "{{ .Dir }}_grpc_client_error", "{{ .Dir }} grpc client error", err))
		return
	}
	resp, err := client.List{{ .Pascal }}s(outgoingContext(c), &{{ .GoPackage }}.List{{ .Pascal }}sRequest{
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// Update proxies PUT /api/v1/{{ .TableName }}/:id to Update{{ .Pascal }}.
func (h *{{ .Pascal }}Handler) Update(c *gin.Context) {
	var req request.Update{{ .Pascal }}Request
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	client, err := h.grpcClient()
	if err != nil {
		httpx.Error(c, apperrors.Wrap(apperrors.KindInternal, "{{ .Dir }}_grpc_client_error", "{{ .Dir }} grpc client error", err))
		return
	}
	resp, err := client.Update{{ .Pascal }}(outgoingContext(c), &{{ .GoPackage }}.Update{{ .Pascal }}Request{
		Id:          c.Param("id"),
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	h.log.Info("gateway {{ .Dir }} update proxied", zap.String("request_id", httpx.RequestID(c)), zap.String("aggregate_id", resp.GetId()))
	httpx.OK(c, resp)
}

// Delete proxies DELETE /api/v1/{{ .TableName }}/:id to Delete{{ .Pascal }}.
func (h *{{ .Pascal }}Handler) Delete(c *gin.Context) {
	client, err := h.grpcClient()
	if err != nil {
		httpx.Error(c, apperrors.Wrap(apperrors.KindInternal, "{{ .Dir }}_grpc_client_error", "{{ .Dir }} grpc client error", err))
		return
	}
	resp, err := client.Delete{{ .Pascal }}(outgoingContext(c), &{{ .GoPackage }}.Delete{{ .Pascal }}Request{Id: c.Param("id")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	h.log.Info("gateway {{ .Dir }} delete proxied", zap.String("request_id", httpx.RequestID(c)), zap.String("aggregate_id", c.Param("id")))
	httpx.OK(c, resp)
}

func (h *{{ .Pascal }}Handler) grpcClient() ({{ .GoPackage }}.{{ .Pascal }}ServiceClient, error) {
	h.once.Do(func() {
		conn, err := grpc.Dial(h.target, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			h.err = err
			return
		}
		h.conn = conn
		h.client = {{ .GoPackage }}.New{{ .Pascal }}ServiceClient(conn)
		h.log.Info("gateway {{ .Dir }} grpc client initialized", zap.String("target", h.target), zap.String("target_env", {{ .GoIdent }}GatewayTargetEnv))
	})
	return h.client, h.err
}
`

const gatewayRoutesTemplate = `package router

import (
	"github.com/gin-gonic/gin"

	"{{ .Module }}/internal/gateway/handler"
)

// register{{ .Pascal }}Routes registers /api/v1/{{ .TableName }} endpoints in one business-specific file.
func register{{ .Pascal }}Routes(v1 *gin.RouterGroup, {{ .GoIdent }}Handler *handler.{{ .Pascal }}Handler) {
	routes := v1.Group("/{{ .TableName }}")
	routes.POST("", {{ .GoIdent }}Handler.Create)
	routes.GET("", {{ .GoIdent }}Handler.List)
	routes.GET("/:id", {{ .GoIdent }}Handler.Get)
	routes.PUT("/:id", {{ .GoIdent }}Handler.Update)
	routes.DELETE("/:id", {{ .GoIdent }}Handler.Delete)
}
`

const cleanGatewayV1WithServiceTemplate = `package router

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"{{ .Module }}/internal/gateway/handler"
)

// registerAPIRoutes creates the /api/v1 route namespace before delegating by business module.
func registerAPIRoutes(r *gin.Engine, log *zap.Logger) {
	api := r.Group("/api")
	v1 := api.Group("/v1")

	register{{ .Pascal }}Routes(v1, handler.New{{ .Pascal }}Handler(log))
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

## 基础 CRUD

生成后的服务已经提供 Create/Get/List/Update/Delete 的基础调用链：

~~~text
proto RPC -> handler -> service -> model.Repository -> repo(Gorm) -> database
~~~

用户可以直接把示例字段 ` + "`Name`" + `、` + "`Description`" + ` 替换成真实业务字段，或者在此基础上新增业务方法。

如果项目包含 Gin gateway，命令也会生成 HTTP 入口：

~~~text
POST   /api/v1/{{ .TableName }}
GET    /api/v1/{{ .TableName }}
GET    /api/v1/{{ .TableName }}/:id
PUT    /api/v1/{{ .TableName }}/:id
DELETE /api/v1/{{ .TableName }}/:id
~~~

gateway 默认调用 ` + "`{{ .Dir }}-service`" + ` 的 ` + "`127.0.0.1:{{ .Port }}`" + `，无需改配置。如需覆盖目标地址，设置：

~~~bash
export APP_{{ .EnvPrefix }}_GRPC_TARGET=127.0.0.1:{{ .Port }}
~~~

## 开发顺序

1. 在 ` + "`api/proto/{{ .Dir }}/v1/{{ .ProtoFile }}`" + ` 中定义 RPC、Request、Response。
2. 执行 ` + "`make proto`" + ` 生成 ` + "`api/gen/{{ .Dir }}/v1`" + `。
3. 在 ` + "`internal/{{ .Dir }}/model`" + ` 补充领域实体、业务错误和仓储接口。
4. 在 ` + "`internal/{{ .Dir }}/service`" + ` 编写业务用例。
5. 在 ` + "`internal/{{ .Dir }}/repo`" + ` 实现数据库访问。
6. 在 ` + "`internal/{{ .Dir }}/handler`" + ` 将 gRPC 请求转成业务命令。
7. 在 ` + "`internal/gateway/request`" + `、` + "`handler`" + `、` + "`router`" + ` 调整 HTTP 入参、控制器和路由。

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
| ` + "`internal/gateway/request`" + ` | HTTP 入参 DTO | 控制器不堆字段，入参校验更清楚 |
| ` + "`internal/gateway/handler`" + ` | HTTP 控制器 | 只做绑定、调用 gRPC client 和统一响应 |
| ` + "`internal/gateway/router`" + ` | HTTP 路由 | 按版本/业务拆分，避免路由堆在一个文件 |

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
	ID          string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func New{{ .Pascal }}(name string, description string) (*{{ .Pascal }}, error) {
	now := time.Now().UTC()
	return &{{ .Pascal }}{
		ID:          uuid.NewString(),
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
~~~

仓储接口也放在 ` + "`model`" + `：

~~~go
package model

import "context"

type Repository interface {
	Save(ctx context.Context, item *{{ .Pascal }}) error
	FindByID(ctx context.Context, id string) (*{{ .Pascal }}, error)
	List(ctx context.Context, offset int, limit int) ([]*{{ .Pascal }}, int64, error)
	Delete(ctx context.Context, id string) error
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
repo := {{ .GoIdent }}repo.NewGormRepository(db, log)
svc := {{ .GoIdent }}service.NewService(repo, log)
~~~

Gorm 仓储示例：

~~~go
type {{ .Pascal }}Model struct {
	ID          string ` + "`gorm:\"primaryKey;size:64\"`" + `
	Name        string ` + "`gorm:\"size:128;not null\"`" + `
	Description string ` + "`gorm:\"type:text\"`" + `
	CreatedAt   time.Time
	UpdatedAt   time.Time
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

## gateway 层

HTTP 入口由脚手架同步生成：

~~~text
internal/gateway/request/{{ .Dir }}_request.go
internal/gateway/handler/{{ .Dir }}_handler.go
internal/gateway/router/{{ .Dir }}_routes.go
~~~

路由按 ` + "`版本/业务/具体接口`" + ` 拆分，当前业务挂在 ` + "`/api/v1/{{ .TableName }}`" + `。gateway handler 默认使用 ` + "`APP_{{ .EnvPrefix }}_GRPC_TARGET`" + ` 覆盖目标地址，不配置时连接 ` + "`127.0.0.1:{{ .Port }}`" + `。
`
