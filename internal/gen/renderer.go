package gen

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"strings"
	"text/template"
	"unicode"

	"github.com/jinzhu/inflection"

	"github.com/mickamy/ormgen/internal/naming"
)

// RenderOption controls the output of RenderFile.
type RenderOption struct {
	DestPkg      string        // output package name (empty = same as source)
	SourceImport string        // import path for source package (required when DestPkg is set)
	PeerInfos    []*StructInfo // other structs in the same package (for join scan field lookups)
}

// Render generates the Go source code for a single StructInfo.
// The returned bytes are formatted by gofmt.
func Render(info *StructInfo) ([]byte, error) {
	return RenderFile([]*StructInfo{info}, RenderOption{})
}

// RenderFile generates a single Go source file for all given StructInfos.
// The returned bytes are formatted by gofmt.
func RenderFile(infos []*StructInfo, opt RenderOption) ([]byte, error) {
	if len(infos) == 0 {
		return nil, errors.New("no structs to render")
	}

	pkg := opt.DestPkg
	if pkg == "" {
		pkg = infos[0].Package
	}

	typePrefix := ""
	if opt.SourceImport != "" {
		// e.g. "github.com/example/model" → "model."
		parts := strings.Split(opt.SourceImport, "/")
		typePrefix = parts[len(parts)-1] + "."
	}

	// allInfos includes both the structs to render and peer structs from the
	// same package. Peers are used only for join scan field lookups.
	allInfos := infos
	if len(opt.PeerInfos) > 0 {
		allInfos = make([]*StructInfo, 0, len(infos)+len(opt.PeerInfos))
		allInfos = append(allInfos, infos...)
		allInfos = append(allInfos, opt.PeerInfos...)
	}

	structs := make([]templateData, 0, len(infos))
	var allExtraImports []importEntry
	seenImports := make(map[string]bool)

	for _, info := range infos {
		pk, err := info.PrimaryKeyField()
		if err != nil {
			return nil, err
		}

		createdAtFields := filterFields(info.Fields, func(f FieldInfo) bool { return f.CreatedAt })
		updatedAtFields := filterFields(info.Fields, func(f FieldInfo) bool { return f.UpdatedAt })
		hasTimestamps := len(createdAtFields) > 0 || len(updatedAtFields) > 0

		relations, extraImports := buildRelationData(info, pk, typePrefix, opt.SourceImport, opt.DestPkg, allInfos)
		for _, ei := range extraImports {
			if !seenImports[ei.Path] {
				seenImports[ei.Path] = true
				allExtraImports = append(allExtraImports, ei)
			}
		}

		data := templateData{
			TypeName:         typePrefix + info.Name,
			TableName:        info.TableName,
			FactoryName:      naming.SnakeToCamel(info.TableName),
			PK:               pk,
			Fields:           info.Fields,
			ScanFunc:         unexportedName("scan" + info.Name),
			ColValFunc:       unexportedName(info.Name + "ColumnValuePairs"),
			SetPKFunc:        unexportedName("set" + info.Name + "PK"),
			ColumnsVar:       unexportedName(naming.SnakeToCamel(info.TableName) + "Columns"),
			IsIntPK:          isIntType(pk.GoType),
			Relations:        relations,
			SetCreatedAtFunc: unexportedName("set" + info.Name + "CreatedAt"),
			SetUpdatedAtFunc: unexportedName("set" + info.Name + "UpdatedAt"),
			CreatedAtFields:  createdAtFields,
			UpdatedAtFields:  updatedAtFields,
			HasTimestamps:    hasTimestamps,
		}
		structs = append(structs, data)
	}

	hasRelations := false
	fileHasTimestamps := false
	for _, s := range structs {
		if len(s.Relations) > 0 {
			hasRelations = true
		}
		if s.HasTimestamps {
			fileHasTimestamps = true
		}
	}

	fileData := fileTemplateData{
		Package:       pkg,
		SourceImport:  opt.SourceImport,
		HasRelations:  hasRelations,
		HasTimestamps: fileHasTimestamps,
		ExtraImports:  allExtraImports,
		Structs:       structs,
	}

	var buf bytes.Buffer
	if err := fileTmpl.Execute(&buf, fileData); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	src, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("gofmt: %w", err)
	}
	return src, nil
}

type importEntry struct {
	Alias string // empty means the last path segment is used as-is
	Path  string
}

