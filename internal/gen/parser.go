package gen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"

	"github.com/mickamy/ormgen/internal/naming"
)

// FieldInfo holds parsed metadata for one struct field.
type FieldInfo struct {
	Name       string // Go field name, e.g. "ID"
	Column     string // DB column name from `db:"id"` tag
	GoType     string // Go type as string, e.g. "int", "string", "time.Time"
	PrimaryKey bool   // true if tag contains "primaryKey"
	CreatedAt  bool   // true if this is a createdAt timestamp field
	UpdatedAt  bool   // true if this is an updatedAt timestamp field
}

// RelationInfo holds parsed metadata for a relation field.
type RelationInfo struct {
	FieldName        string // Go field name, e.g. "Posts" or "User"
	TargetType       string // Target struct name, e.g. "Post" or "User"
	TargetPkgAlias   string // Source file import alias (e.g. "amodel"). Empty for same-package types.
	TargetImportPath string // Full import path (e.g. "github.com/.../auth/model"). Empty for same-package types.
	RelType          string // "has_many", "belongs_to", "has_one", or "many_to_many"
	ForeignKey       string // FK column name, e.g. "user_id"
	IsSlice          bool   // true for has_many / many_to_many ([]Post)
	IsPointer        bool   // true for belongs_to / has_one (*User)
	JoinTable        string // many_to_many only: join table name, e.g. "user_tags"
	References       string // many_to_many only: target FK in join table, e.g. "tag_id"
}

// StructInfo holds parsed metadata for the target struct.
type StructInfo struct {
	Name      string         // Go struct name, e.g. "User"
	Package   string         // Package name, e.g. "model"
	Fields    []FieldInfo    // Non-skipped db fields
	Relations []RelationInfo // Parsed rel tags
	TableName string         // Set by the caller (from CLI flag)
}

// PrimaryKeyField returns the primary key field, or an error if none or
// multiple are defined.
func (s *StructInfo) PrimaryKeyField() (*FieldInfo, error) {
	var pk *FieldInfo
	for i := range s.Fields {
		if s.Fields[i].PrimaryKey {
			if pk != nil {
				return nil, fmt.Errorf("multiple primary keys: %s and %s", pk.Name, s.Fields[i].Name)
			}
			pk = &s.Fields[i]
		}
	}
	if pk == nil {
		return nil, fmt.Errorf("no primary key defined for %s", s.Name)
	}
	return pk, nil
}

// Parse reads the Go file at path and returns StructInfo for every struct
// that has at least one field with a db tag.
func Parse(filePath string) ([]*StructInfo, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse file: %w", err)
	}

	pkg := file.Name.Name
	importMap := buildImportMap(file)
	var infos []*StructInfo

	ast.Inspect(file, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}

		fields := parseStructFields(st)
		relations := parseRelations(st, importMap)
		if len(fields) == 0 {
			return true
		}

		infos = append(infos, &StructInfo{
			Name:      ts.Name.Name,
			Package:   pkg,
			Fields:    fields,
			Relations: relations,
		})
		return true
	})

	return infos, nil
}

// parseStructFields extracts db-tagged fields from an AST struct type.
func parseStructFields(st *ast.StructType) []FieldInfo {
	fields := make([]FieldInfo, 0, len(st.Fields.List))
	for _, field := range st.Fields.List {
		fi, skip := parseField(field)
		if skip {
			continue
		}
		fields = append(fields, fi)
	}
	return fields
}

func parseField(field *ast.Field) (FieldInfo, bool) {
	if len(field.Names) == 0 {
		return FieldInfo{}, true // embedded field, skip
	}

	name := field.Names[0].Name

	// Skip unexported fields.
	if !field.Names[0].IsExported() {
		return FieldInfo{}, true
	}

	goType := typeToString(field.Type)

	// Defaults: column inferred from field name, ID field is primary key.
	column := naming.CamelToSnake(name)
	primaryKey := name == "ID"
	createdAt := name == "CreatedAt"
	updatedAt := name == "UpdatedAt"

	// Skip relation fields — they are handled by parseRelations.
	if field.Tag != nil {
		tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
		if _, ok := tag.Lookup("rel"); ok {
			return FieldInfo{}, true
		}
	}

	// Skip slice and pointer-to-struct fields (likely relations, not columns).
	// Bare exported idents (e.g. StringArray) are NOT skipped here because
	// they may be custom column types implementing sql.Scanner/driver.Valuer.
	if isCompositeType(field.Type) {
		return FieldInfo{}, true
	}

	// Override with db tag if present.
	if field.Tag != nil {
		tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
		if dbTag, ok := tag.Lookup("db"); ok {
			if dbTag == "-" {
				return FieldInfo{}, true // explicitly skipped
			}
			parts := strings.Split(dbTag, ",")
			if parts[0] != "" {
				column = parts[0]
			}
			for _, opt := range parts[1:] {
				switch opt {
				case "primaryKey":
					primaryKey = true
				case "createdAt":
					createdAt = true
				case "updatedAt":
					updatedAt = true
				}
			}
		}
	}

	return FieldInfo{
		Name:       name,
		Column:     column,
		GoType:     goType,
		PrimaryKey: primaryKey,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}, false
}

