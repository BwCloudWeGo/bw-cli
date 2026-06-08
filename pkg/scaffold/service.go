package scaffold

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"unicode"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/BwCloudWeGo/bw-cli/pkg/config"
	"github.com/BwCloudWeGo/bw-cli/pkg/database"
)

const defaultServicePort = 9100

var serviceNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
var tableNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var legacyGatewayTargetPattern = regexp.MustCompile(`(?s)\nfunc ` + `gateway` + `GRPC` + `Target` + `\(envName string, fallback string\) string \{\s*value := strings\.TrimSpace\(os\.Get` + `env\(envName\)\)\s*if value == "" \{\s*return fallback\s*\}\s*return value\s*\}\s*`)
var legacyServiceTargetFallbackPattern = regexp.MustCompile(`cfg\.ServiceTarget\("([^"]+)",\s*[0-9]+\)`)
var gatewayClientDialPattern = regexp.MustCompile(`(?m)^\t([A-Za-z]\w*)Conn, err := grpc\.Dial`)
var gatewayClientConnsPattern = regexp.MustCompile(`conns:\s+\[\]\*grpc\.ClientConn\{([^}]*)\}`)

// ServiceOptions 控制在已有项目内生成 bw-cli 服务。
type ServiceOptions struct {
	RootDir           string
	Name              string
	Port              int
	TableName         string
	SchemaName        string
	RunProto          bool
	RunTidy           bool
	NacosSyncRequired bool
	NacosDataID       string
}

// DeleteServiceOptions 控制从项目中删除已生成服务文件。
type DeleteServiceOptions struct {
	RootDir  string
	Name     string
	RunProto bool
	RunTidy  bool
}

type tableColumn struct {
	Name       string
	PrimaryKey bool
}

type serviceTemplateData struct {
	Module          string
	InputName       string
	Dir             string
	ProtoFile       string
	ProtoPackage    string
	GoPackage       string
	GoIdent         string
	Pascal          string
	ServiceName     string
	Port            int
	TableName       string
	SkipAutoMigrate bool
}