type fileTemplateData struct {
	Package       string
	SourceImport  string
	HasRelations  bool
	HasTimestamps bool
	ExtraImports  []importEntry
	Structs       []templateData
}

type templateData struct {
	TypeName         string
	TableName        string
	FactoryName      string
	PK               *FieldInfo
	Fields           []FieldInfo
	ScanFunc         string
	ColValFunc       string
	SetPKFunc        string
	ColumnsVar       string
	IsIntPK          bool
	Relations        []relationTemplateData
	SetCreatedAtFunc string
	SetUpdatedAtFunc string
	CreatedAtFields  []FieldInfo
	UpdatedAtFields  []FieldInfo
	HasTimestamps    bool
}

type relationTemplateData struct {
	FieldName        string // "Posts"
	ParentType       string // "model.User" or "User" (parent struct type)
	TargetType       string // "model.Post" or "Post"
	TargetFactory    string // "Posts"
	ForeignKey       string // "user_id"
	ForeignKeyField  string // "UserID"
	RelType          string // "has_many", "belongs_to", "has_one", or "many_to_many"
	IsPointer        bool   // true if the source field is a pointer (e.g. *UserEmail)
	PreloaderName    string // "preloadUserPosts"
	KeyType          string // Go type for map key ("int")
	ParentPKField    string // "ID"
	JoinTargetTable  string
	JoinTargetColumn string
	JoinSourceTable  string
	JoinSourceColumn string
	FKIsPointer      bool   // true if the foreign key field is a pointer type (e.g. *string)
	JoinTable        string // many_to_many only: "user_tags"
	References       string // many_to_many only: "tag_id"
	TargetTable      string // many_to_many only: target table name "tags"
	TargetPKColumn   string // many_to_many only: target PK column "id"

	// Join scan support (belongs_to / has_one, same-package only).
	// nil when join scan is not supported (cross-package, has_many, many_to_many).
	JoinScanFields    []FieldInfo // target struct's DB fields
	JoinSelectColumns []string    // target column names for JoinConfig.SelectColumns
	JoinPKGoType      string      // target PK Go type, e.g. "int"
	JoinPKName        string      // target PK Go field name, e.g. "ID"
	JoinNullType      string      // nullable wrapper, e.g. "sql.NullInt64" (pointer only)
	JoinNullField     string      // accessor on NullXxx, e.g. ".Int64" (pointer only)
}

func (d templateData) NonPKFields() []FieldInfo {
	var fields []FieldInfo
	for _, f := range d.Fields {
		if !f.PrimaryKey {
			fields = append(fields, f)
		}
	}
	return fields
}

func (d templateData) CreatedAtColumns() []string {
	cols := make([]string, len(d.CreatedAtFields))
	for i, f := range d.CreatedAtFields {
		cols[i] = f.Column
	}
	return cols
}

var funcMap = template.FuncMap{
	"join": strings.Join,
	"quote": func(s string) string {
		return `"` + s + `"`
	},
	"hasPrefix": strings.HasPrefix,
}

var fileTmpl = template.Must(template.New("gen").Funcs(funcMap).Parse(fileTemplate))

