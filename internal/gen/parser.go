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
}

// RelationInfo holds parsed metadata for a relation field.
type RelationInfo struct {
	FieldName  string // Go field name, e.g. "Posts" or "User"
	TargetType string // Target struct name, e.g. "Post" or "User"
	RelType    string // "has_many", "belongs_to", "has_one", or "many_to_many"
	ForeignKey string // FK column name, e.g. "user_id"
	IsSlice    bool   // true for has_many / many_to_many ([]Post)
	IsPointer  bool   // true for belongs_to / has_one (*User)
	JoinTable  string // many_to_many only: join table name, e.g. "user_tags"
	References string // many_to_many only: target FK in join table, e.g. "tag_id"
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
		relations := parseRelations(st)
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

	// Skip slice and pointer-to-struct fields (likely relations, not columns).
	if isCompositeType(field.Type) {
		return FieldInfo{}, true
	}

	goType := typeToString(field.Type)

	// Defaults: column inferred from field name, ID field is primary key.
	column := naming.CamelToSnake(name)
	primaryKey := name == "ID"

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
				if opt == "primaryKey" {
					primaryKey = true
				}
			}
		}
	}

	return FieldInfo{
		Name:       name,
		Column:     column,
		GoType:     goType,
		PrimaryKey: primaryKey,
	}, false
}

// parseRelations extracts rel-tagged fields from an AST struct type.
func parseRelations(st *ast.StructType) []RelationInfo {
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
		for _, part := range strings.Split(relTag, ",") {
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
		ri.TargetType, ri.IsSlice, ri.IsPointer = extractTargetType(field.Type)

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

// extractTargetType returns the base type name, and whether it's a slice or pointer.
func extractTargetType(expr ast.Expr) (name string, isSlice, isPointer bool) {
	switch t := expr.(type) {
	case *ast.ArrayType:
		if t.Len == nil { // slice
			name, _, _ = extractTargetType(t.Elt)
			return name, true, false
		}
	case *ast.StarExpr:
		name, _, _ = extractTargetType(t.X)
		return name, false, true
	case *ast.Ident:
		return t.Name, false, false
	case *ast.SelectorExpr:
		return t.Sel.Name, false, false
	}
	return "", false, false
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

// isCompositeType returns true for slice, pointer-to-struct, and struct value
// types that should be auto-skipped as DB columns (they are likely relation
// fields). Exported identifiers (e.g. User, Post) are treated as struct types.
// Builtin types (int, string, bool, etc.) are lowercase and pass through.
func isCompositeType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.ArrayType:
		return true // []T or [N]T
	case *ast.Ident:
		// Exported ident in same package = struct type (User, Post, etc.)
		// Builtin types (int, string, bool, ...) are lowercase and won't match.
		return t.IsExported()
	case *ast.StarExpr:
		// *T where T is a struct name (not *string etc.)
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.IsExported()
		}
		// *pkg.Type
		if _, ok := t.X.(*ast.SelectorExpr); ok {
			return true
		}
	}
	return false
}
