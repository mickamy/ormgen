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
		if len(fields) == 0 {
			return true
		}

		infos = append(infos, &StructInfo{
			Name:    ts.Name.Name,
			Package: pkg,
			Fields:  fields,
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