// parseRelations extracts rel-tagged fields from an AST struct type.
func parseRelations(st *ast.StructType, importMap map[string]string) []RelationInfo {
	rels := make([]RelationInfo, 0, len(st.Fields.List))
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 || field.Tag == nil {
			continue
		}

		tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
		relTag, ok := tag.Lookup("rel")
		if !ok || relTag == "" {
			continue
		}

		ri := RelationInfo{FieldName: field.Names[0].Name}

		// Parse rel tag: "has_many,foreign_key:user_id" or
		// "many_to_many,join_table:user_tags,foreign_key:user_id,references:tag_id"
		for part := range strings.SplitSeq(relTag, ",") {
			part = strings.TrimSpace(part)
			if k, v, found := strings.Cut(part, ":"); found {
				switch k {
				case "foreign_key":
					ri.ForeignKey = v
				case "join_table":
					ri.JoinTable = v
				case "references":
					ri.References = v
				}
			} else {
				ri.RelType = part
			}
		}

		// Determine target type from field type.
		var pkgAlias string
		ri.TargetType, pkgAlias, ri.IsSlice, ri.IsPointer = extractTargetType(field.Type)

		// Resolve cross-package import info.
		if pkgAlias != "" {
			ri.TargetPkgAlias = pkgAlias
			if path, ok := importMap[pkgAlias]; ok {
				ri.TargetImportPath = path
			}
		}

		if ri.RelType == "" || ri.ForeignKey == "" || ri.TargetType == "" {
			continue
		}
		if ri.RelType == "many_to_many" && (ri.JoinTable == "" || ri.References == "") {
			continue
		}
		rels = append(rels, ri)
	}
	return rels
}

// extractTargetType returns the base type name, package alias (if cross-package),
// and whether it's a slice or pointer.
func extractTargetType(expr ast.Expr) (name, pkgAlias string, isSlice, isPointer bool) {
	switch t := expr.(type) {
	case *ast.ArrayType:
		if t.Len == nil { // slice
			name, pkgAlias, _, _ = extractTargetType(t.Elt)
			return name, pkgAlias, true, false
		}
	case *ast.StarExpr:
		name, pkgAlias, _, _ = extractTargetType(t.X)
		return name, pkgAlias, false, true
	case *ast.Ident:
		return t.Name, "", false, false
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			return t.Sel.Name, x.Name, false, false
		}
		return t.Sel.Name, "", false, false
	}
	return "", "", false, false
}

// buildImportMap returns a map from import alias (or last path segment) to full import path.
func buildImportMap(file *ast.File) map[string]string {
	m := make(map[string]string, len(file.Imports))
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		var alias string
		if imp.Name != nil {
			alias = imp.Name.Name
		} else {
			parts := strings.Split(path, "/")
			alias = parts[len(parts)-1]
		}
		m[alias] = path
	}
	return m
}

func typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return typeToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + typeToString(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + typeToString(t.Elt)
		}
		return fmt.Sprintf("[%s]%s", typeToString(t.Len), typeToString(t.Elt))
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// isCompositeType returns true for types that are likely relation fields
// (not DB columns).
//
// Heuristics:
//   - []T: relation if T is any struct type (same-package or external).
//     []string, []byte etc. are columns.
//   - *T: relation only if T is a same-package exported type.
//     *time.Time, *sql.NullString etc. are columns.
//
// Bare exported idents (e.g. StringArray, JSON) are NOT considered
// composite — they may be custom column types. Relation fields using
// bare types must be marked with a rel tag instead.
func isCompositeType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.ArrayType:
		// []User, []pkg.Model → relation; []string, []byte → column
		return isStructType(t.Elt)
	case *ast.StarExpr:
		// *User → relation; *time.Time → column
		return isSamePackageModel(t.X)
	}
	return false
}

// isStructType returns true if expr refers to any struct-like type,
// including external package types (e.g. amodel.OAuthAccount).
// Used for slice elements where []pkg.Model is a relation.
func isStructType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.IsExported()
	case *ast.StarExpr:
		return isStructType(t.X)
	case *ast.SelectorExpr:
		return true // pkg.Type
	}
	return false
}

// isSamePackageModel returns true only for same-package exported types.
// Used for pointer fields where *time.Time is a column, not a relation.
func isSamePackageModel(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.IsExported()
	case *ast.StarExpr:
		return isSamePackageModel(t.X)
	}
	return false
}
