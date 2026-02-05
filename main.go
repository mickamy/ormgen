package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/jinzhu/inflection"

	"github.com/mickamy/ormgen/internal/gen"
)

var version = "dev"

func main() {
	typeName := flag.String("type", "", "struct type name (required)")
	tableName := flag.String("table", "", "table name (optional; inferred from -type if omitted)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("ormgen", version)
		return
	}

	if *typeName == "" {
		log.Fatal("-type flag is required")
	}

	if *tableName == "" {
		*tableName = inferTableName(*typeName)
	}

	goFile := os.Getenv("GOFILE")
	if goFile == "" {
		log.Fatal("GOFILE environment variable is not set (run via go:generate)")
	}

	info, err := gen.Parse(goFile, *typeName)
	if err != nil {
		log.Fatalf("parse: %v", err)
	}

	info.TableName = *tableName

	src, err := gen.Render(info)
	if err != nil {
		log.Fatalf("render: %v", err)
	}

	outFile := strings.ToLower(*typeName) + "_gen.go"
	outPath := filepath.Join(filepath.Dir(goFile), outFile)

	if err := os.WriteFile(outPath, src, 0o644); err != nil { //nolint:gosec // generated code should be world-readable
		log.Fatalf("write %s: %v", outPath, err)
	}

	fmt.Printf("ormgen: wrote %s\n", outPath)
}

// inferTableName converts a CamelCase type name to a snake_case plural table name.
// e.g. "User" -> "users", "UserProfile" -> "user_profiles"
func inferTableName(typeName string) string {
	snake := camelToSnake(typeName)
	return inflection.Plural(snake)
}

// camelToSnake converts a CamelCase string to snake_case.
func camelToSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
