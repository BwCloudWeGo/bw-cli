package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCollectProtoFilesReturnsSlashSeparatedRelativePaths(t *testing.T) {
	root := t.TempDir()
	protoRoot := filepath.Join(root, "api", "proto")
	require.NoError(t, os.MkdirAll(filepath.Join(protoRoot, "user", "v1"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(protoRoot, "note", "v1"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(protoRoot, "user", "v1", "user.proto"), []byte("syntax = \"proto3\";"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(protoRoot, "note", "v1", "content.proto"), []byte("syntax = \"proto3\";"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(protoRoot, "README.md"), []byte("skip"), 0o644))

	files, err := collectProtoFiles(protoRoot)

	require.NoError(t, err)
	require.Equal(t, []string{"note/v1/content.proto", "user/v1/user.proto"}, files)
}

func TestCollectProtoFilesAllowsMissingProtoDirectory(t *testing.T) {
	files, err := collectProtoFiles(filepath.Join(t.TempDir(), "api", "proto"))

	require.NoError(t, err)
	require.Empty(t, files)
}

func TestPrependPathUsesCurrentOSPathSeparator(t *testing.T) {
	got := prependPath([]string{"Path=base"}, "first", "second")

	require.Len(t, got, 1)
	require.Equal(t, "Path=first"+string(os.PathListSeparator)+"second"+string(os.PathListSeparator)+"base", got[0])
}

func TestPrependPathAddsPathWhenMissing(t *testing.T) {
	got := prependPath([]string{"OTHER=value"}, "bin")

	require.Len(t, got, 2)
	require.True(t, strings.HasPrefix(got[1], "PATH=bin"))
}