const fileTemplate = `// Code generated by ormgen; DO NOT EDIT.
package {{.Package}}

import (
	{{- if .HasRelations}}
	"context"
	{{- end}}
	"database/sql"
	{{- if .HasTimestamps}}
	"time"
	{{- end}}

	"github.com/mickamy/ormgen/orm"
	{{- if .HasRelations}}
	"github.com/mickamy/ormgen/scope"
	{{- end}}
	{{- if .SourceImport}}
	"{{.SourceImport}}"
	{{- end}}
	{{- range .ExtraImports}}
	{{- if .Alias}}
	{{.Alias}} "{{.Path}}"
	{{- else}}
	"{{.Path}}"
	{{- end}}
	{{- end}}
)
{{range .Structs}}
// {{.FactoryName}} returns a new Query for the {{.TableName}} table.
func {{.FactoryName}}(db orm.Querier) *orm.Query[{{.TypeName}}] {
	{{- if or .Relations .HasTimestamps}}
	q := orm.NewQuery[{{.TypeName}}](
		db, orm.ResolveTableName[{{.TypeName}}]("{{.TableName}}"), {{.ColumnsVar}}, "{{.PK.Column}}",
		{{.ScanFunc}}, {{.ColValFunc}}, {{if .IsIntPK}}{{.SetPKFunc}}{{else}}nil{{end}},
	)
	{{- range .Relations}}
	{{- if ne .RelType "many_to_many"}}
	q.RegisterJoin("{{.FieldName}}", orm.JoinConfig{
		TargetTable: orm.ResolveTableName[{{.TargetType}}]("{{.JoinTargetTable}}"), TargetColumn: "{{.JoinTargetColumn}}",
		SourceTable: orm.ResolveTableName[{{.ParentType}}]("{{.JoinSourceTable}}"), SourceColumn: "{{.JoinSourceColumn}}",
		{{- if .JoinSelectColumns}}
		SelectColumns: []string{ {{- range $i, $c := .JoinSelectColumns}}{{if $i}}, {{end}}{{quote $c}}{{end -}} },
		{{- end}}
	})
	{{- end}}
	q.RegisterPreloader("{{.FieldName}}", {{.PreloaderName}})
	{{- end}}
	{{- if .HasTimestamps}}
	q.RegisterTimestamps(
		{{if .CreatedAtFields}}[]string{ {{- range $i, $c := .CreatedAtColumns}}{{if $i}}, {{end}}{{quote $c}}{{end -}} }{{else}}nil{{end}},
		{{if .CreatedAtFields}}{{.SetCreatedAtFunc}}{{else}}nil{{end}},
		{{if .UpdatedAtFields}}{{.SetUpdatedAtFunc}}{{else}}nil{{end}},
	)
	{{- end}}
	return q
	{{- else}}
	return orm.NewQuery[{{.TypeName}}](
		db, orm.ResolveTableName[{{.TypeName}}]("{{.TableName}}"), {{.ColumnsVar}}, "{{.PK.Column}}",
		{{.ScanFunc}}, {{.ColValFunc}}, {{if .IsIntPK}}{{.SetPKFunc}}{{else}}nil{{end}},
	)
	{{- end}}
}

var {{.ColumnsVar}} = []string{ {{- range $i, $f := .Fields}}{{if $i}}, {{end}}{{quote $f.Column}}{{end -}} }

func {{.ScanFunc}}(rows *sql.Rows) ({{.TypeName}}, error) {
	cols, _ := rows.Columns()
	var v {{.TypeName}}
	{{- range .Relations}}
	{{- if and .JoinScanFields .IsPointer}}
	var joinScan{{.FieldName}}PK {{.JoinNullType}}
	var joinScan{{.FieldName}} {{.TargetType}}
	{{- end}}
	{{- end}}
	dest := make([]any, len(cols))
	for i, col := range cols {
		switch col {
		{{- range .Fields}}
		case {{quote .Column}}:
			dest[i] = &v.{{.Name}}
		{{- end}}
		{{- range $rel := .Relations}}
		{{- range $f := $rel.JoinScanFields}}
		{{- if and $rel.IsPointer $f.PrimaryKey}}
		case "{{$rel.FieldName}}__{{$f.Column}}":
			dest[i] = &joinScan{{$rel.FieldName}}PK
		{{- else if $rel.IsPointer}}
		case "{{$rel.FieldName}}__{{$f.Column}}":
			dest[i] = &joinScan{{$rel.FieldName}}.{{$f.Name}}
		{{- else}}
		case "{{$rel.FieldName}}__{{$f.Column}}":
			dest[i] = &v.{{$rel.FieldName}}.{{$f.Name}}
		{{- end}}
		{{- end}}
		{{- end}}
		default:
			dest[i] = new(any)
		}
	}
	err := rows.Scan(dest...)
	{{- range .Relations}}
	{{- if and .JoinScanFields .IsPointer}}
	if joinScan{{.FieldName}}PK.Valid {
		joinScan{{.FieldName}}.{{.JoinPKName}} = {{.JoinPKGoType}}(joinScan{{.FieldName}}PK{{.JoinNullField}})
		v.{{.FieldName}} = &joinScan{{.FieldName}}
	}
	{{- end}}
	{{- end}}
	return v, err
}

func {{.ColValFunc}}(v *{{.TypeName}}, includesPK bool) ([]string, []any) {
	if includesPK {
		return []string{ {{- range $i, $f := .Fields}}{{if $i}}, {{end}}{{quote $f.Column}}{{end -}} },
			[]any{ {{- range $i, $f := .Fields}}{{if $i}}, {{end}}v.{{$f.Name}}{{end -}} }
	}
	return []string{ {{- range $i, $f := .NonPKFields}}{{if $i}}, {{end}}{{quote $f.Column}}{{end -}} },
		[]any{ {{- range $i, $f := .NonPKFields}}{{if $i}}, {{end}}v.{{$f.Name}}{{end -}} }
}
{{if .IsIntPK}}
func {{.SetPKFunc}}(v *{{.TypeName}}, id int64) {
	v.{{.PK.Name}} = {{.PK.GoType}}(id)
}
{{end}}
{{- if .CreatedAtFields}}
func {{.SetCreatedAtFunc}}(v *{{.TypeName}}, now time.Time) {
	{{- range .CreatedAtFields}}
	{{- if hasPrefix .GoType "*"}}
	if v.{{.Name}} == nil {
		v.{{.Name}} = &now
	}
	{{- else}}
	if v.{{.Name}}.IsZero() {
		v.{{.Name}} = now
	}
	{{- end}}
	{{- end}}
}
{{- end}}
{{- if .UpdatedAtFields}}
func {{.SetUpdatedAtFunc}}(v *{{.TypeName}}, now time.Time) {
	{{- range .UpdatedAtFields}}
	{{- if hasPrefix .GoType "*"}}
	v.{{.Name}} = &now
	{{- else}}
	v.{{.Name}} = now
	{{- end}}
	{{- end}}
}
{{- end}}
{{- range .Relations}}
{{- if eq .RelType "has_many"}}
func {{.PreloaderName}}(ctx context.Context, db orm.Querier, results []{{.ParentType}}) error {
	if len(results) == 0 {
		return nil
	}
	ids := make([]{{.KeyType}}, len(results))
	for i := range results {
		ids[i] = results[i].{{.ParentPKField}}
	}
	related, err := {{.TargetFactory}}(db).Scopes(scope.In("{{.ForeignKey}}", ids)).All(ctx)
	if err != nil {
		return err
	}
	byFK := make(map[{{.KeyType}}][]{{.TargetType}})
	for _, r := range related {
		byFK[r.{{.ForeignKeyField}}] = append(byFK[r.{{.ForeignKeyField}}], r)
	}
	for i := range results {
		results[i].{{.FieldName}} = byFK[results[i].{{.ParentPKField}}]
	}
	return nil
}
{{- else if eq .RelType "has_one"}}
func {{.PreloaderName}}(ctx context.Context, db orm.Querier, results []{{.ParentType}}) error {
	if len(results) == 0 {
		return nil
	}
	ids := make([]{{.KeyType}}, len(results))
	for i := range results {
		ids[i] = results[i].{{.ParentPKField}}
	}
	related, err := {{.TargetFactory}}(db).Scopes(scope.In("{{.ForeignKey}}", ids)).All(ctx)
	if err != nil {
		return err
	}
	{{- if .IsPointer}}
	byFK := make(map[{{.KeyType}}]*{{.TargetType}})
	for i := range related {
		byFK[related[i].{{.ForeignKeyField}}] = &related[i]
	}
	{{- else}}
	byFK := make(map[{{.KeyType}}]{{.TargetType}})
	for _, r := range related {
		byFK[r.{{.ForeignKeyField}}] = r
	}
	{{- end}}
	for i := range results {
		results[i].{{.FieldName}} = byFK[results[i].{{.ParentPKField}}]
	}
	return nil
}
{{- else if eq .RelType "many_to_many"}}
func {{.PreloaderName}}(ctx context.Context, db orm.Querier, results []{{.ParentType}}) error {
	if len(results) == 0 {
		return nil
	}
	ids := make([]{{.KeyType}}, len(results))
	for i := range results {
		ids[i] = results[i].{{.ParentPKField}}
	}
	pairs, err := orm.QueryJoinTable[{{.KeyType}}, {{.KeyType}}]( //nolint:lll
		ctx, db, "{{.JoinTable}}", "{{.ForeignKey}}", "{{.References}}", ids,
	)
	if err != nil {
		return err
	}
	targetIDs := orm.UniqueTargets(pairs)
	related, err := {{.TargetFactory}}(db).Scopes(scope.In("{{.TargetPKColumn}}", targetIDs)).All(ctx)
	if err != nil {
		return err
	}
	byPK := make(map[{{.KeyType}}]{{.TargetType}})
	for _, r := range related {
		byPK[r.ID] = r
	}
	grouped := orm.GroupBySource(pairs)
	for i := range results {
		tIDs := grouped[results[i].{{.ParentPKField}}]
		items := make([]{{.TargetType}}, 0, len(tIDs))
		for _, tid := range tIDs {
			if v, ok := byPK[tid]; ok {
				items = append(items, v)
			}
		}
		results[i].{{.FieldName}} = items
	}
	return nil
}
{{- else}}
func {{.PreloaderName}}(ctx context.Context, db orm.Querier, results []{{.ParentType}}) error {
	if len(results) == 0 {
		return nil
	}
	{{- if .FKIsPointer}}
	ids := make([]{{.KeyType}}, 0, len(results))
	for i := range results {
		if results[i].{{.ForeignKeyField}} != nil {
			ids = append(ids, *results[i].{{.ForeignKeyField}})
		}
	}
	{{- else}}
	ids := make([]{{.KeyType}}, len(results))
	for i := range results {
		ids[i] = results[i].{{.ForeignKeyField}}
	}
	{{- end}}
	related, err := {{.TargetFactory}}(db).Scopes(scope.In("id", ids)).All(ctx)
	if err != nil {
		return err
	}
	{{- if .IsPointer}}
	byPK := make(map[{{.KeyType}}]*{{.TargetType}})
	for i := range related {
		byPK[related[i].ID] = &related[i]
	}
	{{- else}}
	byPK := make(map[{{.KeyType}}]{{.TargetType}})
	for _, r := range related {
		byPK[r.ID] = r
	}
	{{- end}}
	for i := range results {
		{{- if .FKIsPointer}}
		if results[i].{{.ForeignKeyField}} != nil {
			results[i].{{.FieldName}} = byPK[*results[i].{{.ForeignKeyField}}]
		}
		{{- else}}
		results[i].{{.FieldName}} = byPK[results[i].{{.ForeignKeyField}}]
		{{- end}}
	}
	return nil
}
{{- end}}
{{- end}}
{{end}}`

