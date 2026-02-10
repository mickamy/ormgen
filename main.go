package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jinzhu/inflection"

	"github.com/mickamy/ormgen/internal/gen"
	"github.com/mickamy/ormgen/internal/naming"
)

var version = "dev"

func main() {
	source := flag.String("source", "", "source file path (required)")
	destination := flag.String("destination", "", "output directory (default: same as source)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("ormgen", version)
		return
	}

	if *source == "" {
		log.Fatal("-source flag is required")
	}

	infos, err := gen.Parse(*source)
	if err != nil {
		log.Fatalf("parse: %v", err)
	}

	if len(infos) == 0 {
		log.Fatalf("no structs with db tags found in %s", *source)
	}

	for _, info := range infos {
		info.TableName = inferTableName(info.Name)
	}

	var opt gen.RenderOption
	outDir := filepath.Dir(*source)

	if *destination != "" {
		outDir = *destination
		opt.DestPkg = filepath.Base(*destination)
		importPath, err := resolveImportPath(filepath.Dir(*source))
		if err != nil {
			log.Fatalf("resolve import path: %v", err)
		}
		opt.SourceImport = importPath
	}

	src, err := gen.RenderFile(infos, opt)
	if err != nil {
		log.Fatalf("render: %v", err)
	}

	base := strings.TrimSuffix(filepath.Base(*source), ".go")
	outFile := base + "_query_gen.go"
	outPath := filepath.Join(outDir, outFile)

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", filepath.Dir(outPath), err)
	}

	if err := os.WriteFile(outPath, src, 0o644); err != nil { //nolint:gosec // generated code should be world-readable
		log.Fatalf("write %s: %v", outPath, err)
	}

	fmt.Printf("ormgen: wrote %s\n", outPath)
}

// resolveImportPath returns the Go import path for the package in dir.
func resolveImportPath(dir string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "go", "list", "-json", ".")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go list: %w", err)
	}

	var pkg struct {
		ImportPath string `json:"ImportPath"` //nolint:tagliatelle // go list -json uses PascalCase
	}
	if err := json.Unmarshal(out, &pkg); err != nil {
		return "", fmt.Errorf("parse go list output: %w", err)
	}
	return pkg.ImportPath, nil
}

// inferTableName converts a CamelCase type name to a snake_case plural table name.
// e.g. "User" -> "users", "UserProfile" -> "user_profiles"
func inferTableName(typeName string) string {
	snake := naming.CamelToSnake(typeName)
	return inflection.Plural(snake)
}
