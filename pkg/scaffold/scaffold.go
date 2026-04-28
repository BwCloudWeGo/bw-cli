package scaffold

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// InitOptions controls how bw-cli generates a new project from this scaffold.
type InitOptions struct {
	SourceDir  string
	TargetDir  string
	ModulePath string
	RepoURL    string
	Branch     string
	RunTidy    bool
}

// Init copies or clones the scaffold, then rewrites module paths for the target project.
func Init(opts InitOptions) error {
	if opts.TargetDir == "" {
		return errors.New("target dir is required")
	}
	if opts.ModulePath == "" {
		return errors.New("module path is required")
	}

	if opts.RepoURL != "" {
		if err := clone(opts); err != nil {
			return err
		}
	} else {
		if opts.SourceDir == "" {
			return errors.New("source dir or repo url is required")
		}
		if err := copyDir(opts.SourceDir, opts.TargetDir); err != nil {
			return err
		}
	}

	oldModule, err := readModule(filepath.Join(opts.TargetDir, "go.mod"))
	if err != nil {
		return err
	}
	if err := rewriteModule(opts.TargetDir, oldModule, opts.ModulePath); err != nil {
		return err
	}
	if opts.RunTidy {
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = opts.TargetDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("go mod tidy: %w", err)
		}
	}
	return nil
}

func clone(opts InitOptions) error {
	args := []string{"clone", "--depth", "1"}
	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}
	args = append(args, opts.RepoURL, opts.TargetDir)
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	// Generated projects should not inherit the scaffold repository remote.
	return os.RemoveAll(filepath.Join(opts.TargetDir, ".git"))
}

func copyDir(source string, target string) error {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source %s is not a directory", source)
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(target, 0o755)
		}
		if shouldSkip(rel, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		return copyFile(path, dest)
	})
}

func shouldSkip(rel string, entry os.DirEntry) bool {
	name := entry.Name()
	if name == ".git" || name == ".idea" || name == "data" || name == "logs" || name == "tmp" || name == ".DS_Store" {
		return true
	}
	if strings.HasSuffix(name, ".log") {
		return true
	}
	return false
}

func copyFile(source string, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func readModule(goModPath string) (string, error) {
	file, err := os.Open(goModPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", errors.New("module directive not found")
}

func rewriteModule(root string, oldModule string, newModule string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !shouldRewrite(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		updated := strings.ReplaceAll(string(data), oldModule, newModule)
		return os.WriteFile(path, []byte(updated), 0o644)
	})
}

func shouldRewrite(path string) bool {
	if strings.HasSuffix(path, ".pb.go") {
		return false
	}
	switch filepath.Ext(path) {
	case ".go", ".mod", ".md", ".yaml", ".yml", ".proto":
		return true
	default:
		return false
	}
}
