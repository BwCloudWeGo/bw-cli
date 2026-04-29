package scaffold_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/scaffold"
)

func TestInitCopiesSourceAndRewritesModule(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source")
	target := filepath.Join(tmp, "target")
	require.NoError(t, os.MkdirAll(filepath.Join(source, "pkg", "demo"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(source, "api", "proto", "demo", "v1"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(source, "api", "gen", "demo", "v1"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(source, "logs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(source, "go.mod"), []byte("module old/module\n\ngo 1.25.0\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(source, "pkg", "demo", "demo.go"), []byte("package demo\n\nimport _ \"old/module/pkg/logger\"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(source, "api", "proto", "demo", "v1", "demo.proto"), []byte("option go_package = \"old/module/api/gen/demo/v1;demov1\";\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(source, "api", "gen", "demo", "v1", "demo.pb.go"), []byte("package demov1\n\nvar raw = []byte(\"old/module/api/gen/demo/v1\")\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(source, "logs", "skip.log"), []byte("skip"), 0o644))

	err := scaffold.Init(scaffold.InitOptions{
		SourceDir:   source,
		TargetDir:   target,
		ModulePath:  "github.com/acme/demo",
		IncludeDemo: true,
	})

	require.NoError(t, err)
	mod, err := os.ReadFile(filepath.Join(target, "go.mod"))
	require.NoError(t, err)
	require.Contains(t, string(mod), "module github.com/acme/demo")

	code, err := os.ReadFile(filepath.Join(target, "pkg", "demo", "demo.go"))
	require.NoError(t, err)
	require.Contains(t, string(code), "github.com/acme/demo/pkg/logger")

	proto, err := os.ReadFile(filepath.Join(target, "api", "proto", "demo", "v1", "demo.proto"))
	require.NoError(t, err)
	require.Contains(t, string(proto), "github.com/acme/demo/api/gen/demo/v1")

	generated, err := os.ReadFile(filepath.Join(target, "api", "gen", "demo", "v1", "demo.pb.go"))
	require.NoError(t, err)
	require.Contains(t, string(generated), "old/module/api/gen/demo/v1")

	_, err = os.Stat(filepath.Join(target, "logs", "skip.log"))
	require.True(t, os.IsNotExist(err))
}

func TestInitClonesRepositoryWithoutGitMetadata(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for clone-based scaffold test")
	}

	tmp := t.TempDir()
	source := filepath.Join(tmp, "source")
	target := filepath.Join(tmp, "target")
	require.NoError(t, os.MkdirAll(filepath.Join(source, "pkg", "demo"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(source, "go.mod"), []byte("module old/module\n\ngo 1.25.0\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(source, "pkg", "demo", "demo.go"), []byte("package demo\n\nimport _ \"old/module/pkg/logger\"\n"), 0o644))

	runGit(t, source, "init")
	runGit(t, source, "config", "user.email", "test@example.com")
	runGit(t, source, "config", "user.name", "Test User")
	runGit(t, source, "add", ".")
	runGit(t, source, "commit", "-m", "init")

	err := scaffold.Init(scaffold.InitOptions{
		RepoURL:     source,
		TargetDir:   target,
		ModulePath:  "github.com/acme/demo",
		IncludeDemo: true,
	})

	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(target, ".git"))
	require.True(t, os.IsNotExist(err))

	mod, err := os.ReadFile(filepath.Join(target, "go.mod"))
	require.NoError(t, err)
	require.Contains(t, string(mod), "module github.com/acme/demo")
}

func TestInitWithoutDemoRemovesDemoServicesAndWritesCleanGateway(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source")
	target := filepath.Join(tmp, "target")
	writeMinimalScaffold(t, source)

	err := scaffold.Init(scaffold.InitOptions{
		SourceDir:   source,
		TargetDir:   target,
		ModulePath:  "github.com/acme/clean",
		IncludeDemo: false,
	})

	require.NoError(t, err)
	requireNoPath(t, filepath.Join(target, "cmd", "user"))
	requireNoPath(t, filepath.Join(target, "cmd", "note"))
	requireNoPath(t, filepath.Join(target, "cmd", "bw-cli"))
	requireNoPath(t, filepath.Join(target, "internal", "user"))
	requireNoPath(t, filepath.Join(target, "internal", "note"))
	requireNoPath(t, filepath.Join(target, "internal", "content"))
	requireNoPath(t, filepath.Join(target, "pkg", "scaffold"))
	requireNoPath(t, filepath.Join(target, "internal", "gateway", "client"))
	requireNoPath(t, filepath.Join(target, "api", "proto", "user"))
	requireNoPath(t, filepath.Join(target, "api", "proto", "note"))
	requireNoPath(t, filepath.Join(target, "api", "proto", "content"))
	requireNoPath(t, filepath.Join(target, "api", "gen", "user"))
	requireNoPath(t, filepath.Join(target, "api", "gen", "note"))
	requireNoPath(t, filepath.Join(target, "api", "gen", "content"))
	requireNoPath(t, filepath.Join(target, "docs", "superpowers"))

	gatewayMain := readString(t, filepath.Join(target, "cmd", "gateway", "main.go"))
	require.Contains(t, gatewayMain, "github.com/acme/clean/internal/gateway/router")
	require.NotContains(t, gatewayMain, "internal/gateway/client")
	require.Contains(t, gatewayMain, "net.Listen(\"tcp\", addr)")
	require.Contains(t, gatewayMain, "server.Serve(listener)")
	require.Contains(t, gatewayMain, "printStartupSummary(cfg, addr)")
	require.Contains(t, gatewayMain, "[Gateway Start Failed]")
	require.Contains(t, gatewayMain, "service: %s")
	require.Contains(t, gatewayMain, "health: %s/healthz")

	routerFile := readString(t, filepath.Join(target, "internal", "gateway", "router", "router.go"))
	require.Contains(t, routerFile, "func New(log *zap.Logger, middlewareCfg config.MiddlewareConfig) *gin.Engine")
	require.NotContains(t, routerFile, "internal/gateway/client")

	v1File := readString(t, filepath.Join(target, "internal", "gateway", "router", "v1.go"))
	require.Contains(t, v1File, "api.Group(\"/v1\")")
	require.NotContains(t, v1File, "registerUserRoutes")
	require.NotContains(t, v1File, "registerNoteRoutes")

	makefile := readString(t, filepath.Join(target, "Makefile"))
	require.Contains(t, makefile, "$(GO) run ./tools/protogen")
	require.NotContains(t, makefile, "if [")
	require.NotContains(t, makefile, "find .")
	require.NotContains(t, makefile, "sed ")
	require.NotContains(t, makefile, "PATH=\"")
	require.NotContains(t, makefile, "run-user")
	require.NotContains(t, makefile, "run-note")
	require.FileExists(t, filepath.Join(target, "tools", "protogen", "main.go"))
	protogen := readString(t, filepath.Join(target, "tools", "protogen", "main.go"))
	require.Contains(t, protogen, "No proto files found")

	cfg := readString(t, filepath.Join(target, "configs", "config.yaml"))
	require.NotContains(t, cfg, "user_service_name")
	require.NotContains(t, cfg, "note_service_name")

	readme := readString(t, filepath.Join(target, "README.md"))
	require.Contains(t, readme, "github.com/acme/clean")
	require.Contains(t, readme, "[Gateway Started]")
	require.Contains(t, readme, "health: http://127.0.0.1:8080/healthz")
	require.NotContains(t, readme, "user-service")
	require.NotContains(t, readme, "note-service")

	toolkit := readString(t, filepath.Join(target, "docs", "toolkit.md"))
	require.Contains(t, toolkit, "github.com/acme/clean")
	require.NotContains(t, toolkit, "cmd/bw-cli")
	require.NotContains(t, toolkit, "pkg/scaffold")

	mongodb := readString(t, filepath.Join(target, "docs", "mongodb.md"))
	require.Contains(t, mongodb, "github.com/acme/clean")
	require.NotContains(t, mongodb, "cmd/bw-cli")
	require.NotContains(t, mongodb, "internal/note")
	require.NotContains(t, mongodb, "note-service")
}

func TestInitWithDemoKeepsDemoServices(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source")
	target := filepath.Join(tmp, "target")
	writeMinimalScaffold(t, source)

	err := scaffold.Init(scaffold.InitOptions{
		SourceDir:   source,
		TargetDir:   target,
		ModulePath:  "github.com/acme/demo",
		IncludeDemo: true,
	})

	require.NoError(t, err)
	require.FileExists(t, filepath.Join(target, "cmd", "user", "main.go"))
	require.FileExists(t, filepath.Join(target, "cmd", "note", "main.go"))
	requireNoPath(t, filepath.Join(target, "cmd", "bw-cli"))
	require.FileExists(t, filepath.Join(target, "internal", "user", "model", "user.go"))
	require.FileExists(t, filepath.Join(target, "internal", "note", "model", "note.go"))
	requireNoPath(t, filepath.Join(target, "pkg", "scaffold"))
	require.FileExists(t, filepath.Join(target, "api", "proto", "user", "v1", "user.proto"))
	require.FileExists(t, filepath.Join(target, "api", "proto", "note", "v1", "content.proto"))

	readme := readString(t, filepath.Join(target, "README.md"))
	require.Contains(t, readme, "bw-cli demo")
	require.Contains(t, readme, "github.com/acme/demo")
	require.NotContains(t, readme, "cmd/bw-cli")

	toolkit := readString(t, filepath.Join(target, "docs", "toolkit.md"))
	require.Contains(t, toolkit, "github.com/acme/demo")
	require.NotContains(t, toolkit, "cmd/bw-cli")
	require.NotContains(t, toolkit, "pkg/scaffold")
}

func writeMinimalScaffold(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"go.mod": "module old/module\n\ngo 1.25.0\n",
		"Makefile": `proto:
	protoc user/v1/user.proto note/v1/content.proto
run-user:
	go run ./cmd/user
run-note:
	go run ./cmd/note
run-gateway:
	go run ./cmd/gateway
`,
		"README.md":                              "old/module demo with user-service, note-service, and cmd/bw-cli\n",
		"docs/toolkit.md":                        "old/module toolkit with cmd/bw-cli and pkg/scaffold\n",
		"docs/mongodb.md":                        "old/module mongodb with internal/note and note-service\n",
		"docs/superpowers/plans/stale-plan.md":   "old note-service plan\n",
		"docs/superpowers/specs/stale-design.md": "old note-service design\n",
		"configs/config.yaml": `app:
  name: xiaolanshu
  gateway_service_name: gateway
  user_service_name: user-service
  note_service_name: note-service
`,
		"cmd/gateway/main.go":                      "package main\n\nimport _ \"old/module/internal/gateway/client\"\n",
		"cmd/bw-cli/main.go":                       "package main\n",
		"cmd/user/main.go":                         "package main\n",
		"cmd/note/main.go":                         "package main\n",
		"internal/gateway/client/clients.go":       "package client\n",
		"internal/gateway/handler/common.go":       "package handler\n",
		"internal/gateway/handler/user_handler.go": "package handler\n",
		"internal/gateway/handler/note_handler.go": "package handler\n",
		"internal/gateway/request/user_request.go": "package request\n",
		"internal/gateway/request/note_request.go": "package request\n",
		"internal/gateway/router/router.go":        "package router\n\nimport _ \"old/module/internal/gateway/client\"\n",
		"internal/gateway/router/v1.go":            "package router\n\nfunc registerAPIRoutes() { registerUserRoutes(); registerNoteRoutes() }\n",
		"internal/gateway/router/user_routes.go":   "package router\n\nfunc registerUserRoutes() {}\n",
		"internal/gateway/router/note_routes.go":   "package router\n\nfunc registerNoteRoutes() {}\n",
		"internal/gateway/router/router_test.go":   "package router\n",
		"internal/user/model/user.go":              "package model\n",
		"internal/note/model/note.go":              "package model\n",
		"internal/content/model/content.go":        "package model\n",
		"pkg/scaffold/scaffold.go":                 "package scaffold\n",
		"tools/protogen/main.go":                   "package main\n\nconst noProtoMessage = \"No proto files found\"\n",
		"api/proto/user/v1/user.proto":             "option go_package = \"old/module/api/gen/user/v1;userv1\";\n",
		"api/proto/note/v1/content.proto":          "option go_package = \"old/module/api/gen/note/v1;notev1\";\n",
		"api/proto/content/v1/content.proto":       "option go_package = \"old/module/api/gen/content/v1;contentv1\";\n",
		"api/gen/user/v1/user.pb.go":               "package userv1\n",
		"api/gen/note/v1/note.pb.go":               "package notev1\n",
		"api/gen/content/v1/content.pb.go":         "package contentv1\n",
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

func requireNoPath(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	require.True(t, os.IsNotExist(err), "expected %s to be removed", path)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}