func buildRelationData(info *StructInfo, pk *FieldInfo, typePrefix, sourceImport, destPkg string, allInfos []*StructInfo) ([]relationTemplateData, []importEntry) {
	if len(info.Relations) == 0 {
		return nil, nil
	}

	rels := make([]relationTemplateData, 0, len(info.Relations))
	seen := make(map[string]bool)
	var extraImports []importEntry

	for _, rel := range info.Relations {
		targetTable := inflection.Plural(naming.CamelToSnake(rel.TargetType))
		targetFactory := naming.SnakeToCamel(targetTable)
		fkField := naming.SnakeToCamel(rel.ForeignKey)

		// Determine type prefix for the target type.
		targetTypePrefix := typePrefix
		isCrossPkg := rel.TargetImportPath != "" && rel.TargetImportPath != sourceImport
		if isCrossPkg {
			alias := resolveAlias(rel.TargetImportPath, sourceImport)
			targetTypePrefix = alias + "."
			if !seen[rel.TargetImportPath] {
				seen[rel.TargetImportPath] = true
				parts := strings.Split(rel.TargetImportPath, "/")
				lastSeg := parts[len(parts)-1]
				entry := importEntry{Path: rel.TargetImportPath}
				if alias != lastSeg {
					entry.Alias = alias
				}
				extraImports = append(extraImports, entry)
			}
		}

		// For cross-package relations with a separate dest package, the target
		// factory lives in the external query package, not the current one.
		if isCrossPkg && destPkg != "" && sourceImport != "" {
			extQueryImport := replaceLastSegment(rel.TargetImportPath, destPkg)
			destQueryImport := replaceLastSegment(sourceImport, destPkg)
			if extQueryImport != destQueryImport {
				queryAlias := resolveAlias(extQueryImport, destQueryImport)
				targetFactory = queryAlias + "." + targetFactory
				if !seen[extQueryImport] {
					seen[extQueryImport] = true
					entry := importEntry{Path: extQueryImport}
					if queryAlias != destPkg {
						entry.Alias = queryAlias
					}
					extraImports = append(extraImports, entry)
				}
			}
		}

		rd := relationTemplateData{
			FieldName:       rel.FieldName,
			ParentType:      typePrefix + info.Name,
			TargetType:      targetTypePrefix + rel.TargetType,
			TargetFactory:   targetFactory,
			ForeignKey:      rel.ForeignKey,
			ForeignKeyField: fkField,
			RelType:         rel.RelType,
			IsPointer:       rel.IsPointer,
			PreloaderName:   unexportedName("preload" + info.Name + rel.FieldName),
			ParentPKField:   pk.Name,
		}

		switch rel.RelType {
		case "has_many", "has_one":
			rd.KeyType = pk.GoType
			rd.JoinTargetTable = targetTable
			rd.JoinTargetColumn = rel.ForeignKey
			rd.JoinSourceTable = info.TableName
			rd.JoinSourceColumn = pk.Column
		case "many_to_many":
			rd.KeyType = pk.GoType
			rd.JoinTable = rel.JoinTable
			rd.References = rel.References
			rd.TargetTable = targetTable
			rd.TargetPKColumn = "id" // convention
		default: // belongs_to
			fkType := lookupFieldType(info, rel.ForeignKey)
			if strings.HasPrefix(fkType, "*") {
				rd.FKIsPointer = true
				fkType = fkType[1:]
			}
			rd.KeyType = fkType
			rd.JoinTargetTable = targetTable
			rd.JoinTargetColumn = "id" // convention: target PK is "id"
			rd.JoinSourceTable = info.TableName
			rd.JoinSourceColumn = rel.ForeignKey
		}

		// Populate join scan fields for belongs_to / has_one when the target
		// struct is in the same package (available in allInfos).
		if (rel.RelType == "belongs_to" || rel.RelType == "has_one") && !isCrossPkg {
			if targetInfo := findStructInfo(allInfos, rel.TargetType); targetInfo != nil {
				rd.JoinScanFields = targetInfo.Fields
				rd.JoinSelectColumns = make([]string, len(targetInfo.Fields))
				for i, f := range targetInfo.Fields {
					rd.JoinSelectColumns[i] = f.Column
				}
				if targetPK, err := targetInfo.PrimaryKeyField(); err == nil {
					rd.JoinPKGoType = targetPK.GoType
					rd.JoinPKName = targetPK.Name
					if rel.IsPointer {
						rd.JoinNullType, rd.JoinNullField = nullTypeFor(targetPK.GoType)
					}
				}
			}
		}

		rels = append(rels, rd)
	}
	return rels, extraImports
}

