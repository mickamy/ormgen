package gen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
)

// FieldInfo holds parsed metadata for one struct field.
type FieldInfo struct {
	Name       string // Go field name, e.g. "ID"
	Column     string // DB column name from `db:"id"` tag
	GoType     string // Go type as string, e.g. "int", "string", "time.Time"
	PrimaryKey bool   // true if tag contains "primaryKey"
}

// StructInfo holds parsed metadata for the target struct.
type StructInfo struct {
	Name      string      // Go struct name, e.g. "User"
	Package   string      // Package name, e.g. "model"
	Fields    []FieldInfo // Non-skipped db fields
	TableName string      // Set by the caller (from CLI flag)
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

// Parse reads the Go file at path, finds the struct named typeName,
// and returns its StructInfo.
func Parse(filePath string, typeName string) (*StructInfo, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse file: %w", err)
	}

	info := &StructInfo{
		Name:    typeName,
		Package: file.Name.Name,
	}

	var found bool
	ast.Inspect(file, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != typeName {
			return true
		}

		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return false
		}

		found = true
		for _, field := range st.Fields.List {
			fi, skip := parseField(field)
			if skip {
				continue
			}
			info.Fields = append(info.Fields, fi)
		}
		return false
	})

	if !found {
		return nil, fmt.Errorf("type %s not found in %s", typeName, filePath)
	}

	return info, nil
}

func parseField(field *ast.Field) (FieldInfo, bool) {
	if len(field.Names) == 0 {
		return FieldInfo{}, true // embedded field, skip
	}

	name := field.Names[0].Name
	goType := typeToString(field.Type)

	if field.Tag == nil {
		return FieldInfo{}, true // no tag, skip
	}

	tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
	dbTag, ok := tag.Lookup("db")
	if !ok {
		return FieldInfo{}, true // no db tag, skip
	}

	if dbTag == "-" {
		return FieldInfo{}, true // explicitly skipped
	}

	parts := strings.Split(dbTag, ",")
	column := parts[0]
	primaryKey := false
	for _, opt := range parts[1:] {
		if opt == "primaryKey" {
			primaryKey = true
		}
	}

	return FieldInfo{
		Name:       name,
		Column:     column,
		GoType:     goType,
		PrimaryKey: primaryKey,
	}, false
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
