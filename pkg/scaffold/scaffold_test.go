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
		SourceDir:  source,
		TargetDir:  target,
		ModulePath: "github.com/acme/demo",
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
		RepoURL:    source,
		TargetDir:  target,
		ModulePath: "github.com/acme/demo",
	})

	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(target, ".git"))
	require.True(t, os.IsNotExist(err))

	mod, err := os.ReadFile(filepath.Join(target, "go.mod"))
	require.NoError(t, err)
	require.Contains(t, string(mod), "module github.com/acme/demo")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}