// resolveAlias determines the import alias for an external package.
// If the last path segment conflicts with the source import's last segment,
// it prepends the previous segment to disambiguate.
func resolveAlias(importPath, sourceImport string) string {
	parts := strings.Split(importPath, "/")
	lastSeg := parts[len(parts)-1]

	if sourceImport == "" {
		return lastSeg
	}

	srcParts := strings.Split(sourceImport, "/")
	srcLastSeg := srcParts[len(srcParts)-1]

	if lastSeg != srcLastSeg {
		return lastSeg
	}

	// Conflict: e.g. both end in "model". Use previous segment + last segment.
	if len(parts) >= 2 {
		return parts[len(parts)-2] + lastSeg
	}
	return lastSeg
}

// replaceLastSegment replaces the last path segment of an import path.
// e.g. replaceLastSegment("github.com/foo/model", "query") → "github.com/foo/query"
func replaceLastSegment(importPath, newSeg string) string {
	i := strings.LastIndex(importPath, "/")
	if i < 0 {
		return newSeg
	}
	return importPath[:i+1] + newSeg
}

func lookupFieldType(info *StructInfo, column string) string {
	for _, f := range info.Fields {
		if f.Column == column {
			return f.GoType
		}
	}
	return "int" // fallback
}

func unexportedName(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func filterFields(fields []FieldInfo, pred func(FieldInfo) bool) []FieldInfo {
	var out []FieldInfo
	for _, f := range fields {
		if pred(f) {
			out = append(out, f)
		}
	}
	return out
}

func findStructInfo(infos []*StructInfo, name string) *StructInfo {
	for _, info := range infos {
		if info.Name == name {
			return info
		}
	}
	return nil
}

func nullTypeFor(goType string) (nullType, nullField string) {
	if goType == "string" {
		return "sql.NullString", ".String"
	}
	return "sql.NullInt64", ".Int64"
}

func isIntType(goType string) bool {
	switch goType {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64":
		return true
	default:
		return false
	}
}