// AddService 在已有 bw-cli 项目中创建完整 gRPC 服务骨架。
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
	if err := applyTableOption(root, opts, &data); err != nil {
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

// DeleteService 删除 AddService 生成的文件和配置项。
func DeleteService(opts DeleteServiceOptions) error {
	root, err := serviceRoot(opts.RootDir)
	if err != nil {
		return err
	}
	module, err := readModule(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	data, err := buildServiceTemplateData(module, opts.Name, 0)
	if err != nil {
		return err
	}
	if err := removeServicePaths(root, data); err != nil {
		return err
	}
	if err := removeServiceMakeTarget(root, data.Dir); err != nil {
		return err
	}
	if err := removeServiceConfig(root, data.Dir); err != nil {
		return err
	}
	if err := removeGatewayRegistration(root, data); err != nil {
		return err
	}
	if enabled, dataID := nacosEnabled(root); enabled {
		fmt.Printf("nacos is enabled: sync configs/config.yaml service removal to Nacos data_id %s\n", dataID)
	}
	if opts.RunProto {
		if err := runProjectCommand(root, "go", "run", "./tools/protogen"); err != nil {
			return fmt.Errorf("generate proto after deleting %s: %w", data.Dir, err)
		}
	}
	if err := gofmtAfterDelete(root); err != nil {
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
		Port:         port,
		TableName:    dir + "s",
	}, nil
}

func applyTableOption(root string, opts ServiceOptions, data *serviceTemplateData) error {
	table := strings.TrimSpace(opts.TableName)
	if table == "" {
		return nil
	}
	if !tableNamePattern.MatchString(table) {
		return fmt.Errorf("table name %q must contain only letters, digits and underscore, and cannot start with a digit", table)
	}
	columns, err := loadTableColumns(root, table, opts.SchemaName)
	if err != nil {
		return err
	}
	if err := validateDefaultCRUDColumns(table, columns); err != nil {
		return err
	}
	data.TableName = table
	data.SkipAutoMigrate = true
	return nil
}

func loadTableColumns(root string, table string, schema string) ([]tableColumn, error) {
	configPath := filepath.Join(root, "configs", "config.yaml")
	if !exists(configPath) {
		return nil, fmt.Errorf("--table requires %s", configPath)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config for table %s: %w", table, err)
	}
	dbCfg := cfg.Database
	if strings.EqualFold(dbCfg.Driver, "sqlite") && dbCfg.DSN != "" && !filepath.IsAbs(dbCfg.DSN) {
		dbCfg.DSN = filepath.Join(root, dbCfg.DSN)
	}
	db, err := database.Open(dbCfg, cfg.MySQL, cfg.PostgreSQL, zap.NewNop())
	if err != nil {
		return nil, fmt.Errorf("open database for table %s: %w", table, err)
	}
	sqlDB, err := db.DB()
	if err == nil {
		defer sqlDB.Close()
	}
	return inspectTableColumns(db, dbCfg.Driver, table, schema)
}

func inspectTableColumns(db *gorm.DB, driver string, table string, schema string) ([]tableColumn, error) {
	switch strings.ToLower(driver) {
	case "sqlite":
		return inspectSQLiteColumns(db, table)
	case "mysql":
		return inspectMySQLColumns(db, table, schema)
	case "postgres", "postgresql", "pg":
		return inspectPostgreSQLColumns(db, table, schema)
	default:
		return nil, fmt.Errorf("--table does not support database driver %q", driver)
	}
}

func inspectSQLiteColumns(db *gorm.DB, table string) ([]tableColumn, error) {
	rows, err := db.Raw("PRAGMA table_info(" + quoteSQLiteIdent(table) + ")").Rows()
	if err != nil {
		return nil, fmt.Errorf("inspect sqlite table %s: %w", table, err)
	}
	defer rows.Close()
	columns := []tableColumn{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		columns = append(columns, tableColumn{
			Name:       name,
			PrimaryKey: pk > 0,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("table %s not found or has no columns", table)
	}
	return columns, nil
}

func inspectMySQLColumns(db *gorm.DB, table string, schema string) ([]tableColumn, error) {
	if schema == "" {
		var current string
		if err := db.Raw("select database()").Scan(&current).Error; err != nil {
			return nil, err
		}
		schema = current
	}
	rows, err := db.Raw(`select column_name, data_type, is_nullable, column_key, extra, column_default
from information_schema.columns
where table_schema = ? and table_name = ?
order by ordinal_position`, schema, table).Rows()
	if err != nil {
		return nil, fmt.Errorf("inspect mysql table %s: %w", table, err)
	}
	defer rows.Close()
	return scanInformationSchemaColumns(rows, table)
}

func inspectPostgreSQLColumns(db *gorm.DB, table string, schema string) ([]tableColumn, error) {
	if schema == "" {
		schema = "public"
	}
	rows, err := db.Raw(`select c.column_name, c.data_type, c.is_nullable,
case when tc.constraint_type = 'PRIMARY KEY' then 'PRI' else '' end as column_key,
'' as extra,
c.column_default
from information_schema.columns c
left join information_schema.key_column_usage kcu
  on c.table_schema = kcu.table_schema and c.table_name = kcu.table_name and c.column_name = kcu.column_name
left join information_schema.table_constraints tc
  on kcu.constraint_schema = tc.constraint_schema and kcu.constraint_name = tc.constraint_name
where c.table_schema = ? and c.table_name = ?
order by c.ordinal_position`, schema, table).Rows()
	if err != nil {
		return nil, fmt.Errorf("inspect postgresql table %s: %w", table, err)
	}
	defer rows.Close()
	return scanInformationSchemaColumns(rows, table)
}

func scanInformationSchemaColumns(rows *sql.Rows, table string) ([]tableColumn, error) {
	columns := []tableColumn{}
	for rows.Next() {
		var name, typ, nullable, key, extra string
		var defaultValue any
		if err := rows.Scan(&name, &typ, &nullable, &key, &extra, &defaultValue); err != nil {
			return nil, err
		}
		columns = append(columns, tableColumn{
			Name:       name,
			PrimaryKey: key == "PRI",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("table %s not found or has no columns", table)
	}
	return columns, nil
}

func quoteSQLiteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func validateDefaultCRUDColumns(table string, columns []tableColumn) error {
	seen := make(map[string]tableColumn, len(columns))
	for _, column := range columns {
		seen[strings.ToLower(column.Name)] = column
	}
	id := seen["id"]
	if !id.PrimaryKey {
		return fmt.Errorf("--table %s requires id to be the primary key", table)
	}
	return nil
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
		filepath.Join("api", "proto", data.Dir, "v1", data.ProtoFile):      renderServiceTemplate(serviceProtoTemplate, data),
		filepath.Join("cmd", data.Dir, "main.go"):                          renderServiceTemplate(serviceMainTemplate, data),
		filepath.Join("internal", data.Dir, "entity", data.Dir+".go"):      renderServiceTemplate(serviceEntityTemplate, data),
		filepath.Join("internal", data.Dir, "entity", "repository.go"):     renderServiceTemplate(serviceRepositoryTemplate, data),
		filepath.Join("internal", data.Dir, "model", data.Dir+".go"):       renderServiceTemplate(servicePersistenceModelTemplate, data),
		filepath.Join("internal", data.Dir, "dto", "command.go"):           renderServiceTemplate(serviceCommandTemplate, data),
		filepath.Join("internal", data.Dir, "dto", data.Dir+".go"):         renderServiceTemplate(serviceDTOTemplate, data),
		filepath.Join("internal", data.Dir, "service", "service.go"):       renderServiceTemplate(serviceUseCaseTemplate, data),
		filepath.Join("internal", data.Dir, "service", "service_test.go"):  renderServiceTemplate(serviceUseCaseTestTemplate, data),
		filepath.Join("internal", data.Dir, "repo", "gorm_repository.go"):  renderServiceTemplate(serviceGormRepoTemplate, data),
		filepath.Join("internal", data.Dir, "repo", "mongo_repository.go"): renderServiceTemplate(serviceMongoRepoTemplate, data),
		filepath.Join("internal", data.Dir, "handler", "server.go"):        renderServiceTemplate(serviceHandlerTemplate, data),
		filepath.Join("docs", "services", data.Dir+".md"):                  renderServiceTemplate(serviceDocTemplate, data),
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
	clientsPath := filepath.Join(root, "internal", "gateway", "client", "clients.go")
	if err := ensureGatewayClientsFile(clientsPath, data); err != nil {
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

func removeServicePaths(root string, data serviceTemplateData) error {
	for _, rel := range []string{
		filepath.Join("cmd", data.Dir),
		filepath.Join("internal", data.Dir),
		filepath.Join("api", "proto", data.Dir),
		filepath.Join("api", "gen", data.Dir),
		filepath.Join("internal", "gateway", "request", data.Dir+"_request.go"),
		filepath.Join("internal", "gateway", "handler", data.Dir+"_handler.go"),
		filepath.Join("internal", "gateway", "router", data.Dir+"_routes.go"),
		filepath.Join("docs", "services", data.Dir+".md"),
	} {
		path := filepath.Join(root, rel)
		if !exists(path) {
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove %s: %w", rel, err)
		}
	}
	return nil
}

func removeServiceMakeTarget(root string, serviceDir string) error {
	path := filepath.Join(root, "Makefile")
	if !exists(path) {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	target := "run-" + serviceDir
	lines := strings.Split(string(content), "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, ".PHONY:") {
			out = append(out, removeMakePhonyTarget(line, target))
			continue
		}
		if strings.TrimSpace(line) == target+":" {
			i++
			for i < len(lines) && (strings.HasPrefix(lines[i], "\t") || strings.TrimSpace(lines[i]) == "") {
				i++
			}
			i--
			continue
		}
		out = append(out, line)
	}
	return os.WriteFile(path, []byte(strings.TrimRight(strings.Join(out, "\n"), "\n")+"\n"), 0o644)
}

func removeMakePhonyTarget(line string, target string) string {
	parts := strings.Fields(line)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == target {
			continue
		}
		out = append(out, part)
	}
	return strings.Join(out, " ")
}

func removeServiceConfig(root string, serviceDir string) error {
	path := filepath.Join(root, "configs", "config.yaml")
	if !exists(path) {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(content), "\n")
	servicesLine := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "services:" && !strings.HasPrefix(line, " ") {
			servicesLine = i
			break
		}
	}
	if servicesLine == -1 {
		return nil
	}
	start := -1
	serviceHeader := "  " + serviceDir + ":"
	for i := servicesLine + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			break
		}
		if line == serviceHeader {
			start = i
			break
		}
	}
	if start == -1 {
		return nil
	}
	end := start + 1
	for end < len(lines) {
		line := lines[end]
		if strings.TrimSpace(line) == "" {
			end++
			continue
		}
		if !strings.HasPrefix(line, " ") || (strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ")) {
			break
		}
		end++
	}
	lines = append(lines[:start], lines[end:]...)
	return os.WriteFile(path, []byte(strings.TrimRight(strings.Join(lines, "\n"), "\n")+"\n"), 0o644)
}

func removeGatewayRegistration(root string, data serviceTemplateData) error {
	if err := removeGatewayV1Registration(filepath.Join(root, "internal", "gateway", "router", "v1.go"), data); err != nil {
		return err
	}
	return removeGatewayClient(filepath.Join(root, "internal", "gateway", "client", "clients.go"), data)
}

func removeGatewayV1Registration(path string, data serviceTemplateData) error {
	if !exists(path) {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	registrationPattern := regexp.MustCompile(`(?m)^\s*register` + regexp.QuoteMeta(data.Pascal) + `Routes\(v1,\s*handler\.New` + regexp.QuoteMeta(data.Pascal) + `Handler\([^\n]*\)\)\s*$\n?`)
	text := registrationPattern.ReplaceAllString(string(content), "")
	if !hasGatewayRouteRegistration(text) {
		text = removeImport(text, fmt.Sprintf("%q", data.Module+"/internal/gateway/handler"))
		text = ensureUnusedGatewayV1Line(text)
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func hasGatewayRouteRegistration(text string) bool {
	return regexp.MustCompile(`register[A-Za-z0-9]+Routes\(v1,`).MatchString(text)
}

func ensureUnusedGatewayV1Line(text string) string {
	if strings.Contains(text, "\n\t_ = v1\n") {
		return text
	}
	marker := "\tv1 := api.Group(\"/v1\")\n"
	if !strings.Contains(text, marker) {
		return text
	}
	return strings.Replace(text, marker, marker+"\t_ = v1\n", 1)
}

func removeGatewayClient(path string, data serviceTemplateData) error {
	if !exists(path) {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(content)
	text = removeImport(text, fmt.Sprintf("%s %q", data.GoPackage, data.Module+"/api/gen/"+data.Dir+"/v1"))
	text = regexp.MustCompile(`(?m)^\s*`+regexp.QuoteMeta(data.Pascal)+`\s+`+regexp.QuoteMeta(data.GoPackage)+`\.`+regexp.QuoteMeta(data.Pascal)+`ServiceClient\s*$\n?`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?m)^\s*`+regexp.QuoteMeta(data.GoIdent)+`Target := cfg\.ServiceTarget\("`+regexp.QuoteMeta(data.Dir)+`"\)\s*$\n?`).ReplaceAllString(text, "")
	text = removeGatewayClientDialBlock(text, data)
	text = regexp.MustCompile(`(?m)^\s*zap\.String\("`+regexp.QuoteMeta(data.Dir)+`_target",\s*`+regexp.QuoteMeta(data.GoIdent)+`Target\),\s*$\n?`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?m)^\s*`+regexp.QuoteMeta(data.Pascal)+`:\s+`+regexp.QuoteMeta(data.GoPackage)+`\.New`+regexp.QuoteMeta(data.Pascal)+`ServiceClient\(`+regexp.QuoteMeta(data.GoIdent)+`Conn\),\s*$\n?`).ReplaceAllString(text, "")
	text = removeGatewayConnFromList(text, data.GoIdent+"Conn")
	text = removeGatewayConnCloseLines(text, data.GoIdent+"Conn")
	if !strings.Contains(text, "fmt.") {
		text = removeImport(text, "\"fmt\"")
	}
	if !strings.Contains(text, "insecure.") {
		text = removeImport(text, "\"google.golang.org/grpc/credentials/insecure\"")
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func removeGatewayClientDialBlock(text string, data serviceTemplateData) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !strings.Contains(line, data.GoIdent+"Conn, err := grpc.Dial") {
			out = append(out, line)
			continue
		}
		i++
		for i < len(lines) {
			if strings.TrimSpace(lines[i]) == "}" {
				break
			}
			i++
		}
	}
	return strings.Join(out, "\n")
}

func removeGatewayConnFromList(text string, connName string) string {
	return gatewayClientConnsPattern.ReplaceAllStringFunc(text, func(value string) string {
		prefix := value[:strings.Index(value, "{")+1]
		body := strings.TrimSuffix(strings.TrimPrefix(value, prefix), "}")
		parts := strings.Split(body, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			name := strings.TrimSpace(part)
			if name == "" || name == connName {
				continue
			}
			out = append(out, name)
		}
		return prefix + strings.Join(out, ", ") + "}"
	})
}

func removeGatewayConnCloseLines(text string, connName string) string {
	return regexp.MustCompile(`(?m)^\s*`+regexp.QuoteMeta(connName)+`\.Close\(\)\s*$\n?`).ReplaceAllString(text, "")
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
		filepath.Join("internal", serviceDir, "entity", serviceDir+".go"),
		filepath.Join("internal", serviceDir, "entity", "repository.go"),
		filepath.Join("internal", serviceDir, "model", serviceDir+".go"),
		filepath.Join("internal", serviceDir, "dto", "command.go"),
		filepath.Join("internal", serviceDir, "dto", serviceDir+".go"),
		filepath.Join("internal", serviceDir, "service", "service.go"),
		filepath.Join("internal", serviceDir, "service", "service_test.go"),
		filepath.Join("internal", serviceDir, "repo", "gorm_repository.go"),
		filepath.Join("internal", serviceDir, "repo", "mongo_repository.go"),
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

func gofmtAfterDelete(root string) error {
	args := []string{}
	for _, rel := range []string{
		filepath.Join("internal", "gateway", "client", "clients.go"),
		filepath.Join("internal", "gateway", "router", "v1.go"),
		filepath.Join("internal", "gateway", "router", "router.go"),
		filepath.Join("cmd", "gateway", "main.go"),
	} {
		if exists(filepath.Join(root, rel)) {
			args = append(args, rel)
		}
	}
	if len(args) == 0 {
		return nil
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
		routerText = ensureImport(routerText, fmt.Sprintf("%q", data.Module+"/internal/gateway/client"))
		if strings.Contains(routerText, "func New(log *zap.Logger, middlewareCfg config.MiddlewareConfig) *gin.Engine") {
			routerText = strings.Replace(routerText, "func New(log *zap.Logger, middlewareCfg config.MiddlewareConfig) *gin.Engine", "func New(clients *client.Clients, log *zap.Logger, middlewareCfg config.MiddlewareConfig) *gin.Engine", 1)
		}
		if strings.Contains(routerText, "registerAPIRoutes(r)") {
			routerText = strings.Replace(routerText, "registerAPIRoutes(r)", "registerAPIRoutes(r, clients, log)", 1)
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
	registration := fmt.Sprintf("register%sRoutes(v1, handler.New%sHandler(clients.%s, log))", data.Pascal, data.Pascal, data.Pascal)
	if strings.Contains(v1Text, registration) {
		return nil
	}
	if strings.Contains(v1Text, "func registerAPIRoutes(r *gin.Engine)") {
		return os.WriteFile(v1Path, []byte(renderServiceTemplate(cleanGatewayV1WithServiceTemplate, data)), 0o644)
	}
	if !strings.Contains(v1Text, "func registerAPIRoutes(r *gin.Engine, clients *client.Clients") {
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
	text = ensureImport(text, fmt.Sprintf("%q", data.Module+"/internal/gateway/client"))
	if strings.Contains(text, "engine := router.New(log, cfg.Middleware)") {
		initBlock := `gatewayClients, err := client.New(cfg, log)
	if err != nil {
		log.Fatal("initialize grpc clients failed", zap.Error(err))
	}
	defer gatewayClients.Close()

	engine := router.New(gatewayClients, log, cfg.Middleware)`
		text = strings.Replace(text, "engine := router.New(log, cfg.Middleware)", initBlock, 1)
	}
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

// {{ .Pascal }}Service 是 {{ .Dir }} 业务服务的 gRPC 边界。
// 默认 CRUD 契约可直接调用；业务增长时再扩展 message 和 RPC。
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
{{ if not .SkipAutoMigrate }}
	if err := {{ .GoIdent }}repo.AutoMigrate(db); err != nil {
		log.Fatal("migrate {{ .Dir }} database failed", zap.Error(err))
	}
{{ end }}

	repo := {{ .GoIdent }}repo.NewGormRepository(db, log)
	svc := {{ .GoIdent }}service.NewService(repo, log)
	server := grpc.NewServer(grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor(log)))
	{{ .GoPackage }}.Register{{ .Pascal }}ServiceServer(server, {{ .GoIdent }}handler.NewServer(svc, log))

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

const serviceEntityTemplate = `package entity

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

// {{ .Pascal }} 是 {{ .Dir }} 业务服务的聚合根。
// 业务明确后，请将 Name 和 Description 替换为真实业务字段。
type {{ .Pascal }} struct {
	ID        string
	Name        string
	Description string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// New{{ .Pascal }} 校验输入，并创建带框架管理身份字段的聚合。
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

// Update 修改可变字段，并把校验保留在业务实体内部。
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

const serviceRepositoryTemplate = `package entity

import "context"

// Repository 定义 {{ .Dir }} service 层需要的持久化行为。
type Repository interface {
	Save(ctx context.Context, item *{{ .Pascal }}) error
	FindByID(ctx context.Context, id string) (*{{ .Pascal }}, error)
	List(ctx context.Context, offset int, limit int) ([]*{{ .Pascal }}, int64, error)
	Delete(ctx context.Context, id string) error
}
`

const servicePersistenceModelTemplate = `package model

import "time"

// {{ .Pascal }}Model 是 {{ .TableName }} 表的 Gorm 持久化模型。
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

const {{ .GoIdent }}MongoCollectionName = "{{ .TableName }}"

// {{ .Pascal }}Document 是 {{ .TableName }} 集合的 MongoDB 文档模型。
type {{ .Pascal }}Document struct {
	ID          string    ` + "`bson:\"_id\"`" + `
	Name        string    ` + "`bson:\"name\"`" + `
	Description string    ` + "`bson:\"description\"`" + `
	CreatedAt   time.Time ` + "`bson:\"created_at\"`" + `
	UpdatedAt   time.Time ` + "`bson:\"updated_at\"`" + `
}

func ({{ .Pascal }}Document) MongoCollectionName() string {
	return {{ .GoIdent }}MongoCollectionName
}
`

const serviceCommandTemplate = `package dto

// CreateCommand 包含创建 {{ .Dir }} 记录的入参。
type CreateCommand struct {
	Name        string
	Description string
}

// UpdateCommand 包含更新 {{ .Dir }} 记录的入参。
type UpdateCommand struct {
	ID          string
	Name        string
	Description string
}

// ListCommand 包含查询 {{ .Dir }} 记录列表的分页入参。
type ListCommand struct {
	Page     int32
	PageSize int32
}
`

const serviceDTOTemplate = `package dto

import (
	"time"

	"{{ .Module }}/internal/{{ .Dir }}/entity"
)

// {{ .Pascal }}DTO 由用例返回，并由 handler 转换。
type {{ .Pascal }}DTO struct {
	ID          string
	Name        string
	Description string
	CreatedAt   string
	UpdatedAt   string
}

// List{{ .Pascal }}DTO 包含分页列表出参。
type List{{ .Pascal }}DTO struct {
	Items []*{{ .Pascal }}DTO
	Total int64
}

// From{{ .Pascal }} 将 {{ .Dir }} 聚合转换为 service 响应 DTO。
func From{{ .Pascal }}(item *entity.{{ .Pascal }}) *{{ .Pascal }}DTO {
	return &{{ .Pascal }}DTO{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   formatTime(item.CreatedAt),
		UpdatedAt:   formatTime(item.UpdatedAt),
	}
}

// formatTime 让零值时间保持为空，并用稳定 API 格式序列化真实时间。
func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}
`

const serviceUseCaseTemplate = `package service

import (
	"context"

	"go.uber.org/zap"

	"{{ .Module }}/internal/{{ .Dir }}/dto"
	"{{ .Module }}/internal/{{ .Dir }}/entity"
)

// Service 编排 {{ .Dir }} 用例。
type Service struct {
	repo entity.Repository
	log  *zap.Logger
}

// NewService 创建 {{ .Dir }} 用例服务。
func NewService(repo entity.Repository, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	return &Service{repo: repo, log: log}
}

// Create 创建 {{ .Dir }} 记录。
func (s *Service) Create(ctx context.Context, cmd dto.CreateCommand) (*dto.{{ .Pascal }}DTO, error) {
	item, err := entity.New{{ .Pascal }}(cmd.Name, cmd.Description)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, item); err != nil {
		return nil, err
	}
	s.log.Info("{{ .Dir }} created", zap.String("aggregate_id", item.ID), zap.String("use_case", "Create{{ .Pascal }}"))
	return dto.From{{ .Pascal }}(item), nil
}

// Get 根据 ID 返回一条 {{ .Dir }} 记录。
func (s *Service) Get(ctx context.Context, id string) (*dto.{{ .Pascal }}DTO, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return dto.From{{ .Pascal }}(item), nil
}

// List 返回分页的 {{ .Dir }} 记录。
func (s *Service) List(ctx context.Context, cmd dto.ListCommand) (*dto.List{{ .Pascal }}DTO, error) {
	offset, limit := normalizePagination(cmd.Page, cmd.PageSize)
	items, total, err := s.repo.List(ctx, offset, limit)
	if err != nil {
		return nil, err
	}
	output := &dto.List{{ .Pascal }}DTO{Items: make([]*dto.{{ .Pascal }}DTO, 0, len(items)), Total: total}
	for _, item := range items {
		output.Items = append(output.Items, dto.From{{ .Pascal }}(item))
	}
	return output, nil
}

// Update 根据 ID 修改一条 {{ .Dir }} 记录。
func (s *Service) Update(ctx context.Context, cmd dto.UpdateCommand) (*dto.{{ .Pascal }}DTO, error) {
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
	return dto.From{{ .Pascal }}(item), nil
}

// Delete 根据 ID 删除一条 {{ .Dir }} 记录。
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

const serviceUseCaseTestTemplate = `package service

import (
	"context"
	"testing"

	"{{ .Module }}/internal/{{ .Dir }}/dto"
	"{{ .Module }}/internal/{{ .Dir }}/entity"
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

	created, err := svc.Create(ctx, dto.CreateCommand{Name: "first", Description: "created from service test"})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "first", created.Name)

	got, err := svc.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)

	list, err := svc.List(ctx, dto.ListCommand{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), list.Total)
	require.Len(t, list.Items, 1)

	updated, err := svc.Update(ctx, dto.UpdateCommand{ID: created.ID, Name: "updated", Description: "updated from service test"})
	require.NoError(t, err)
	require.Equal(t, "updated", updated.Name)

	require.NoError(t, svc.Delete(ctx, created.ID))
	_, err = svc.Get(ctx, created.ID)
	require.ErrorIs(t, err, entity.Err{{ .Pascal }}NotFound)
}

type fakeRepository struct {
	items map[string]*entity.{{ .Pascal }}
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{items: make(map[string]*entity.{{ .Pascal }})}
}

func (r *fakeRepository) Save(ctx context.Context, item *entity.{{ .Pascal }}) error {
	copy := *item
	r.items[item.ID] = &copy
	return nil
}

func (r *fakeRepository) FindByID(ctx context.Context, id string) (*entity.{{ .Pascal }}, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, entity.Err{{ .Pascal }}NotFound
	}
	copy := *item
	return &copy, nil
}

func (r *fakeRepository) List(ctx context.Context, offset int, limit int) ([]*entity.{{ .Pascal }}, int64, error) {
	items := make([]*entity.{{ .Pascal }}, 0, len(r.items))
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
		return entity.Err{{ .Pascal }}NotFound
	}
	delete(r.items, id)
	return nil
}

var _ entity.Repository = (*fakeRepository)(nil)
`

const serviceGormRepoTemplate = `package repo

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"{{ .Module }}/internal/{{ .Dir }}/entity"
	dbmodel "{{ .Module }}/internal/{{ .Dir }}/model"
)

// GormRepository 使用 Gorm 持久化 {{ .Dir }} 聚合。
type GormRepository struct {
	db  *gorm.DB
	log *zap.Logger
}

// NewGormRepository 创建 {{ .Dir }} 仓储，并支持可选结构化日志。
func NewGormRepository(db *gorm.DB, loggers ...*zap.Logger) *GormRepository {
	log := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}
	return &GormRepository{db: db, log: log}
}

// AutoMigrate 创建或更新 {{ .TableName }} 表结构。
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&dbmodel.{{ .Pascal }}Model{})
}

// Save 新增或更新 {{ .Dir }} 聚合。
func (r *GormRepository) Save(ctx context.Context, item *entity.{{ .Pascal }}) error {
	start := time.Now()
	tx := r.db.WithContext(ctx).Save(toRecord(item))
	r.logOperation("Save", tx.RowsAffected, start, tx.Error)
	return tx.Error
}

// FindByID 根据 ID 加载 {{ .Dir }} 聚合。
func (r *GormRepository) FindByID(ctx context.Context, id string) (*entity.{{ .Pascal }}, error) {
	start := time.Now()
	var record dbmodel.{{ .Pascal }}Model
	tx := r.db.WithContext(ctx).Where("id = ?", id).First(&record)
	err := tx.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = entity.Err{{ .Pascal }}NotFound
	}
	if err != nil {
		r.logOperation("FindByID", tx.RowsAffected, start, err)
		return nil, err
	}
	r.logOperation("FindByID", tx.RowsAffected, start, nil)
	return toDomain(&record), nil
}

// List 加载分页 {{ .Dir }} 聚合。
func (r *GormRepository) List(ctx context.Context, offset int, limit int) ([]*entity.{{ .Pascal }}, int64, error) {
	start := time.Now()
	var total int64
	countTx := r.db.WithContext(ctx).Model(&dbmodel.{{ .Pascal }}Model{}).Count(&total)
	if countTx.Error != nil {
		r.logOperation("Count", countTx.RowsAffected, start, countTx.Error)
		return nil, 0, countTx.Error
	}
	var records []dbmodel.{{ .Pascal }}Model
	tx := r.db.WithContext(ctx).
		Order("created_at desc").
		Offset(offset).
		Limit(limit).
		Find(&records)
	if tx.Error != nil {
		r.logOperation("List", tx.RowsAffected, start, tx.Error)
		return nil, 0, tx.Error
	}
	items := make([]*entity.{{ .Pascal }}, 0, len(records))
	for i := range records {
		items = append(items, toDomain(&records[i]))
	}
	r.logOperation("List", tx.RowsAffected, start, nil)
	return items, total, nil
}

// Delete 根据 ID 删除 {{ .Dir }} 聚合。
func (r *GormRepository) Delete(ctx context.Context, id string) error {
	start := time.Now()
	tx := r.db.WithContext(ctx).Where("id = ?", id).Delete(&dbmodel.{{ .Pascal }}Model{})
	err := tx.Error
	if err == nil && tx.RowsAffected == 0 {
		err = entity.Err{{ .Pascal }}NotFound
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

func toRecord(item *entity.{{ .Pascal }}) *dbmodel.{{ .Pascal }}Model {
	return &dbmodel.{{ .Pascal }}Model{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func toDomain(record *dbmodel.{{ .Pascal }}Model) *entity.{{ .Pascal }} {
	return &entity.{{ .Pascal }}{
		ID:          record.ID,
		Name:        record.Name,
		Description: record.Description,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}

var _ entity.Repository = (*GormRepository)(nil)
`

const serviceMongoRepoTemplate = `package repo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"

	"{{ .Module }}/internal/{{ .Dir }}/entity"
	dbmodel "{{ .Module }}/internal/{{ .Dir }}/model"
	"{{ .Module }}/pkg/mongox"
)

// MongoRepository 使用共享 mongox 文档存储持久化 {{ .Dir }} 聚合。
// 它实现 entity.Repository，可在不修改 service 代码的情况下替换 GormRepository。
type MongoRepository struct {
	documents *mongox.DocumentStore[dbmodel.{{ .Pascal }}Document]
	log       *zap.Logger
}

// NewMongoRepository 使用配置好的数据库创建 MongoDB 仓储。
func NewMongoRepository(db *mongo.Database, loggers ...*zap.Logger) *MongoRepository {
	log := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}
	return &MongoRepository{
		documents: mongox.NewDocumentStore[dbmodel.{{ .Pascal }}Document](db, log),
		log:       log,
	}
}

// Save 按 MongoDB _id 新增或更新 {{ .Dir }} 聚合。
func (r *MongoRepository) Save(ctx context.Context, item *entity.{{ .Pascal }}) error {
	start := time.Now()
	_, err := r.documents.UpsertByID(ctx, item.ID, toDocument(item))
	r.logOperation("Save", item.ID, 0, start, err)
	return err
}

// FindByID 按 MongoDB _id 加载 {{ .Dir }} 聚合。
func (r *MongoRepository) FindByID(ctx context.Context, id string) (*entity.{{ .Pascal }}, error) {
	start := time.Now()
	document, err := r.documents.FindByID(ctx, id)
	if errors.Is(err, mongox.ErrNotFound) {
		err = entity.Err{{ .Pascal }}NotFound
	}
	r.logOperation("FindByID", id, 0, start, err)
	if err != nil {
		return nil, err
	}
	return toDomainFromDocument(document), nil
}

// List 按创建时间排序加载分页 {{ .Dir }} 聚合。
func (r *MongoRepository) List(ctx context.Context, offset int, limit int) ([]*entity.{{ .Pascal }}, int64, error) {
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

	items := make([]*entity.{{ .Pascal }}, 0, len(documents))
	for i := range documents {
		items = append(items, toDomainFromDocument(&documents[i]))
	}
	r.logOperation("List", "", total, start, nil)
	return items, total, nil
}

// Delete 按 MongoDB _id 删除 {{ .Dir }} 聚合。
func (r *MongoRepository) Delete(ctx context.Context, id string) error {
	start := time.Now()
	result, err := r.documents.DeleteByID(ctx, id)
	if err == nil && result != nil && result.DeletedCount == 0 {
		err = entity.Err{{ .Pascal }}NotFound
	}
	r.logOperation("Delete", id, 0, start, err)
	return err
}

func (r *MongoRepository) logOperation(operation string, id string, total int64, start time.Time, err error) {
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

func toDocument(item *entity.{{ .Pascal }}) *dbmodel.{{ .Pascal }}Document {
	return &dbmodel.{{ .Pascal }}Document{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func toDomainFromDocument(document *dbmodel.{{ .Pascal }}Document) *entity.{{ .Pascal }} {
	return &entity.{{ .Pascal }}{
		ID:          document.ID,
		Name:        document.Name,
		Description: document.Description,
		CreatedAt:   document.CreatedAt,
		UpdatedAt:   document.UpdatedAt,
	}
}

var _ entity.Repository = (*MongoRepository)(nil)
`

const serviceHandlerTemplate = `package handler

import (
	"context"
	stderrors "errors"

	"go.uber.org/zap"

	{{ .GoPackage }} "{{ .Module }}/api/gen/{{ .Dir }}/v1"
	"{{ .Module }}/internal/{{ .Dir }}/dto"
	"{{ .Module }}/internal/{{ .Dir }}/entity"
	"{{ .Module }}/internal/{{ .Dir }}/service"
	apperrors "{{ .Module }}/pkg/errors"
)

// Server 将 {{ .Dir }} gRPC 请求适配到 service 用例。
type Server struct {
	{{ .GoPackage }}.Unimplemented{{ .Pascal }}ServiceServer
	svc *service.Service
	log *zap.Logger
}

// NewServer 创建 {{ .Dir }} gRPC 服务端适配器。
func NewServer(svc *service.Service, log *zap.Logger) *Server {
	if log == nil {
		log = zap.NewNop()
	}
	return &Server{svc: svc, log: log}
}

// Create{{ .Pascal }} 处理创建 RPC。
func (s *Server) Create{{ .Pascal }}(ctx context.Context, req *{{ .GoPackage }}.Create{{ .Pascal }}Request) (*{{ .GoPackage }}.{{ .Pascal }}Response, error) {
	item, err := s.svc.Create(ctx, dto.CreateCommand{
		Name:        req.GetName(),
		Description: req.GetDescription(),
	})
	if err != nil {
		return nil, map{{ .Pascal }}Error(err)
	}
	return toProto(item), nil
}

// Get{{ .Pascal }} 处理按 ID 查询。
func (s *Server) Get{{ .Pascal }}(ctx context.Context, req *{{ .GoPackage }}.Get{{ .Pascal }}Request) (*{{ .GoPackage }}.{{ .Pascal }}Response, error) {
	item, err := s.svc.Get(ctx, req.GetId())
	if err != nil {
		return nil, map{{ .Pascal }}Error(err)
	}
	return toProto(item), nil
}

// List{{ .Pascal }}s 处理分页列表查询。
func (s *Server) List{{ .Pascal }}s(ctx context.Context, req *{{ .GoPackage }}.List{{ .Pascal }}sRequest) (*{{ .GoPackage }}.List{{ .Pascal }}sResponse, error) {
	list, err := s.svc.List(ctx, dto.ListCommand{
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

// Update{{ .Pascal }} 处理按 ID 更新。
func (s *Server) Update{{ .Pascal }}(ctx context.Context, req *{{ .GoPackage }}.Update{{ .Pascal }}Request) (*{{ .GoPackage }}.{{ .Pascal }}Response, error) {
	item, err := s.svc.Update(ctx, dto.UpdateCommand{
		ID:          req.GetId(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
	})
	if err != nil {
		return nil, map{{ .Pascal }}Error(err)
	}
	return toProto(item), nil
}

// Delete{{ .Pascal }} 处理按 ID 删除。
func (s *Server) Delete{{ .Pascal }}(ctx context.Context, req *{{ .GoPackage }}.Delete{{ .Pascal }}Request) (*{{ .GoPackage }}.Delete{{ .Pascal }}Response, error) {
	if err := s.svc.Delete(ctx, req.GetId()); err != nil {
		return nil, map{{ .Pascal }}Error(err)
	}
	return &{{ .GoPackage }}.Delete{{ .Pascal }}Response{Success: true}, nil
}

func toProto(item *dto.{{ .Pascal }}DTO) *{{ .GoPackage }}.{{ .Pascal }}Response {
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
	case stderrors.Is(err, entity.ErrInvalid{{ .Pascal }}):
		return apperrors.InvalidArgument("invalid_{{ .Dir }}", "invalid {{ .Dir }} input")
	case stderrors.Is(err, entity.Err{{ .Pascal }}NotFound):
		return apperrors.NotFound("{{ .Dir }}_not_found", "{{ .Dir }} not found")
	default:
		return apperrors.Wrap(apperrors.KindInternal, "{{ .Dir }}_service_error", "{{ .Dir }} service error", err)
	}
}
`

const gatewayClientsTemplate = `package client

import (
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	{{ .GoPackage }} "{{ .Module }}/api/gen/{{ .Dir }}/v1"
	"{{ .Module }}/pkg/config"
)

// Clients 聚合 HTTP gateway 使用的所有 gRPC client。
type Clients struct {
	{{ .Pascal }}  {{ .GoPackage }}.{{ .Pascal }}ServiceClient
	Config *config.Config

	conns []*grpc.ClientConn
}

// New 连接配置的 gRPC 目标地址并创建强类型服务 client。
func New(cfg *config.Config, log *zap.Logger) (*Clients, error) {
	{{ .GoIdent }}Target := cfg.ServiceTarget("{{ .Dir }}")
	{{ .GoIdent }}Conn, err := grpc.Dial({{ .GoIdent }}Target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial {{ .Dir }} service: %w", err)
	}

	log.Info("grpc clients initialized",
		zap.String("{{ .Dir }}_target", {{ .GoIdent }}Target),
	)
	return &Clients{
		{{ .Pascal }}:  {{ .GoPackage }}.New{{ .Pascal }}ServiceClient({{ .GoIdent }}Conn),
		Config: cfg,
		conns:  []*grpc.ClientConn{ {{ .GoIdent }}Conn },
	}, nil
}

// Close 释放 gateway 的所有 gRPC client 连接。
func (c *Clients) Close() {
	for _, conn := range c.conns {
		_ = conn.Close()
	}
}
`

const gatewayCommonTemplate = `package handler

import (
	"context"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/metadata"

	"{{ .Module }}/pkg/grpcx"
	"{{ .Module }}/pkg/httpx"
)

// outgoingContext 将请求 ID 等 gateway 元数据转发到下游 gRPC 调用。
func outgoingContext(c *gin.Context) context.Context {
	return metadata.AppendToOutgoingContext(c.Request.Context(), grpcx.MetadataRequestID, httpx.RequestID(c))
}
`

const gatewayRequestTemplate = `package request

// Create{{ .Pascal }}Request 是 POST /api/v1/{{ .TableName }} 使用的 JSON 载荷。
type Create{{ .Pascal }}Request struct {
	Name        string ` + "`json:\"name\" binding:\"required\"`" + `
	Description string ` + "`json:\"description\"`" + `
}

// Update{{ .Pascal }}Request 是 PUT /api/v1/{{ .TableName }}/:id 使用的 JSON 载荷。
type Update{{ .Pascal }}Request struct {
	Name        string ` + "`json:\"name\" binding:\"required\"`" + `
	Description string ` + "`json:\"description\"`" + `
}

// List{{ .Pascal }}Request 是 GET /api/v1/{{ .TableName }} 使用的查询参数载荷。
type List{{ .Pascal }}Request struct {
	Page     int32 ` + "`form:\"page\"`" + `
	PageSize int32 ` + "`form:\"page_size\"`" + `
}
`

const gatewayHandlerTemplate = `package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	{{ .GoPackage }} "{{ .Module }}/api/gen/{{ .Dir }}/v1"
	"{{ .Module }}/internal/gateway/request"
	apperrors "{{ .Module }}/pkg/errors"
	"{{ .Module }}/pkg/httpx"
)

// {{ .Pascal }}Handler 将 {{ .Dir }} HTTP 接口适配到生成的 gRPC client。
type {{ .Pascal }}Handler struct {
	client {{ .GoPackage }}.{{ .Pascal }}ServiceClient
	log    *zap.Logger
}

// New{{ .Pascal }}Handler 将 {{ .Dir }} gRPC client 注入 HTTP handler 方法。
func New{{ .Pascal }}Handler(client {{ .GoPackage }}.{{ .Pascal }}ServiceClient, log *zap.Logger) *{{ .Pascal }}Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &{{ .Pascal }}Handler{
		client: client,
		log:    log,
	}
}

// Create 将 POST /api/v1/{{ .TableName }} 代理到 Create{{ .Pascal }}。
func (h *{{ .Pascal }}Handler) Create(c *gin.Context) {
	var req request.Create{{ .Pascal }}Request
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
	h.log.Info("gateway {{ .Dir }} create proxied", zap.String("request_id", httpx.RequestID(c)), zap.String("aggregate_id", resp.GetId()))
	httpx.Created(c, resp)
}

// Get 将 GET /api/v1/{{ .TableName }}/:id 代理到 Get{{ .Pascal }}。
func (h *{{ .Pascal }}Handler) Get(c *gin.Context) {
	resp, err := h.client.Get{{ .Pascal }}(outgoingContext(c), &{{ .GoPackage }}.Get{{ .Pascal }}Request{Id: c.Param("id")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// List 将 GET /api/v1/{{ .TableName }} 代理到 List{{ .Pascal }}s。
func (h *{{ .Pascal }}Handler) List(c *gin.Context) {
	var req request.List{{ .Pascal }}Request
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

// Update 将 PUT /api/v1/{{ .TableName }}/:id 代理到 Update{{ .Pascal }}。
func (h *{{ .Pascal }}Handler) Update(c *gin.Context) {
	var req request.Update{{ .Pascal }}Request
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
	h.log.Info("gateway {{ .Dir }} update proxied", zap.String("request_id", httpx.RequestID(c)), zap.String("aggregate_id", resp.GetId()))
	httpx.OK(c, resp)
}

// Delete 将 DELETE /api/v1/{{ .TableName }}/:id 代理到 Delete{{ .Pascal }}。
func (h *{{ .Pascal }}Handler) Delete(c *gin.Context) {
	resp, err := h.client.Delete{{ .Pascal }}(outgoingContext(c), &{{ .GoPackage }}.Delete{{ .Pascal }}Request{Id: c.Param("id")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	h.log.Info("gateway {{ .Dir }} delete proxied", zap.String("request_id", httpx.RequestID(c)), zap.String("aggregate_id", c.Param("id")))
	httpx.OK(c, resp)
}
`

const gatewayRoutesTemplate = `package router

import (
	"github.com/gin-gonic/gin"

	"{{ .Module }}/internal/gateway/handler"
)

// register{{ .Pascal }}Routes 在独立业务文件中注册 /api/v1/{{ .TableName }} 接口。
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

	"{{ .Module }}/internal/gateway/client"
	"{{ .Module }}/internal/gateway/handler"
)

// registerAPIRoutes 创建 /api/v1 路由命名空间，再按业务模块分发。
func registerAPIRoutes(r *gin.Engine, clients *client.Clients, log *zap.Logger) {
	api := r.Group("/api")
	v1 := api.Group("/v1")

	register{{ .Pascal }}Routes(v1, handler.New{{ .Pascal }}Handler(clients.{{ .Pascal }}, log))
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
internal/{{ .Dir }}/entity                     # 业务实体、业务错误、Repository 接口
internal/{{ .Dir }}/model                      # 数据库表结构和文档结构
internal/{{ .Dir }}/dto/command.go             # 业务用例入参命令
internal/{{ .Dir }}/dto/{{ .Dir }}.go          # 业务用例出参 DTO 和转换
internal/{{ .Dir }}/service/service.go         # 业务流程编排
internal/{{ .Dir }}/repo                       # Gorm 和 MongoDB 仓储实现
internal/{{ .Dir }}/handler                    # gRPC 入站适配器
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

如果要修改端口，改 ` + "`services.{{ .Dir }}.port`" + ` 即可。如果项目启用了 Nacos，请把本地 ` + "`configs/config.yaml`" + ` 中新增的 ` + "`services.{{ .Dir }}`" + ` 同步到 Nacos 对应配置。

## 基础 CRUD

生成后的服务已经提供 Create/Get/List/Update/Delete 的基础调用链：

~~~text
proto RPC -> handler -> service -> entity.Repository -> repo(Gorm) -> database
~~~

默认启动使用 ` + "`repo/gorm_repository.go`" + `，无需改配置即可运行。命令同时生成 ` + "`repo/mongo_repository.go`" + `，MongoDB 仓储已通过 ` + "`mongox.NewDocumentStore[dbmodel.{{ .Pascal }}Document]`" + ` 接好基础 CRUD；需要切换 MongoDB 时，只替换 ` + "`cmd/{{ .Dir }}/main.go`" + ` 中注入的 repository。

用户可以直接把示例字段 ` + "`Name`" + `、` + "`Description`" + ` 替换成真实业务字段，或者在此基础上新增业务方法。

如果项目包含 Gin gateway，命令也会生成 HTTP 入口：

~~~text
POST   /api/v1/{{ .TableName }}
GET    /api/v1/{{ .TableName }}
GET    /api/v1/{{ .TableName }}/:id
PUT    /api/v1/{{ .TableName }}/:id
DELETE /api/v1/{{ .TableName }}/:id
~~~

gateway client 默认调用 ` + "`services.{{ .Dir }}.target`" + `。如需调整地址，修改 ` + "`configs/config.yaml`" + ` 中的 ` + "`services.{{ .Dir }}.target`" + `。

## 开发顺序

1. 在 ` + "`api/proto/{{ .Dir }}/v1/{{ .ProtoFile }}`" + ` 中定义 RPC、Request、Response。
2. 执行 ` + "`make proto`" + ` 生成 ` + "`api/gen/{{ .Dir }}/v1`" + `。
3. 在 ` + "`internal/{{ .Dir }}/entity`" + ` 补充业务实体、业务错误和 Repository 接口。
4. 在 ` + "`internal/{{ .Dir }}/model`" + ` 补充 Gorm 表结构、MongoDB 文档结构、` + "`TableName()`" + ` 和 ` + "`MongoCollectionName()`" + `。
5. 在 ` + "`internal/{{ .Dir }}/dto/command.go`" + ` 写入参，在 ` + "`dto/{{ .Dir }}.go`" + ` 写出参和转换，在 ` + "`service/service.go`" + ` 编排业务用例。
6. 在 ` + "`internal/{{ .Dir }}/repo`" + ` 实现数据库访问和 entity/model 映射。
7. 在 ` + "`internal/{{ .Dir }}/handler`" + ` 将 gRPC 请求转成业务命令。
8. 在 ` + "`internal/gateway/request`" + `、` + "`handler`" + `、` + "`router`" + ` 调整 HTTP 入参、控制器和路由。

## 每一层怎么写，为什么这么写

| 层级 | 写什么 | 为什么 |
| --- | --- | --- |
| ` + "`api/proto/{{ .Dir }}/v1`" + ` | RPC、Request、Response、` + "`go_package`" + ` | 先稳定外部契约，避免内部模型直接暴露 |
| ` + "`api/gen/{{ .Dir }}/v1`" + ` | ` + "`make proto`" + ` 生成代码 | 保持 proto 与 Go 类型一致，不手写 |
| ` + "`cmd/{{ .Dir }}`" + ` | 配置、日志、数据库、gRPC server 组装 | main 只负责依赖装配，不写业务 |
| ` + "`internal/{{ .Dir }}/entity`" + ` | 业务实体、业务错误、Repository 接口 | 业务核心不依赖 Gin、gRPC、Gorm |
| ` + "`internal/{{ .Dir }}/model`" + ` | Gorm 表结构、MongoDB 文档结构、表名和集合名 | 数据库结构单独维护，不混入查询逻辑 |
| ` + "`internal/{{ .Dir }}/dto/command.go`" + ` | 业务用例入参，例如 ` + "`CreateCommand`" + `、` + "`UpdateCommand`" + ` | handler 只负责组装命令，入参与流程分开 |
| ` + "`internal/{{ .Dir }}/dto/{{ .Dir }}.go`" + ` | 业务用例出参和业务实体转换 | 对外不暴露业务实体和数据库模型 |
| ` + "`internal/{{ .Dir }}/service/service.go`" + ` | 用例编排、事务意图、调用仓储接口 | 表达业务流程，依赖接口而不是数据库实现 |
| ` + "`internal/{{ .Dir }}/repo`" + ` | Gorm/MongoDB/Redis 等实现，entity/model 映射 | 数据库访问集中管理，方便替换和测试 |
| ` + "`internal/{{ .Dir }}/handler`" + ` | gRPC request/response 适配 | 协议转换和错误映射，不写数据库逻辑 |
| ` + "`internal/gateway/request`" + ` | HTTP 入参 DTO | 控制器不堆字段，入参校验更清楚 |
| ` + "`internal/gateway/handler`" + ` | HTTP 控制器 | 只做绑定、调用 gRPC client 和统一响应 |
| ` + "`internal/gateway/router`" + ` | HTTP 路由 | 按版本/业务拆分，避免路由堆在一个文件 |

## entity 层

` + "`entity`" + ` 放业务核心，不引入 Gorm、Gin、gRPC SDK，也不写数据库 tag。

~~~go
package entity

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

仓储接口也放在 ` + "`entity`" + `：

~~~go
package entity

import "context"

type Repository interface {
	Save(ctx context.Context, item *{{ .Pascal }}) error
	FindByID(ctx context.Context, id string) (*{{ .Pascal }}, error)
	List(ctx context.Context, offset int, limit int) ([]*{{ .Pascal }}, int64, error)
	Delete(ctx context.Context, id string) error
}
~~~

这样写的原因是：` + "`service`" + ` 只关心业务需要的能力，不关心底层用 MySQL、PostgreSQL、MongoDB 还是测试 fake。

## model 层：数据库结构

` + "`model`" + ` 只维护数据库结构。Gorm 结构写 ` + "`gorm`" + ` tag 和 ` + "`TableName()`" + `，MongoDB 文档结构写 ` + "`bson`" + ` tag 和 ` + "`MongoCollectionName()`" + `，不写查询逻辑、不放业务错误。

~~~go
package model

import "time"

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

type {{ .Pascal }}Document struct {
	ID          string    ` + "`bson:\"_id\"`" + `
	Name        string    ` + "`bson:\"name\"`" + `
	Description string    ` + "`bson:\"description\"`" + `
	CreatedAt   time.Time ` + "`bson:\"created_at\"`" + `
	UpdatedAt   time.Time ` + "`bson:\"updated_at\"`" + `
}

func ({{ .Pascal }}Document) MongoCollectionName() string {
	return "{{ .TableName }}"
}
~~~

## service 层

` + "`dto`" + ` 和 ` + "`service`" + ` 按职责拆开，避免一个文件同时承担入参、出参和流程编排：

~~~text
internal/{{ .Dir }}/dto/command.go      # CreateCommand、UpdateCommand、ListCommand
internal/{{ .Dir }}/dto/{{ .Dir }}.go   # {{ .Pascal }}DTO、List{{ .Pascal }}DTO、From{{ .Pascal }}
internal/{{ .Dir }}/service/service.go  # Service、NewService、Create/Get/List/Update/Delete
~~~

` + "`service.go`" + ` 只编排业务流程，只依赖 ` + "`entity.Repository`" + `。

~~~go
package service

import (
	"go.uber.org/zap"

	"{{ .Module }}/internal/{{ .Dir }}/entity"
)

type Service struct {
	repo entity.Repository
	log  *zap.Logger
}

func NewService(repo entity.Repository, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	return &Service{repo: repo, log: log}
}
~~~

新增业务时先在 ` + "`dto/command.go`" + ` 定义命令对象，例如 ` + "`CreateCommand`" + `，再在 ` + "`service/service.go`" + ` 调用业务实体和仓储接口，最后在 ` + "`dto/{{ .Dir }}.go`" + ` 做出参转换。这样 handler 不会堆业务判断，service 也更容易写单元测试。

## repo 层：数据库在哪里操作，如何操作

数据库操作只放在 ` + "`internal/{{ .Dir }}/repo`" + `。脚手架会同时生成两个仓储实现：

~~~text
internal/{{ .Dir }}/repo/gorm_repository.go   # 默认启用，适合 MySQL/PostgreSQL/SQLite
internal/{{ .Dir }}/repo/mongo_repository.go  # 已封装 DocumentStore，适合文档存储
~~~

启动入口 ` + "`cmd/{{ .Dir }}/main.go`" + ` 默认打开 Gorm 数据库并注入 repo：

~~~go
db, err := database.Open(cfg.Database, cfg.MySQL, cfg.PostgreSQL, log)
repo := {{ .GoIdent }}repo.NewGormRepository(db, log)
svc := {{ .GoIdent }}service.NewService(repo, log)
~~~

Gorm 仓储示例：

~~~go
type GormRepository struct {
	db  *gorm.DB
	log *zap.Logger
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&dbmodel.{{ .Pascal }}Model{})
}
~~~

MongoDB 仓储也已经生成，核心是文档结构体自己声明 collection 名称，然后直接创建公共 ` + "`DocumentStore`" + `：

~~~go
type MongoRepository struct {
	documents *mongox.DocumentStore[dbmodel.{{ .Pascal }}Document]
}

func NewMongoRepository(db *mongo.Database, log *zap.Logger) *MongoRepository {
	return &MongoRepository{
		documents: mongox.NewDocumentStore[dbmodel.{{ .Pascal }}Document](db, log),
	}
}
~~~

切换到 MongoDB 时，在 ` + "`cmd/{{ .Dir }}/main.go`" + ` 中用配置文件创建 Mongo client 和 database，然后把 repo 注入改成：

~~~go
mongoClient, err := mongox.NewClient(cfg.MongoDB.MongoxConfig())
if err != nil {
	log.Fatal("create mongodb client failed", zap.Error(err))
}
defer mongoClient.Disconnect(context.Background())

mongoDB := mongox.Database(mongoClient, cfg.MongoDB.Database)
repo := {{ .GoIdent }}repo.NewMongoRepository(mongoDB, log)
svc := {{ .GoIdent }}service.NewService(repo, log)
~~~

数据库操作规则：

- ` + "`handler`" + ` 不直接操作数据库。
- ` + "`service`" + ` 不直接使用 ` + "`*gorm.DB`" + `。
- ` + "`entity`" + ` 不写 Gorm/BSON tag，避免业务模型和数据库实现耦合。
- ` + "`model`" + ` 只写数据库结构、tag、` + "`TableName()`" + ` 和 ` + "`MongoCollectionName()`" + `，不写查询逻辑。
- 查询、分页、事务、锁、索引相关实现都放在 ` + "`repo`" + `。
- 需要事务时，在 repo 层内部使用 ` + "`db.Transaction(func(tx *gorm.DB) error { ... })`" + `。
- 多数据源时保持接口不变，例如 ` + "`GormRepository`" + `、` + "`MongoRepository`" + ` 都实现 ` + "`entity.Repository`" + `。

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

路由按 ` + "`版本/业务/具体接口`" + ` 拆分，当前业务挂在 ` + "`/api/v1/{{ .TableName }}`" + `。gateway client 默认读取 ` + "`services.{{ .Dir }}.target`" + `。
`
