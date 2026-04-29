// Command protogen generates Go protobuf files with platform-neutral path handling.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type protoConfig struct {
	Protoc    string
	ProtoPath string
	ProtoOut  string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg := protoConfigFromEnv()
	files, err := collectProtoFiles(cfg.ProtoPath)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stdout, "No proto files found")
		return nil
	}
	if err := os.MkdirAll(cfg.ProtoOut, 0o755); err != nil {
		return fmt.Errorf("create proto output directory %s: %w", cfg.ProtoOut, err)
	}

	args := []string{
		"--proto_path=" + cfg.ProtoPath,
		"--go_out=" + cfg.ProtoOut,
		"--go_opt=paths=source_relative",
		"--go-grpc_out=" + cfg.ProtoOut,
		"--go-grpc_opt=paths=source_relative",
	}
	args = append(args, files...)

	cmd := exec.Command(cfg.Protoc, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = withGoPluginPath(os.Environ())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("generate proto on %s/%s: %w", runtime.GOOS, runtime.GOARCH, err)
	}
	return nil
}

func protoConfigFromEnv() protoConfig {
	return protoConfig{
		Protoc:    getenvDefault("PROTOC", "protoc"),
		ProtoPath: filepath.Clean(getenvDefault("PROTO_PATH", filepath.Join("api", "proto"))),
		ProtoOut:  filepath.Clean(getenvDefault("PROTO_OUT", filepath.Join("api", "gen"))),
	}
}

func getenvDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func collectProtoFiles(protoPath string) ([]string, error) {
	info, err := os.Stat(protoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat proto path %s: %w", protoPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("proto path %s is not a directory", protoPath)
	}

	files := make([]string, 0)
	err = filepath.WalkDir(protoPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".proto") {
			return nil
		}
		rel, err := filepath.Rel(protoPath, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk proto path %s: %w", protoPath, err)
	}
	sort.Strings(files)
	return files, nil
}

func withGoPluginPath(env []string) []string {
	paths := make([]string, 0, 2)
	if goBin := goEnv("GOBIN"); goBin != "" {
		paths = append(paths, goBin)
	}
	if goPath := goEnv("GOPATH"); goPath != "" {
		paths = append(paths, filepath.Join(goPath, "bin"))
	}
	return prependPath(env, paths...)
}

func goEnv(key string) string {
	output, err := exec.Command("go", "env", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func prependPath(env []string, paths ...string) []string {
	cleaned := cleanPathEntries(paths)
	if len(cleaned) == 0 {
		return env
	}
	separator := string(os.PathListSeparator)
	for i, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if !ok || !strings.EqualFold(key, "PATH") {
			continue
		}
		env[i] = key + "=" + strings.Join(append(cleaned, value), separator)
		return env
	}
	return append(env, "PATH="+strings.Join(cleaned, separator))
}

func cleanPathEntries(paths []string) []string {
	cleaned := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path != "" {
			cleaned = append(cleaned, path)
		}
	}
	return cleaned
}
