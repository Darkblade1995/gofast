package main

import (
	"fmt"
	"os"

	"gofast/internal/codegen"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "generate":
		runGenerate(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runGenerate(args []string) {
	checkOnly := false
	var sourcePath string

	for _, arg := range args {
		if arg == "--check" {
			checkOnly = true
			continue
		}
		sourcePath = arg
	}

	if sourcePath == "" {
		fmt.Fprintln(os.Stderr, "missing source file path")
		printUsage()
		os.Exit(1)
	}

	manifest, err := codegen.LoadManifest(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load manifest: %v\n", err)
		os.Exit(1)
	}

	matches, err := manifest.HashMatches(sourcePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to check file hash: %v\n", err)
		os.Exit(1)
	}

	if checkOnly {
		if matches {
			fmt.Printf("%s: up to date\n", sourcePath)
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "%s: stale, run `gofast generate %s`\n", sourcePath, sourcePath)
		os.Exit(1)
	}

	if matches {
		fmt.Printf("%s: unchanged, skipping\n", sourcePath)
		return
	}

	outPaths, err := codegen.GenerateAndWrite(sourcePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate failed: %v\n", err)
		os.Exit(1)
	}

	if len(outPaths) == 0 {
		fmt.Printf("%s: nothing to generate\n", sourcePath)
		return
	}

	for _, p := range outPaths {
		fmt.Printf("generated: %s\n", p)
	}

	if err := manifest.Update(sourcePath, outPaths); err != nil {
		fmt.Fprintf(os.Stderr, "failed to update manifest: %v\n", err)
		os.Exit(1)
	}

	if err := manifest.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to save manifest: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("usage: gofast generate [--check] <path-to-file.go>")
}
