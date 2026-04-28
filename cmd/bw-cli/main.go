package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BwCloudWeGo/bw-cli/pkg/scaffold"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "new", "init":
		runNew(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func runNew(args []string) {
	fs := flag.NewFlagSet("new", flag.ExitOnError)
	modulePath := fs.String("module", "", "go module path, for example github.com/acme/demo")
	sourceDir := fs.String("source", "", "local scaffold source directory")
	repoURL := fs.String("repo", "", "git repository url of the scaffold")
	branch := fs.String("branch", "", "git branch or tag used with --repo")
	tidy := fs.Bool("tidy", false, "run go mod tidy after generating project")

	targetArg := ""
	parseArgs := args
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		targetArg = args[0]
		parseArgs = args[1:]
	}
	_ = fs.Parse(parseArgs)
	if targetArg == "" && fs.NArg() == 1 {
		targetArg = fs.Arg(0)
	}
	if targetArg == "" {
		fmt.Fprintln(os.Stderr, "project target directory is required")
		fs.Usage()
		os.Exit(2)
	}
	target, err := filepath.Abs(targetArg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	source := ""
	if *sourceDir != "" {
		source, err = filepath.Abs(*sourceDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	if err := scaffold.Init(scaffold.InitOptions{
		SourceDir:  source,
		TargetDir:  target,
		ModulePath: *modulePath,
		RepoURL:    *repoURL,
		Branch:     *branch,
		RunTidy:    *tidy,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("scaffold initialized at %s\n", target)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  bw-cli new <target-dir> --module github.com/acme/demo --source .")
	fmt.Fprintln(os.Stderr, "  bw-cli new <target-dir> --module github.com/acme/demo --repo https://github.com/BwCloudWeGo/bw-cli.git")
}
