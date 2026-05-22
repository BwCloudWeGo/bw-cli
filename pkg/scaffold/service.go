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
var legacyGatewayTargetPattern = regexp.MustCompile(`(?s)\nfunc ` + `gateway` + `GRPC` + `Target` + `\(envName string, fallback string\) string \{\s*value := strings\.TrimSpace\(os\.Get` + `env\(envName\)\)\s*if value == "" \{\s*return fallback\s*\}\s*return value\s*\}\s*`)
var legacyServiceTargetFallbackPattern = regexp.MustCompile(`cfg\.ServiceTarget\("([^"]+)",\s*[0-9]+\)`)
var gatewayClientDialPattern = regexp.MustCompile(`(?m)^\t([A-Za-z]\w*)Conn, err := grpc\.Dial`)
var gatewayClientConnsPattern = regexp.MustCompile(`conns:\s+\[\]\*grpc\.ClientConn\{([^}]*)\}`)

// ServiceOptions controls bw-cli service generation inside an existing project.
type ServiceOptions struct {
	RootDir           string
	Name              string
	Port              int
	RunProto          bool
	RunTidy           bool
	NacosSyncRequired bool
	NacosDataID       string
}

type serviceTemplateData struct {
	Module       string
	InputName    string
	Dir          string
	ProtoFile    string
	ProtoPackage string
	GoPackage    string
	GoIdent      string
	HTTPAlias    string
	Pascal       string
	ServiceName  string
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
	port := opts.Port
	if port == 0 {
		port, err = nextServicePort(root)
		if err != nil {
			return err
		}
	}
	data, err := buildServiceTemplateData(module, opts.Name, port)
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
	if err := addServiceConfig(root, data); err != nil {
		return err
	}
	if enabled, dataID := nacosEnabled(root); enabled {
		fmt.Printf("nacos is enabled: sync configs/config.yaml service changes to Nacos data_id %s\n", dataID)
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
		HTTPAlias:    strings.ToLower(pascal) + "http",
		Pascal:       pascal,
		ServiceName:  strings.Join(parts, "-") + "-service",
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
		filepath.Join("api", "proto", data.Dir, "v1", data.ProtoFile):                                  renderServiceTemplate(serviceProtoTemplate, data),
		filepath.Join("cmd", data.Dir, "main.go"):                                                      renderServiceTemplate(serviceMainTemplate, data),
		filepath.Join("internal", data.Dir, "domain", "entity.go"):                                     renderServiceTemplate(serviceModelTemplate, data),
		filepath.Join("internal", data.Dir, "domain", "repository.go"):                                 renderServiceTemplate(serviceRepositoryTemplate, data),
		filepath.Join("internal", data.Dir, "application", "command.go"):                               renderServiceTemplate(serviceCommandTemplate, data),
		filepath.Join("internal", data.Dir, "application", "dto.go"):                                   renderServiceTemplate(serviceDTOTemplate, data),
		filepath.Join("internal", data.Dir, "application", "service.go"):                               renderServiceTemplate(serviceUseCaseTemplate, data),
		filepath.Join("internal", data.Dir, "application", "service_test.go"):                          renderServiceTemplate(serviceUseCaseTestTemplate, data),
		filepath.Join("internal", data.Dir, "infrastructure", "persistence", "gorm", "model.go"):       renderServiceTemplate(serviceGormModelTemplate, data),
		filepath.Join("internal", data.Dir, "infrastructure", "persistence", "gorm", "mapper.go"):      renderServiceTemplate(serviceGormMapperTemplate, data),
		filepath.Join("internal", data.Dir, "infrastructure", "persistence", "gorm", "repository.go"):  renderServiceTemplate(serviceGormRepoTemplate, data),
		filepath.Join("internal", data.Dir, "infrastructure", "persistence", "gorm", "migrate.go"):     renderServiceTemplate(serviceGormMigrateTemplate, data),
		filepath.Join("internal", data.Dir, "infrastructure", "persistence", "mongo", "document.go"):   renderServiceTemplate(serviceMongoDocumentTemplate, data),
		filepath.Join("internal", data.Dir, "infrastructure", "persistence", "mongo", "mapper.go"):     renderServiceTemplate(serviceMongoMapperTemplate, data),
		filepath.Join("internal", data.Dir, "infrastructure", "persistence", "mongo", "repository.go"): renderServiceTemplate(serviceMongoRepoTemplate, data),
		filepath.Join("internal", data.Dir, "interfaces", "rpc", "server.go"):                          renderServiceTemplate(serviceHandlerTemplate, data),
		filepath.Join("docs", "services", data.Dir+".md"):                                              renderServiceTemplate(serviceDocTemplate, data),
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
	contextPath := filepath.Join(root, "internal", "gateway", "svc", "context.go")
	if !exists(contextPath) {
		if err := writeNewFile(contextPath, []byte(renderServiceTemplate(gatewayServiceContextTemplate, data))); err != nil {
			return err
		}
	}
	files := map[string]string{
		filepath.Join("internal", "gateway", "interfaces", "http", data.Dir, "request.go"): renderServiceTemplate(gatewayRequestTemplate, data),
		filepath.Join("internal", "gateway", "interfaces", "http", data.Dir, "handler.go"): renderServiceTemplate(gatewayHandlerTemplate, data),
		filepath.Join("internal", "gateway", "interfaces", "http", data.Dir, "routes.go"):  renderServiceTemplate(gatewayRoutesTemplate, data),
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
	if strings.Contains(text, "func "+"configuredGateway"+"GRPCTarget") {
		return nil
	}
	if legacyGatewayTargetPattern.MatchString(text) {
		text = legacyGatewayTargetPattern.ReplaceAllString(text, "\n")
		text = removeImport(text, "\"os\"")
		return os.WriteFile(path, []byte(text), 0o644)
	}
	return nil
}

func ensureGatewayClientsFile(path string, data serviceTemplateData) error {
	if !exists(path) {
		return writeNewFile(path, []byte(renderServiceTemplate(gatewayClientsTemplate, data)))
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(content)
	text = legacyServiceTargetFallbackPattern.ReplaceAllString(text, `cfg.ServiceTarget("$1")`)
	field := fmt.Sprintf("\t%s  %s.%sServiceClient\n", data.Pascal, data.GoPackage, data.Pascal)
	if strings.Contains(text, field) {
		return nil
	}

	text = ensureImport(text, fmt.Sprintf("%s %q", data.GoPackage, data.Module+"/api/gen/"+data.Dir+"/v1"))
	text = strings.Replace(text, "type Clients struct {\n", "type Clients struct {\n"+field, 1)

	targetLine := fmt.Sprintf("\t%sTarget := cfg.ServiceTarget(%q)\n", data.GoIdent, data.Dir)
	if loc := gatewayClientDialPattern.FindStringIndex(text); loc != nil {
		text = text[:loc[0]] + targetLine + text[loc[0]:]
	}

	existingConnNames := gatewayClientConnNames(text)
	dialBlock := fmt.Sprintf("\t%sConn, err := grpc.Dial(%sTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))\n\tif err != nil {\n%s\t\treturn nil, fmt.Errorf(\"dial %s service: %%w\", err)\n\t}\n", data.GoIdent, data.GoIdent, closeConnLines(existingConnNames), data.Dir)
	if marker := "\n\tlog.Info(\"grpc clients initialized\","; strings.Contains(text, marker) {
		text = strings.Replace(text, marker, "\n"+dialBlock+marker, 1)
	}

	zapLine := fmt.Sprintf("\t\tzap.String(\"%s_target\", %sTarget),", data.Dir, data.GoIdent)
	if marker := "\n\t)\n\treturn &Clients{"; strings.Contains(text, marker) && !strings.Contains(text, zapLine) {
		text = strings.Replace(text, marker, "\n"+zapLine+marker, 1)
	}

	clientLine := fmt.Sprintf("\t\t%s:  %s.New%sServiceClient(%sConn),\n", data.Pascal, data.GoPackage, data.Pascal, data.GoIdent)
	if strings.Contains(text, "\t\tConfig: cfg,\n") {
		text = strings.Replace(text, "\t\tConfig: cfg,\n", clientLine+"\t\tConfig: cfg,\n", 1)
	}
	text = gatewayClientConnsPattern.ReplaceAllStringFunc(text, func(value string) string {
		if strings.Contains(value, data.GoIdent+"Conn") {
			return value
		}
		return strings.TrimRight(value, "}") + ", " + data.GoIdent + "Conn}"
	})
	return os.WriteFile(path, []byte(text), 0o644)
}

func gatewayClientConnNames(text string) []string {
	matches := gatewayClientDialPattern.FindAllStringSubmatch(text, -1)
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, match[1]+"Conn")
	}
	return names
}

func closeConnLines(connNames []string) string {
	var b strings.Builder
	for _, name := range connNames {
		fmt.Fprintf(&b, "\t\t%s.Close()\n", name)
	}
	return b.String()
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

func removeImport(text string, quotedPackage string) string {
	lines := strings.Split(text, "\n")
	out := lines[:0]
	for _, line := range lines {
		if strings.TrimSpace(line) == quotedPackage {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
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

func nextServicePort(root string) (int, error) {
	path := filepath.Join(root, "configs", "config.yaml")
	if !exists(path) {
		return defaultServicePort, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	maxPort := defaultServicePort - 1
	portPattern := regexp.MustCompile(`(?m)^\s+port:\s*([0-9]+)\s*$`)
	for _, match := range portPattern.FindAllStringSubmatch(string(data), -1) {
		var port int
		if _, err := fmt.Sscanf(match[1], "%d", &port); err == nil && port > maxPort {
			maxPort = port
		}
	}
	if maxPort < defaultServicePort {
		return defaultServicePort, nil
	}
	return maxPort + 1, nil
}

func addServiceConfig(root string, data serviceTemplateData) error {
	path := filepath.Join(root, "configs", "config.yaml")
	if !exists(path) {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(content)
	serviceBlock := fmt.Sprintf(`  %s:
    name: %s
    port: %d
    target: 127.0.0.1:%d
`, data.Dir, data.ServiceName, data.Port, data.Port)
	if regexp.MustCompile(`(?m)^\s{2}` + regexp.QuoteMeta(data.Dir) + `:\s*$`).MatchString(text) {
		return nil
	}
	if !regexp.MustCompile(`(?m)^services:\s*$`).MatchString(text) {
		insert := "\nservices:\n" + serviceBlock
		if idx := strings.Index(text, "\ndatabase:"); idx >= 0 {
			text = text[:idx] + insert + text[idx:]
		} else {
			text = strings.TrimRight(text, "\n") + insert + "\n"
		}
		return os.WriteFile(path, []byte(text), 0o644)
	}
	lines := strings.Split(text, "\n")
	insertAt := len(lines)
	for i, line := range lines {
		if strings.TrimSpace(line) == "services:" {
			insertAt = i + 1
			continue
		}
		if insertAt != len(lines) && line != "" && !strings.HasPrefix(line, " ") && strings.HasSuffix(line, ":") {
			insertAt = i
			break
		}
	}
	blockLines := strings.Split(strings.TrimRight(serviceBlock, "\n"), "\n")
	lines = append(lines[:insertAt], append(blockLines, lines[insertAt:]...)...)
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func nacosEnabled(root string) (bool, string) {
	path := filepath.Join(root, "configs", "nacos.yaml")
	if !exists(path) {
		return false, "xiaolanshu.yaml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, "xiaolanshu.yaml"
	}
	text := string(data)
	enabled := regexp.MustCompile(`(?m)^enabled:\s*true\s*$`).MatchString(text)
	dataID := "xiaolanshu.yaml"
	if match := regexp.MustCompile(`(?m)^data_id:\s*"?([^"\n]+)"?\s*$`).FindStringSubmatch(text); len(match) == 2 {
		dataID = strings.TrimSpace(match[1])
	}
	return enabled, dataID
}

func gofmtService(root string, serviceDir string) error {
	args := []string{
		filepath.Join("cmd", serviceDir, "main.go"),
		filepath.Join("internal", serviceDir, "domain", "entity.go"),
		filepath.Join("internal", serviceDir, "domain", "repository.go"),
		filepath.Join("internal", serviceDir, "application", "command.go"),
		filepath.Join("internal", serviceDir, "application", "dto.go"),
		filepath.Join("internal", serviceDir, "application", "service.go"),
		filepath.Join("internal", serviceDir, "application", "service_test.go"),
		filepath.Join("internal", serviceDir, "infrastructure", "persistence", "gorm", "model.go"),
		filepath.Join("internal", serviceDir, "infrastructure", "persistence", "gorm", "mapper.go"),
		filepath.Join("internal", serviceDir, "infrastructure", "persistence", "gorm", "repository.go"),
		filepath.Join("internal", serviceDir, "infrastructure", "persistence", "gorm", "migrate.go"),
		filepath.Join("internal", serviceDir, "infrastructure", "persistence", "mongo", "document.go"),
		filepath.Join("internal", serviceDir, "infrastructure", "persistence", "mongo", "mapper.go"),
		filepath.Join("internal", serviceDir, "infrastructure", "persistence", "mongo", "repository.go"),
		filepath.Join("internal", serviceDir, "interfaces", "rpc", "server.go"),
	}
	for _, rel := range []string{
		filepath.Join("internal", "gateway", "svc", "context.go"),
		filepath.Join("internal", "gateway", "interfaces", "http", serviceDir, "request.go"),
		filepath.Join("internal", "gateway", "interfaces", "http", serviceDir, "handler.go"),
		filepath.Join("internal", "gateway", "interfaces", "http", serviceDir, "routes.go"),
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
		routerText = ensureImport(routerText, fmt.Sprintf("%q", data.Module+"/internal/gateway/svc"))
		routerText = removeImport(routerText, fmt.Sprintf("%q", data.Module+"/internal/gateway/client"))
		if strings.Contains(routerText, "func New(clients *client.Clients, log *zap.Logger, middlewareCfg config.MiddlewareConfig) *gin.Engine") {
			routerText = strings.Replace(routerText, "func New(clients *client.Clients, log *zap.Logger, middlewareCfg config.MiddlewareConfig) *gin.Engine", "func New(log *zap.Logger, middlewareCfg config.MiddlewareConfig) *gin.Engine", 1)
		}
		if strings.Contains(routerText, "registerAPIRoutes(r)") {
			routerText = strings.Replace(routerText, "registerAPIRoutes(r)", "ctx := svc.NewServiceContext(log)\n\tregisterAPIRoutes(r, ctx)", 1)
		}
		if strings.Contains(routerText, "registerAPIRoutes(r, clients, log)") {
			routerText = strings.Replace(routerText, "registerAPIRoutes(r, clients, log)", "ctx := svc.NewServiceContext(log)\n\tregisterAPIRoutes(r, ctx)", 1)
		}
		if err := os.WriteFile(routerPath, []byte(routerText), 0o644); err != nil {
			return err
		}
	}
	if err := patchGatewayMain(root, data); err != nil {
		return err
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
	registration := fmt.Sprintf("%s.RegisterRoutes(v1, ctx)", data.HTTPAlias)
	if strings.Contains(v1Text, registration) {
		return nil
	}
	if strings.Contains(v1Text, "func registerAPIRoutes(r *gin.Engine)") {
		return os.WriteFile(v1Path, []byte(renderServiceTemplate(cleanGatewayV1WithServiceTemplate, data)), 0o644)
	}
	if !strings.Contains(v1Text, "func registerAPIRoutes(r *gin.Engine, ctx *svc.ServiceContext)") {
		return nil
	}
	v1Text = ensureImport(v1Text, fmt.Sprintf("%s %q", data.HTTPAlias, data.Module+"/internal/gateway/interfaces/http/"+data.Dir))
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

func patchGatewayMain(root string, data serviceTemplateData) error {
	mainPath := filepath.Join(root, "cmd", "gateway", "main.go")
	if !exists(mainPath) {
		return nil
	}
	content, err := os.ReadFile(mainPath)
	if err != nil {
		return err
	}
	text := string(content)
	text = removeImport(text, fmt.Sprintf("%q", data.Module+"/internal/gateway/client"))
	text = strings.Replace(text, "engine := router.New(gatewayClients, log, cfg.Middleware)", "engine := router.New(log, cfg.Middleware)", 1)
	return os.WriteFile(mainPath, []byte(text), 0o644)
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
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	{{ .GoPackage }} "{{ .Module }}/api/gen/{{ .Dir }}/v1"
	{{ .GoIdent }}app "{{ .Module }}/internal/{{ .Dir }}/application"
	{{ .GoIdent }}gorm "{{ .Module }}/internal/{{ .Dir }}/infrastructure/persistence/gorm"
	{{ .GoIdent }}rpc "{{ .Module }}/internal/{{ .Dir }}/interfaces/rpc"
	"{{ .Module }}/pkg/config"
	"{{ .Module }}/pkg/database"
	"{{ .Module }}/pkg/grpcx"
	"{{ .Module }}/pkg/logger"
)

const serviceName = "{{ .ServiceName }}"
const defaultGRPCPort = {{ .Port }}

func main() {
	if err := config.InitGlobal("configs/config.yaml"); err != nil {
		panic(err)
	}
	cfg := config.MustGlobal()
	cfg.Log.Service = cfg.ServiceName("{{ .Dir }}")
	cfg.Log = logger.WithDailyFileName(cfg.Log, time.Now())

	log, err := logger.New(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer log.Sync()
	config.PrintSourceNotice(cfg, os.Stdout)

	db, err := database.Open(cfg.Database, cfg.MySQL, cfg.PostgreSQL, log)
	if err != nil {
		log.Fatal("open database failed", zap.Error(err))
	}
	if err := {{ .GoIdent }}gorm.AutoMigrate(db); err != nil {
		log.Fatal("migrate {{ .Dir }} database failed", zap.Error(err))
	}

	repo := {{ .GoIdent }}gorm.NewRepository(db, log)
	svc := {{ .GoIdent }}app.NewService(repo, log)
	server := grpc.NewServer(grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor(log)))
	{{ .GoPackage }}.Register{{ .Pascal }}ServiceServer(server, {{ .GoIdent }}rpc.NewServer(svc, log))

	port := cfg.ServicePort("{{ .Dir }}", defaultGRPCPort)
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
	fmt.Fprintf(os.Stdout, "  config: services.{{ .Dir }}.port\n\n")
}

func shutdownOnSignal(server *grpc.Server, log *zap.Logger) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Info("service shutting down", zap.String("service", serviceName))
	server.GracefulStop()
}
`

const serviceModelTemplate = `package domain

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

const serviceRepositoryTemplate = `package domain

import "context"

// Repository defines persistence behavior required by the {{ .Dir }} service layer.
type Repository interface {
	Save(ctx context.Context, item *{{ .Pascal }}) error
	FindByID(ctx context.Context, id string) (*{{ .Pascal }}, error)
	List(ctx context.Context, offset int, limit int) ([]*{{ .Pascal }}, int64, error)
	Delete(ctx context.Context, id string) error
}
`

const serviceCommandTemplate = `package application

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
`

const serviceDTOTemplate = `package application

import (
	"time"

	"{{ .Module }}/internal/{{ .Dir }}/domain"
)

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

// From{{ .Pascal }} converts a {{ .Dir }} aggregate into the service response DTO.
func From{{ .Pascal }}(item *domain.{{ .Pascal }}) *{{ .Pascal }}DTO {
	return &{{ .Pascal }}DTO{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   formatTime(item.CreatedAt),
		UpdatedAt:   formatTime(item.UpdatedAt),
	}
}

// formatTime keeps zero time empty and serializes real values in a stable API format.
func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}
`

const serviceUseCaseTemplate = `package application

import (
	"context"

	"go.uber.org/zap"

	"{{ .Module }}/internal/{{ .Dir }}/domain"
)

// Service orchestrates {{ .Dir }} use cases.
type Service struct {
	repo domain.Repository
	log  *zap.Logger
}

// NewService constructs the {{ .Dir }} use-case service.
func NewService(repo domain.Repository, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	return &Service{repo: repo, log: log}
}

// Create creates a {{ .Dir }} record.
func (s *Service) Create(ctx context.Context, cmd CreateCommand) (*{{ .Pascal }}DTO, error) {
	item, err := domain.New{{ .Pascal }}(cmd.Name, cmd.Description)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, item); err != nil {
		return nil, err
	}
	s.log.Info("{{ .Dir }} created", zap.String("aggregate_id", item.ID), zap.String("use_case", "Create{{ .Pascal }}"))
	return From{{ .Pascal }}(item), nil
}

// Get returns one {{ .Dir }} record by id.
func (s *Service) Get(ctx context.Context, id string) (*{{ .Pascal }}DTO, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return From{{ .Pascal }}(item), nil
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
		output.Items = append(output.Items, From{{ .Pascal }}(item))
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
	return From{{ .Pascal }}(item), nil
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
`

const serviceUseCaseTestTemplate = `package application

import (
	"context"
	"testing"

	"{{ .Module }}/internal/{{ .Dir }}/domain"
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
	require.ErrorIs(t, err, domain.Err{{ .Pascal }}NotFound)
}

type fakeRepository struct {
	items map[string]*domain.{{ .Pascal }}
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{items: make(map[string]*domain.{{ .Pascal }})}
}

func (r *fakeRepository) Save(ctx context.Context, item *domain.{{ .Pascal }}) error {
	copy := *item
	r.items[item.ID] = &copy
	return nil
}

func (r *fakeRepository) FindByID(ctx context.Context, id string) (*domain.{{ .Pascal }}, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, domain.Err{{ .Pascal }}NotFound
	}
	copy := *item
	return &copy, nil
}

func (r *fakeRepository) List(ctx context.Context, offset int, limit int) ([]*domain.{{ .Pascal }}, int64, error) {
	items := make([]*domain.{{ .Pascal }}, 0, len(r.items))
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
		return domain.Err{{ .Pascal }}NotFound
	}
	delete(r.items, id)
	return nil
}

var _ domain.Repository = (*fakeRepository)(nil)
`

const serviceGormModelTemplate = `package gorm

import "time"

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
`

const serviceGormMapperTemplate = `package gorm

import "{{ .Module }}/internal/{{ .Dir }}/domain"

func toRecord(item *domain.{{ .Pascal }}) *{{ .Pascal }}Model {
	return &{{ .Pascal }}Model{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func toDomain(record *{{ .Pascal }}Model) *domain.{{ .Pascal }} {
	return &domain.{{ .Pascal }}{
		ID:          record.ID,
		Name:        record.Name,
		Description: record.Description,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}
`

const serviceGormMigrateTemplate = `package gorm

import "gorm.io/gorm"

// AutoMigrate creates or updates the {{ .TableName }} table schema.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&{{ .Pascal }}Model{})
}
`

const serviceGormRepoTemplate = `package gorm

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"{{ .Module }}/internal/{{ .Dir }}/domain"
)

// Repository persists {{ .Dir }} aggregates with Gorm.
type Repository struct {
	db  *gorm.DB
	log *zap.Logger
}

// NewRepository constructs a {{ .Dir }} Gorm repository with optional structured logging.
func NewRepository(db *gorm.DB, loggers ...*zap.Logger) *Repository {
	log := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}
	return &Repository{db: db, log: log}
}

// Save inserts or updates a {{ .Dir }} aggregate.
func (r *Repository) Save(ctx context.Context, item *domain.{{ .Pascal }}) error {
	start := time.Now()
	tx := r.db.WithContext(ctx).Save(toRecord(item))
	r.logOperation("Save", tx.RowsAffected, start, tx.Error)
	return tx.Error
}

// FindByID loads a {{ .Dir }} aggregate by id.
func (r *Repository) FindByID(ctx context.Context, id string) (*domain.{{ .Pascal }}, error) {
	start := time.Now()
	var record {{ .Pascal }}Model
	tx := r.db.WithContext(ctx).Where("id = ?", id).First(&record)
	err := tx.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = domain.Err{{ .Pascal }}NotFound
	}
	if err != nil {
		r.logOperation("FindByID", tx.RowsAffected, start, err)
		return nil, err
	}
	r.logOperation("FindByID", tx.RowsAffected, start, nil)
	return toDomain(&record), nil
}

// List loads paginated {{ .Dir }} aggregates.
func (r *Repository) List(ctx context.Context, offset int, limit int) ([]*domain.{{ .Pascal }}, int64, error) {
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
	items := make([]*domain.{{ .Pascal }}, 0, len(records))
	for i := range records {
		items = append(items, toDomain(&records[i]))
	}
	r.logOperation("List", tx.RowsAffected, start, nil)
	return items, total, nil
}

// Delete removes a {{ .Dir }} aggregate by id.
func (r *Repository) Delete(ctx context.Context, id string) error {
	start := time.Now()
	tx := r.db.WithContext(ctx).Where("id = ?", id).Delete(&{{ .Pascal }}Model{})
	err := tx.Error
	if err == nil && tx.RowsAffected == 0 {
		err = domain.Err{{ .Pascal }}NotFound
	}
	r.logOperation("Delete", tx.RowsAffected, start, err)
	return err
}

func (r *Repository) logOperation(operation string, rows int64, start time.Time, err error) {
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

var _ domain.Repository = (*Repository)(nil)
`

const serviceMongoDocumentTemplate = `package mongo

import "time"

const {{ .GoIdent }}MongoCollectionName = "{{ .TableName }}"

// {{ .Pascal }}Document is the MongoDB document for the {{ .Dir }} aggregate.
type {{ .Pascal }}Document struct {
	ID          string    ` + "`bson:\"_id\"`" + `
	Name        string    ` + "`bson:\"name\"`" + `
	Description string    ` + "`bson:\"description\"`" + `
	CreatedAt   time.Time ` + "`bson:\"created_at\"`" + `
	UpdatedAt   time.Time ` + "`bson:\"updated_at\"`" + `
}

// MongoCollectionName declares the MongoDB collection for {{ .Pascal }}Document.
func ({{ .Pascal }}Document) MongoCollectionName() string {
	return {{ .GoIdent }}MongoCollectionName
}
`

const serviceMongoMapperTemplate = `package mongo

import "{{ .Module }}/internal/{{ .Dir }}/domain"

func toDocument(item *domain.{{ .Pascal }}) *{{ .Pascal }}Document {
	return &{{ .Pascal }}Document{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func toDomainFromDocument(document *{{ .Pascal }}Document) *domain.{{ .Pascal }} {
	return &domain.{{ .Pascal }}{
		ID:          document.ID,
		Name:        document.Name,
		Description: document.Description,
		CreatedAt:   document.CreatedAt,
		UpdatedAt:   document.UpdatedAt,
	}
}
`

const serviceMongoRepoTemplate = `package mongo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"

	"{{ .Module }}/internal/{{ .Dir }}/domain"
	"{{ .Module }}/pkg/mongox"
)

// Repository persists {{ .Dir }} aggregates with the shared mongox DocumentStore.
type Repository struct {
	documents *mongox.DocumentStore[{{ .Pascal }}Document]
	log       *zap.Logger
}

// NewRepository constructs a MongoDB repository using the configured database.
func NewRepository(db *mongo.Database, loggers ...*zap.Logger) *Repository {
	log := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}
	return &Repository{
		documents: mongox.NewDocumentStore[{{ .Pascal }}Document](db, log),
		log:       log,
	}
}

// Save inserts or updates a {{ .Dir }} aggregate by MongoDB _id.
func (r *Repository) Save(ctx context.Context, item *domain.{{ .Pascal }}) error {
	start := time.Now()
	_, err := r.documents.UpsertByID(ctx, item.ID, toDocument(item))
	r.logOperation("Save", item.ID, 0, start, err)
	return err
}

// FindByID loads a {{ .Dir }} aggregate by MongoDB _id.
func (r *Repository) FindByID(ctx context.Context, id string) (*domain.{{ .Pascal }}, error) {
	start := time.Now()
	document, err := r.documents.FindByID(ctx, id)
	if errors.Is(err, mongox.ErrNotFound) {
		err = domain.Err{{ .Pascal }}NotFound
	}
	r.logOperation("FindByID", id, 0, start, err)
	if err != nil {
		return nil, err
	}
	return toDomainFromDocument(document), nil
}

// List loads paginated {{ .Dir }} aggregates ordered by creation time.
func (r *Repository) List(ctx context.Context, offset int, limit int) ([]*domain.{{ .Pascal }}, int64, error) {
	start := time.Now()
	filter := bson.M{}
	total, err := r.documents.Count(ctx, filter)
	if err != nil {
		r.logOperation("Count", "", 0, start, err)
		return nil, 0, err
	}

	documents, err := r.documents.FindMany(ctx, filter,
		options.Find().
			SetSort(bson.D{{ "{{" }}Key: "created_at", Value: -1{{ "}}" }}).
			SetSkip(int64(offset)).
			SetLimit(int64(limit)),
	)
	if err != nil {
		r.logOperation("List", "", total, start, err)
		return nil, 0, err
	}

	items := make([]*domain.{{ .Pascal }}, 0, len(documents))
	for i := range documents {
		items = append(items, toDomainFromDocument(&documents[i]))
	}
	r.logOperation("List", "", total, start, nil)
	return items, total, nil
}

// Delete removes a {{ .Dir }} aggregate by MongoDB _id.
func (r *Repository) Delete(ctx context.Context, id string) error {
	start := time.Now()
	result, err := r.documents.DeleteByID(ctx, id)
	if err == nil && result != nil && result.DeletedCount == 0 {
		err = domain.Err{{ .Pascal }}NotFound
	}
	r.logOperation("Delete", id, 0, start, err)
	return err
}

func (r *Repository) logOperation(operation string, id string, total int64, start time.Time, err error) {
	fields := []zap.Field{
		zap.String("repository", "{{ .Dir }}_mongo"),
		zap.String("operation", operation),
		zap.String("aggregate_id", id),
		zap.Int64("total", total),
		zap.Float64("latency_ms", float64(time.Since(start).Microseconds())/1000),
	}
	if err != nil {
		fields = append(fields, zap.Error(err))
		r.log.Warn("mongodb repository operation completed with error", fields...)
		return
	}
	r.log.Info("mongodb repository operation completed", fields...)
}

var _ domain.Repository = (*Repository)(nil)
`

const serviceHandlerTemplate = `package rpc

import (
	"context"
	stderrors "errors"

	"go.uber.org/zap"

	{{ .GoPackage }} "{{ .Module }}/api/gen/{{ .Dir }}/v1"
	"{{ .Module }}/internal/{{ .Dir }}/application"
	"{{ .Module }}/internal/{{ .Dir }}/domain"
	apperrors "{{ .Module }}/pkg/errors"
)

// Server adapts {{ .Dir }} gRPC requests to application use cases.
type Server struct {
	{{ .GoPackage }}.Unimplemented{{ .Pascal }}ServiceServer
	svc *application.Service
	log *zap.Logger
}

// NewServer constructs the {{ .Dir }} gRPC server adapter.
func NewServer(svc *application.Service, log *zap.Logger) *Server {
	if log == nil {
		log = zap.NewNop()
	}
	return &Server{svc: svc, log: log}
}

// Create{{ .Pascal }} handles the create RPC.
func (s *Server) Create{{ .Pascal }}(ctx context.Context, req *{{ .GoPackage }}.Create{{ .Pascal }}Request) (*{{ .GoPackage }}.{{ .Pascal }}Response, error) {
	item, err := s.svc.Create(ctx, application.CreateCommand{
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
	list, err := s.svc.List(ctx, application.ListCommand{
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
	item, err := s.svc.Update(ctx, application.UpdateCommand{
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

func toProto(item *application.{{ .Pascal }}DTO) *{{ .GoPackage }}.{{ .Pascal }}Response {
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
	case stderrors.Is(err, domain.ErrInvalid{{ .Pascal }}):
		return apperrors.InvalidArgument("invalid_{{ .Dir }}", "invalid {{ .Dir }} input")
	case stderrors.Is(err, domain.Err{{ .Pascal }}NotFound):
		return apperrors.NotFound("{{ .Dir }}_not_found", "{{ .Dir }} not found")
	default:
		return apperrors.Wrap(apperrors.KindInternal, "{{ .Dir }}_service_error", "{{ .Dir }} service error", err)
	}
}
`
const gatewayClientsTemplate = `package client
`

const gatewayCommonTemplate = `package handler
`

const gatewayServiceContextTemplate = `package svc

import (
	"strings"

	"go.uber.org/zap"

	"{{ .Module }}/pkg/config"
)

// ServiceContext groups shared gateway dependencies, similar to go-zero svc context.
type ServiceContext struct {
	Config *config.Config
	Log    *zap.Logger
}

// NewServiceContext constructs gateway dependencies shared by HTTP interface modules.
func NewServiceContext(log *zap.Logger) *ServiceContext {
	if log == nil {
		log = zap.NewNop()
	}
	return &ServiceContext{Config: config.MustGlobal(), Log: log}
}

// GRPCTarget returns services.<name>.target or a conservative fallback.
func (c *ServiceContext) GRPCTarget(serviceName string, fallback string) string {
	if c != nil && c.Config != nil {
		if target := strings.TrimSpace(c.Config.ServiceTarget(serviceName)); target != "" {
			return target
		}
	}
	return fallback
}
`

const gatewayRequestTemplate = `package {{ .Dir }}

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

const gatewayHandlerTemplate = `package {{ .Dir }}

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	{{ .GoPackage }} "{{ .Module }}/api/gen/{{ .Dir }}/v1"
	"{{ .Module }}/internal/gateway/svc"
	apperrors "{{ .Module }}/pkg/errors"
	"{{ .Module }}/pkg/grpcx"
	"{{ .Module }}/pkg/httpx"
)

const {{ .GoIdent }}DefaultGRPCTarget = "127.0.0.1:{{ .Port }}"

// {{ .Pascal }}Handler adapts {{ .Dir }} HTTP endpoints to the generated gRPC client.
type {{ .Pascal }}Handler struct {
	ctx    *svc.ServiceContext
	client {{ .GoPackage }}.{{ .Pascal }}ServiceClient
	conn   *grpc.ClientConn
}

// New{{ .Pascal }}Handler dials the {{ .Dir }} service and wires HTTP handler methods.
func New{{ .Pascal }}Handler(ctx *svc.ServiceContext) (*{{ .Pascal }}Handler, error) {
	if ctx == nil {
		return nil, fmt.Errorf("gateway service context is nil")
	}
	target := ctx.GRPCTarget("{{ .Dir }}", {{ .GoIdent }}DefaultGRPCTarget)
	conn, err := grpc.Dial(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial {{ .Dir }} service: %w", err)
	}
	ctx.Log.Info("gateway grpc client initialized", zap.String("service", "{{ .Dir }}"), zap.String("target", target))
	return &{{ .Pascal }}Handler{
		ctx:    ctx,
		client: {{ .GoPackage }}.New{{ .Pascal }}ServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close releases the handler-owned gRPC connection.
func (h *{{ .Pascal }}Handler) Close() error {
	if h == nil || h.conn == nil {
		return nil
	}
	return h.conn.Close()
}

// Create proxies POST /api/v1/{{ .TableName }} to Create{{ .Pascal }}.
func (h *{{ .Pascal }}Handler) Create(c *gin.Context) {
	var req Create{{ .Pascal }}Request
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	resp, err := h.client.Create{{ .Pascal }}(outgoingContext(c), &{{ .GoPackage }}.Create{{ .Pascal }}Request{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	h.ctx.Log.Info("gateway {{ .Dir }} create proxied", zap.String("request_id", httpx.RequestID(c)), zap.String("aggregate_id", resp.GetId()))
	httpx.Created(c, resp)
}

// Get proxies GET /api/v1/{{ .TableName }}/:id to Get{{ .Pascal }}.
func (h *{{ .Pascal }}Handler) Get(c *gin.Context) {
	resp, err := h.client.Get{{ .Pascal }}(outgoingContext(c), &{{ .GoPackage }}.Get{{ .Pascal }}Request{Id: c.Param("id")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// List proxies GET /api/v1/{{ .TableName }} to List{{ .Pascal }}s.
func (h *{{ .Pascal }}Handler) List(c *gin.Context) {
	var req List{{ .Pascal }}Request
	if err := c.ShouldBindQuery(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	resp, err := h.client.List{{ .Pascal }}s(outgoingContext(c), &{{ .GoPackage }}.List{{ .Pascal }}sRequest{
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
	var req Update{{ .Pascal }}Request
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	resp, err := h.client.Update{{ .Pascal }}(outgoingContext(c), &{{ .GoPackage }}.Update{{ .Pascal }}Request{
		Id:          c.Param("id"),
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	h.ctx.Log.Info("gateway {{ .Dir }} update proxied", zap.String("request_id", httpx.RequestID(c)), zap.String("aggregate_id", resp.GetId()))
	httpx.OK(c, resp)
}

// Delete proxies DELETE /api/v1/{{ .TableName }}/:id to Delete{{ .Pascal }}.
func (h *{{ .Pascal }}Handler) Delete(c *gin.Context) {
	resp, err := h.client.Delete{{ .Pascal }}(outgoingContext(c), &{{ .GoPackage }}.Delete{{ .Pascal }}Request{Id: c.Param("id")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	h.ctx.Log.Info("gateway {{ .Dir }} delete proxied", zap.String("request_id", httpx.RequestID(c)), zap.String("aggregate_id", c.Param("id")))
	httpx.OK(c, resp)
}

func outgoingContext(c *gin.Context) context.Context {
	return metadata.AppendToOutgoingContext(c.Request.Context(), grpcx.MetadataRequestID, httpx.RequestID(c))
}
`

const gatewayRoutesTemplate = `package {{ .Dir }}

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"{{ .Module }}/internal/gateway/svc"
)

// RegisterRoutes registers /api/v1/{{ .TableName }} endpoints in the {{ .Dir }} HTTP module.
func RegisterRoutes(v1 *gin.RouterGroup, ctx *svc.ServiceContext) {
	handler, err := New{{ .Pascal }}Handler(ctx)
	if err != nil {
		ctx.Log.Fatal("initialize {{ .Dir }} gateway handler failed", zap.Error(err))
	}
	routes := v1.Group("/{{ .TableName }}")
	routes.POST("", handler.Create)
	routes.GET("", handler.List)
	routes.GET("/:id", handler.Get)
	routes.PUT("/:id", handler.Update)
	routes.DELETE("/:id", handler.Delete)
}
`

const cleanGatewayV1WithServiceTemplate = `package router

import (
	"github.com/gin-gonic/gin"

	{{ .HTTPAlias }} "{{ .Module }}/internal/gateway/interfaces/http/{{ .Dir }}"
	"{{ .Module }}/internal/gateway/svc"
)

// registerAPIRoutes creates the /api/v1 route namespace before delegating by business module.
func registerAPIRoutes(r *gin.Engine, ctx *svc.ServiceContext) {
	api := r.Group("/api")
	v1 := api.Group("/v1")

	{{ .HTTPAlias }}.RegisterRoutes(v1, ctx)
}
`

const serviceDocTemplate = `# {{ .Pascal }} 服务开发说明

本服务由以下命令生成：

~~~bash
bw-cli service {{ .InputName }} --port {{ .Port }}
~~~

## 目录结构

~~~text
api/proto/{{ .Dir }}/v1/{{ .ProtoFile }}                         # gRPC 协议定义
api/gen/{{ .Dir }}/v1                                            # make proto 生成代码
cmd/{{ .Dir }}/main.go                                           # 服务启动与依赖装配
internal/{{ .Dir }}/domain/entity.go                             # 领域实体和值对象
internal/{{ .Dir }}/domain/repository.go                         # 仓储接口
internal/{{ .Dir }}/application/command.go                       # 用例入参命令
internal/{{ .Dir }}/application/dto.go                           # 用例出参 DTO 和转换
internal/{{ .Dir }}/application/service.go                       # 用例编排
internal/{{ .Dir }}/infrastructure/persistence/gorm              # 表结构、mapper、仓储、迁移
internal/{{ .Dir }}/infrastructure/persistence/mongo             # 文档结构、mapper、仓储
internal/{{ .Dir }}/interfaces/rpc/server.go                     # gRPC 入站适配
internal/gateway/svc/context.go                                  # gateway 依赖聚合
internal/gateway/interfaces/http/{{ .Dir }}                      # HTTP request/handler/routes
~~~

## 启动

~~~bash
make proto
make run-{{ .Dir }}
~~~

默认端口是 ` + "`{{ .Port }}`" + `。命令会自动写入 ` + "`configs/config.yaml`" + `：

~~~yaml
services:
  {{ .Dir }}:
    name: {{ .ServiceName }}
    port: {{ .Port }}
    target: 127.0.0.1:{{ .Port }}
~~~

## 基础 CRUD

生成后的服务已经提供 Create/Get/List/Update/Delete 的基础调用链：

~~~text
proto RPC -> interfaces/rpc -> application.Service -> domain.Repository -> infrastructure/gorm -> database
~~~

默认启动使用 ` + "`infrastructure/persistence/gorm`" + `。命令同时生成 ` + "`infrastructure/persistence/mongo`" + `，MongoDB 仓储已通过 ` + "`mongox.NewDocumentStore[{{ .Pascal }}Document]`" + ` 接好基础 CRUD；需要切换 MongoDB 时，只替换 ` + "`cmd/{{ .Dir }}/main.go`" + ` 中注入的 repository。

如果项目包含 Gin gateway，命令也会生成 HTTP 入口：

~~~text
POST   /api/v1/{{ .TableName }}
GET    /api/v1/{{ .TableName }}
GET    /api/v1/{{ .TableName }}/:id
PUT    /api/v1/{{ .TableName }}/:id
DELETE /api/v1/{{ .TableName }}/:id
~~~

gateway 默认读取 ` + "`services.{{ .Dir }}.target`" + `。如需调整地址，修改 ` + "`configs/config.yaml`" + ` 中的 ` + "`services.{{ .Dir }}.target`" + `。

## 开发顺序

1. 在 ` + "`api/proto/{{ .Dir }}/v1/{{ .ProtoFile }}`" + ` 中定义 RPC、Request、Response。
2. 执行 ` + "`make proto`" + ` 生成 ` + "`api/gen/{{ .Dir }}/v1`" + `。
3. 在 ` + "`internal/{{ .Dir }}/domain`" + ` 补充领域实体、业务错误和仓储接口。
4. 在 ` + "`internal/{{ .Dir }}/application`" + ` 写命令、DTO 和用例编排。
5. 在 ` + "`internal/{{ .Dir }}/infrastructure/persistence`" + ` 实现数据库访问。
6. 在 ` + "`internal/{{ .Dir }}/interfaces/rpc`" + ` 将 gRPC 请求转成业务命令。
7. 在 ` + "`internal/gateway/interfaces/http/{{ .Dir }}`" + ` 调整 HTTP 入参、控制器和路由。

## 分层约束

| 层级 | 写什么 | 为什么 |
| --- | --- | --- |
| ` + "`domain`" + ` | 领域实体、业务错误、Repository 接口 | 业务核心不依赖 Gin、gRPC、Gorm |
| ` + "`application`" + ` | Command、DTO、Service | 用例编排独立，方便单测 |
| ` + "`infrastructure/persistence/gorm`" + ` | Gorm Model、mapper、repository、AutoMigrate | 表结构和数据库操作集中管理 |
| ` + "`infrastructure/persistence/mongo`" + ` | Mongo Document、mapper、repository | 文档结构不污染领域模型 |
| ` + "`interfaces/rpc`" + ` | proto request/response 适配 | 协议转换和错误映射，不写数据库逻辑 |
| ` + "`internal/gateway/interfaces/http/{{ .Dir }}`" + ` | HTTP request、handler、routes | HTTP 层只处理 Web 协议和 gRPC 调用 |

数据库操作规则：

- ` + "`domain`" + ` 不写 Gorm tag。
- ` + "`application`" + ` 不直接使用 ` + "`*gorm.DB`" + `、Mongo collection 或外部 SDK。
- 查询、分页、事务、锁、索引相关实现都放在 ` + "`infrastructure/persistence`" + `。
- 多数据源时保持接口不变，例如 Gorm 和 Mongo repository 都实现 ` + "`domain.Repository`" + `。
`
