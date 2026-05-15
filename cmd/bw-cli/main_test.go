package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseGenerateOptionsUsesOfficialRepoForCleanProject(t *testing.T) {
	opts, err := parseGenerateOptions([]string{"my-service", "--module", "github.com/acme/my-service"})

	require.NoError(t, err)
	require.Equal(t, "github.com/acme/my-service", opts.ModulePath)
	require.Equal(t, defaultRepoURL, opts.RepoURL)
	require.Equal(t, defaultBranch, opts.Branch)
	require.False(t, opts.RunTidy)
	require.Equal(t, "my-service", filepath.Base(opts.TargetDir))
}

func TestParseGenerateOptionsKeepsTidyFlag(t *testing.T) {
	opts, err := parseGenerateOptions([]string{"service", "--module", "github.com/acme/service", "--tidy"})

	require.NoError(t, err)
	require.Equal(t, "github.com/acme/service", opts.ModulePath)
	require.Equal(t, defaultRepoURL, opts.RepoURL)
	require.Equal(t, defaultBranch, opts.Branch)
	require.True(t, opts.RunTidy)
	require.Equal(t, "service", filepath.Base(opts.TargetDir))
}

func TestParseServiceOptionsUsesCurrentDirectory(t *testing.T) {
	opts, err := parseServiceOptions([]string{"order", "--port", "9103", "--tidy"})

	require.NoError(t, err)
	require.Equal(t, "order", opts.Name)
	require.Equal(t, 9103, opts.Port)
	require.True(t, opts.RunTidy)
	require.True(t, opts.RunProto)
	require.NotEmpty(t, opts.RootDir)
}

func TestParseServiceOptionsSupportsSkipProto(t *testing.T) {
	opts, err := parseServiceOptions([]string{"comment", "--dir", ".", "--skip-proto"})

	require.NoError(t, err)
	require.Equal(t, "comment", opts.Name)
	require.False(t, opts.RunProto)
}

func TestParseServiceOptionsSupportsTableDrivenFlags(t *testing.T) {
	opts, err := parseServiceOptions([]string{
		"order",
		"--table", "orders",
		"--schema", "configs/services/order.yaml",
		"--yes",
	})

	require.NoError(t, err)
	require.Equal(t, "order", opts.Name)
	require.Equal(t, "orders", opts.Table)
	require.Equal(t, "configs/services/order.yaml", opts.SchemaPath)
	require.True(t, opts.AssumeYes)
}
