package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/nacosx"
)

func TestLoadUsesConfigYAMLWhenNacosDisabled(t *testing.T) {
	restore := replaceRemoteConfigLoader(t, func(nacosx.Config) (string, error) {
		t.Fatal("remote config loader should not be called when nacos is disabled")
		return "", nil
	})
	defer restore()

	dir := t.TempDir()
	configPath := writeFile(t, dir, "config.yaml", `
http:
  port: 8181
`)
	writeFile(t, dir, "nacos.yaml", `
enabled: false
`)

	cfg, err := Load(configPath)

	require.NoError(t, err)
	require.Equal(t, 8181, cfg.HTTP.Port)
	require.Equal(t, SourceLocal, cfg.Source)
	require.False(t, cfg.UsingNacos())
}

func TestLoadUsesDefaultConfigPathWhenNacosDisabled(t *testing.T) {
	restore := replaceRemoteConfigLoader(t, func(nacosx.Config) (string, error) {
		t.Fatal("remote config loader should not be called when nacos is disabled")
		return "", nil
	})
	defer restore()

	dir := t.TempDir()
	previousDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer func() {
		require.NoError(t, os.Chdir(previousDir))
	}()

	require.NoError(t, os.MkdirAll("configs", 0o755))
	writeFile(t, filepath.Join(dir, "configs"), "config.yaml", `
http:
  port: 8181
`)
	writeFile(t, filepath.Join(dir, "configs"), "nacos.yaml", `
enabled: false
`)

	cfg, err := Load("")

	require.NoError(t, err)
	require.Equal(t, 8181, cfg.HTTP.Port)
	require.Equal(t, SourceLocal, cfg.Source)
}

func TestLoadReturnsErrorWhenNacosEnabledAndRemoteConfigFails(t *testing.T) {
	restore := replaceRemoteConfigLoader(t, func(nacosx.Config) (string, error) {
		return "", errors.New("nacos unavailable")
	})
	defer restore()

	dir := t.TempDir()
	configPath := writeFile(t, dir, "config.yaml", `
http:
  port: 8181
`)
	writeFile(t, dir, "nacos.yaml", `
enabled: true
fail_fast: true
`)

	cfg, err := Load(configPath)
	require.Nil(t, cfg)
	require.ErrorContains(t, err, "load nacos config")
}

func TestLoadUsesRemoteNacosYAMLAsApplicationConfigWithoutEnvOverrides(t *testing.T) {
	t.Setenv("HTTP_PORT", "10001")
	restore := replaceRemoteConfigLoader(t, func(cfg nacosx.Config) (string, error) {
		require.Equal(t, "xiaolanshu.yaml", cfg.DataID)
		require.Equal(t, "DEFAULT_GROUP", cfg.Group)
		return `
http:
  port: 9191
middleware:
  jwt:
    secret: remote-secret
`, nil
	})
	defer restore()

	dir := t.TempDir()
	configPath := writeFile(t, dir, "config.yaml", `
http:
  port: 8181
middleware:
  jwt:
    secret: local-secret
`)
	writeFile(t, dir, "nacos.yaml", `
enabled: true
data_id: xiaolanshu.yaml
group: DEFAULT_GROUP
`)

	cfg, err := Load(configPath)
	require.NoError(t, err)
	require.Equal(t, 9191, cfg.HTTP.Port)
	require.Equal(t, "remote-secret", cfg.Middleware.JWT.Secret)
	require.Equal(t, SourceNacos, cfg.Source)
	require.True(t, cfg.UsingNacos())
}

func TestLoadUsesNacosYAMLNextToExplicitConfigPath(t *testing.T) {
	restore := replaceRemoteConfigLoader(t, func(cfg nacosx.Config) (string, error) {
		require.Equal(t, "custom.yaml", cfg.DataID)
		return `
http:
  port: 9191
`, nil
	})
	defer restore()

	dir := t.TempDir()
	configPath := writeFile(t, dir, "local.yaml", `
http:
  port: 8181
`)
	writeFile(t, dir, "nacos.yaml", `
enabled: true
data_id: custom.yaml
`)

	cfg, err := Load(configPath)
	require.NoError(t, err)
	require.Equal(t, 9191, cfg.HTTP.Port)
}

func TestPrintSourceNoticeOnlyWritesNacosConfigDetails(t *testing.T) {
	var local bytes.Buffer
	PrintSourceNotice(&Config{Source: SourceLocal}, &local)
	require.Empty(t, local.String())

	var remote bytes.Buffer
	PrintSourceNotice(&Config{
		Source: SourceNacos,
		Nacos: nacosx.Config{
			ServerAddr:  "127.0.0.1",
			ServerPort:  8848,
			NamespaceID: "public",
			Group:       "DEFAULT_GROUP",
			DataID:      "xiaolanshu.yaml",
		},
	}, &remote)

	output := remote.String()
	require.Contains(t, output, "source: nacos")
	require.Contains(t, output, "server: 127.0.0.1:8848")
	require.Contains(t, output, "namespace_id: public")
	require.Contains(t, output, "group: DEFAULT_GROUP")
	require.Contains(t, output, "data_id: xiaolanshu.yaml")
}

func replaceRemoteConfigLoader(t *testing.T, loader func(nacosx.Config) (string, error)) func() {
	t.Helper()
	previous := remoteConfigLoader
	remoteConfigLoader = loader
	return func() {
		remoteConfigLoader = previous
	}
}

func writeFile(t *testing.T, dir string, name string, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}
