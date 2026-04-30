package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/BwCloudWeGo/bw-cli/pkg/scaffold"
)

const (
	defaultRepoURL = "https://github.com/BwCloudWeGo/bw-cli.git"
	defaultBranch  = "main"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "new", "init":
		runGenerate(os.Args[2:], false)
	case "demo":
		runGenerate(os.Args[2:], true)
	case "service", "add-service":
		runService(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func runGenerate(args []string, includeDemo bool) {
	opts, err := parseGenerateOptions(args, includeDemo)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			usage()
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		usage()
		os.Exit(2)
	}
	if err := scaffold.Init(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("scaffold initialized at %s\n", opts.TargetDir)
}

func runService(args []string) {
	opts, err := parseServiceOptions(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			usage()
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		usage()
		os.Exit(2)
	}
	if err := scaffold.AddService(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("service %s initialized at %s\n", opts.Name, opts.RootDir)
}

func parseGenerateOptions(args []string, includeDemo bool) (scaffold.InitOptions, error) {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	modulePath := fs.String("module", "", "go module path, for example github.com/acme/demo")
	sourceDir := fs.String("source", "", "local scaffold source directory")
	repoURL := fs.String("repo", defaultRepoURL, "git repository url of the scaffold")
	branch := fs.String("branch", defaultBranch, "git branch or tag used with --repo")
	tidy := fs.Bool("tidy", false, "run go mod tidy after generating project")

	targetArg, parseArgs := splitTargetArg(args)
	if err := fs.Parse(parseArgs); err != nil {
		return scaffold.InitOptions{}, err
	}
	if targetArg == "" && fs.NArg() == 1 {
		targetArg = fs.Arg(0)
	}
	if targetArg == "" {
		return scaffold.InitOptions{}, fmt.Errorf("project target directory is required")
	}
	target, err := filepath.Abs(targetArg)
	if err != nil {
		return scaffold.InitOptions{}, err
	}
	source := ""
	if *sourceDir != "" {
		source, err = filepath.Abs(*sourceDir)
		if err != nil {
			return scaffold.InitOptions{}, err
		}
	}

	repo := *repoURL
	if source != "" {
		repo = ""
	}
	return scaffold.InitOptions{
		SourceDir:   source,
		TargetDir:   target,
		ModulePath:  *modulePath,
		RepoURL:     repo,
		Branch:      *branch,
		RunTidy:     *tidy,
		IncludeDemo: includeDemo,
	}, nil
}

func parseServiceOptions(args []string) (scaffold.ServiceOptions, error) {
	fs := flag.NewFlagSet("service", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	rootDir := fs.String("dir", ".", "project root directory, defaults to current directory")
	port := fs.Int("port", 9100, "default gRPC port for the generated service")
	skipProto := fs.Bool("skip-proto", false, "skip proto code generation after writing files")
	tidy := fs.Bool("tidy", false, "run go mod tidy after generating service")

	serviceArg, parseArgs := splitTargetArg(args)
	if err := fs.Parse(parseArgs); err != nil {
		return scaffold.ServiceOptions{}, err
	}
	if serviceArg == "" && fs.NArg() == 1 {
		serviceArg = fs.Arg(0)
	}
	if serviceArg == "" {
		return scaffold.ServiceOptions{}, fmt.Errorf("service name is required")
	}
	root, err := filepath.Abs(*rootDir)
	if err != nil {
		return scaffold.ServiceOptions{}, err
	}
	return scaffold.ServiceOptions{
		RootDir:  root,
		Name:     serviceArg,
		Port:     *port,
		RunProto: !*skipProto,
		RunTidy:  *tidy,
	}, nil
}

func splitTargetArg(args []string) (string, []string) {
	if len(args) == 0 || args[0] == "" || args[0][0] == '-' {
		return "", args
	}
	return args[0], args[1:]
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  bw-cli new <target-dir> --module github.com/acme/demo [--tidy]")
	fmt.Fprintln(os.Stderr, "  bw-cli demo <target-dir> --module github.com/acme/demo [--tidy]")
	fmt.Fprintln(os.Stderr, "  bw-cli new <target-dir> --module github.com/acme/demo --source . [--tidy]")
	fmt.Fprintln(os.Stderr, "  bw-cli service <service-name> [--dir .] [--port 9100] [--tidy]")
}
