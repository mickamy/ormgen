package gen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mickamy/ormgen/internal/gen"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

func TestParse(t *testing.T) {
	t.Parallel()

	infos, err := gen.Parse(testdataPath("user.go"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(infos) != 2 {
		t.Fatalf("len(infos) = %d, want 2", len(infos))
	}

	// Package is set for all
	for _, info := range infos {
		if info.Package != "testdata" {
			t.Errorf("%s: Package = %q, want %q", info.Name, info.Package, "testdata")
		}
	}

	t.Run("User", func(t *testing.T) {
		t.Parallel()

		info := infos[0]
		if info.Name != "User" {
			t.Errorf("Name = %q, want %q", info.Name, "User")
		}

		// 7 db fields (Posts is db:"-", internal has no tag)
		if len(info.Fields) != 7 {
			t.Fatalf("len(Fields) = %d, want 7", len(info.Fields))
		}

		// Check first field
		f := info.Fields[0]
		if f.Name != "ID" || f.Column != "id" || f.GoType != "int" || !f.PrimaryKey {
			t.Errorf("Fields[0] = %+v", f)
		}

		// Check time.Time field (CreatedAt convention)
		f = info.Fields[5]
		if f.Name != "CreatedAt" || f.Column != "created_at" || f.GoType != "time.Time" || !f.CreatedAt {
			t.Errorf("Fields[5] = %+v", f)
		}

		// Check UpdatedAt convention
		f = info.Fields[6]
		if f.Name != "UpdatedAt" || f.Column != "updated_at" || !f.UpdatedAt {
			t.Errorf("Fields[6] = %+v", f)
		}
	})

	t.Run("Post", func(t *testing.T) {
		t.Parallel()

		info := infos[1]
		if info.Name != "Post" {
			t.Errorf("Name = %q, want %q", info.Name, "Post")
		}

		if len(info.Fields) != 3 {
			t.Fatalf("len(Fields) = %d, want 3", len(info.Fields))
		}
		if info.Fields[0].Column != "id" || !info.Fields[0].PrimaryKey {
			t.Errorf("Fields[0] = %+v", info.Fields[0])
		}
	})
}

func TestParsePrimaryKeyField(t *testing.T) {
	t.Parallel()

	infos, err := gen.Parse(testdataPath("user.go"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	pk, err := infos[0].PrimaryKeyField()
	if err != nil {
		t.Fatalf("PrimaryKeyField: %v", err)
	}
	if pk.Name != "ID" || pk.Column != "id" {
		t.Errorf("PK = %+v", pk)
	}
}

func TestParseNoPrimaryKey(t *testing.T) {
	t.Parallel()

	infos, err := gen.Parse(testdataPath("no_pk.go"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("len(infos) = %d, want 1", len(infos))
	}

	_, err = infos[0].PrimaryKeyField()
	if err == nil {
		t.Fatal("expected error for no primary key, got nil")
	}
}

func TestParseInferredColumns(t *testing.T) {
	t.Parallel()

	infos, err := gen.Parse(testdataPath("inferred.go"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("len(infos) = %d, want 1", len(infos))
	}

	info := infos[0]

	// ID (db:",primaryKey"), Name (no tag), CreatedAt (no tag)
	// Secret (db:"-") and internal (unexported) are skipped
	if len(info.Fields) != 3 {
		t.Fatalf("len(Fields) = %d, want 3", len(info.Fields))
	}

	f := info.Fields[0]
	if f.Name != "ID" || f.Column != "id" || !f.PrimaryKey {
		t.Errorf("Fields[0] = %+v", f)
	}

	f = info.Fields[1]
	if f.Name != "Name" || f.Column != "name" {
		t.Errorf("Fields[1] = %+v", f)
	}

	f = info.Fields[2]
	if f.Name != "CreatedAt" || f.Column != "created_at" || !f.CreatedAt {
		t.Errorf("Fields[2] = %+v", f)
	}
}

func TestParseTimestamps(t *testing.T) {
	t.Parallel()

	infos, err := gen.Parse(testdataPath("timestamps.go"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(infos) != 3 {
		t.Fatalf("len(infos) = %d, want 3", len(infos))
	}

	t.Run("WithTimestamps convention", func(t *testing.T) {
		t.Parallel()

		info := infos[0]
		if info.Name != "WithTimestamps" {
			t.Fatalf("Name = %q, want %q", info.Name, "WithTimestamps")
		}
		// ID, Name, CreatedAt, UpdatedAt
		if len(info.Fields) != 4 {
			t.Fatalf("len(Fields) = %d, want 4", len(info.Fields))
		}

		f := info.Fields[2]
		if f.Name != "CreatedAt" || f.Column != "created_at" || !f.CreatedAt {
			t.Errorf("CreatedAt = %+v", f)
		}
		f = info.Fields[3]
		if f.Name != "UpdatedAt" || f.Column != "updated_at" || !f.UpdatedAt {
			t.Errorf("UpdatedAt = %+v", f)
		}
	})

	t.Run("WithCustomTimestampCols tag", func(t *testing.T) {
		t.Parallel()

		info := infos[1]
		if info.Name != "WithCustomTimestampCols" {
			t.Fatalf("Name = %q, want %q", info.Name, "WithCustomTimestampCols")
		}
		if len(info.Fields) != 3 {
			t.Fatalf("len(Fields) = %d, want 3", len(info.Fields))
		}

		f := info.Fields[1]
		if f.Name != "InsertedAt" || f.Column != "inserted_at" || !f.CreatedAt {
			t.Errorf("InsertedAt = %+v", f)
		}
		f = info.Fields[2]
		if f.Name != "ModifiedAt" || f.Column != "modified_at" || !f.UpdatedAt {
			t.Errorf("ModifiedAt = %+v", f)
		}
	})

	t.Run("WithTagAndConvention", func(t *testing.T) {
		t.Parallel()

		info := infos[2]
		if info.Name != "WithTagAndConvention" {
			t.Fatalf("Name = %q, want %q", info.Name, "WithTagAndConvention")
		}
		if len(info.Fields) != 3 {
			t.Fatalf("len(Fields) = %d, want 3", len(info.Fields))
		}

		f := info.Fields[1]
		if f.Name != "CreatedAt" || f.Column != "created_at" || !f.CreatedAt {
			t.Errorf("CreatedAt = %+v", f)
		}
		f = info.Fields[2]
		if f.Name != "UpdatedAt" || f.Column != "updated_at" || !f.UpdatedAt {
			t.Errorf("UpdatedAt = %+v", f)
		}
	})
}

func TestParseCustomTypes(t *testing.T) {
	t.Parallel()

	infos, err := gen.Parse(testdataPath("custom_types.go"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(infos) != 2 {
		t.Fatalf("len(infos) = %d, want 2", len(infos))
	}

	t.Run("Repository with db tag", func(t *testing.T) {
		t.Parallel()

		info := infos[0]
		if info.Name != "Repository" {
			t.Fatalf("Name = %q, want %q", info.Name, "Repository")
		}

		// ID, Name, Topics
		if len(info.Fields) != 3 {
			t.Fatalf("len(Fields) = %d, want 3", len(info.Fields))
		}

		f := info.Fields[2]
		if f.Name != "Topics" || f.Column != "topics" || f.GoType != "StringArray" {
			t.Errorf("Fields[2] = %+v", f)
		}
	})

	t.Run("NoTagCustomType convention", func(t *testing.T) {
		t.Parallel()

		info := infos[1]
		if info.Name != "NoTagCustomType" {
			t.Fatalf("Name = %q, want %q", info.Name, "NoTagCustomType")
		}

		// ID, Name, Tags — Owner (*User with rel tag) should be skipped
		if len(info.Fields) != 3 {
			t.Fatalf("len(Fields) = %d, want 3", len(info.Fields))
		}

		f := info.Fields[2]
		if f.Name != "Tags" || f.Column != "tags" || f.GoType != "StringArray" {
			t.Errorf("Fields[2] = %+v", f)
		}

		// Owner should be in relations, not fields
		if len(info.Relations) != 1 {
			t.Fatalf("len(Relations) = %d, want 1", len(info.Relations))
		}
		if info.Relations[0].FieldName != "Owner" {
			t.Errorf("Relations[0].FieldName = %q, want %q", info.Relations[0].FieldName, "Owner")
		}
	})
}

func TestParseRelations(t *testing.T) { //nolint:gocyclo // test function with many assertions
	t.Parallel()

	infos, err := gen.Parse(testdataPath("relations.go"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(infos) != 4 {
		t.Fatalf("len(infos) = %d, want 4", len(infos))
	}

	t.Run("Author has_many Articles, has_one Profile, many_to_many Tags", func(t *testing.T) {
		t.Parallel()

		info := infos[0]
		if info.Name != "Author" {
			t.Fatalf("Name = %q, want %q", info.Name, "Author")
		}
		if len(info.Relations) != 3 {
			t.Fatalf("len(Relations) = %d, want 3", len(info.Relations))
		}

		rel1 := info.Relations[0]
		if rel1.FieldName != "Articles" {
			t.Errorf("FieldName = %q, want %q", rel1.FieldName, "Articles")
		}
		if rel1.TargetType != "Article" {
			t.Errorf("TargetType = %q, want %q", rel1.TargetType, "Article")
		}
		if rel1.RelType != "has_many" {
			t.Errorf("RelType = %q, want %q", rel1.RelType, "has_many")
		}
		if rel1.ForeignKey != "author_id" {
			t.Errorf("ForeignKey = %q, want %q", rel1.ForeignKey, "author_id")
		}
		if !rel1.IsSlice {
			t.Error("IsSlice = false, want true")
		}
		if rel1.IsPointer {
			t.Error("IsPointer = true, want false")
		}

		// has_one
		rel2 := info.Relations[1]
		if rel2.FieldName != "Profile" {
			t.Errorf("FieldName = %q, want %q", rel2.FieldName, "Profile")
		}
		if rel2.TargetType != "Profile" {
			t.Errorf("TargetType = %q, want %q", rel2.TargetType, "Profile")
		}
		if rel2.RelType != "has_one" {
			t.Errorf("RelType = %q, want %q", rel2.RelType, "has_one")
		}
		if rel2.ForeignKey != "author_id" {
			t.Errorf("ForeignKey = %q, want %q", rel2.ForeignKey, "author_id")
		}
		if rel2.IsSlice {
			t.Error("IsSlice = true, want false")
		}
		if !rel2.IsPointer {
			t.Error("IsPointer = false, want true")
		}

		// many_to_many
		rel3 := info.Relations[2]
		if rel3.FieldName != "Tags" {
			t.Errorf("FieldName = %q, want %q", rel3.FieldName, "Tags")
		}
		if rel3.TargetType != "Tag" {
			t.Errorf("TargetType = %q, want %q", rel3.TargetType, "Tag")
		}
		if rel3.RelType != "many_to_many" {
			t.Errorf("RelType = %q, want %q", rel3.RelType, "many_to_many")
		}
		if rel3.ForeignKey != "author_id" {
			t.Errorf("ForeignKey = %q, want %q", rel3.ForeignKey, "author_id")
		}
		if rel3.JoinTable != "author_tags" {
			t.Errorf("JoinTable = %q, want %q", rel3.JoinTable, "author_tags")
		}
		if rel3.References != "tag_id" {
			t.Errorf("References = %q, want %q", rel3.References, "tag_id")
		}
		if !rel3.IsSlice {
			t.Error("IsSlice = false, want true")
		}
		if rel3.IsPointer {
			t.Error("IsPointer = true, want false")
		}
	})

	t.Run("Article belongs_to Author", func(t *testing.T) {
		t.Parallel()

		info := infos[3]
		if info.Name != "Article" {
			t.Fatalf("Name = %q, want %q", info.Name, "Article")
		}
		if len(info.Relations) != 1 {
			t.Fatalf("len(Relations) = %d, want 1", len(info.Relations))
		}

		rel := info.Relations[0]
		if rel.FieldName != "Author" {
			t.Errorf("FieldName = %q, want %q", rel.FieldName, "Author")
		}
		if rel.TargetType != "Author" {
			t.Errorf("TargetType = %q, want %q", rel.TargetType, "Author")
		}
		if rel.RelType != "belongs_to" {
			t.Errorf("RelType = %q, want %q", rel.RelType, "belongs_to")
		}
		if rel.ForeignKey != "author_id" {
			t.Errorf("ForeignKey = %q, want %q", rel.ForeignKey, "author_id")
		}
		if rel.IsSlice {
			t.Error("IsSlice = true, want false")
		}
		if !rel.IsPointer {
			t.Error("IsPointer = false, want true")
		}
	})
}

func TestParseCrossPackageRelations(t *testing.T) {
	t.Parallel()

	infos, err := gen.Parse(testdataPath("cross_pkg_relations.go"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	info := findStructInInfos(t, infos, "EndUser")

	if len(info.Relations) != 2 {
		t.Fatalf("len(Relations) = %d, want 2", len(info.Relations))
	}

	// Cross-package relation: []amodel.OAuthAccount
	rel0 := info.Relations[0]
	if rel0.TargetType != "OAuthAccount" {
		t.Errorf("Relations[0].TargetType = %q, want %q", rel0.TargetType, "OAuthAccount")
	}
	if rel0.TargetPkgAlias != "amodel" {
		t.Errorf("Relations[0].TargetPkgAlias = %q, want %q", rel0.TargetPkgAlias, "amodel")
	}
	if rel0.TargetImportPath != "github.com/example/auth/model" {
		t.Errorf("Relations[0].TargetImportPath = %q, want %q", rel0.TargetImportPath, "github.com/example/auth/model")
	}

	// Same-package relation: *UserEmail
	rel1 := info.Relations[1]
	if rel1.TargetType != "UserEmail" {
		t.Errorf("Relations[1].TargetType = %q, want %q", rel1.TargetType, "UserEmail")
	}
	if rel1.TargetPkgAlias != "" {
		t.Errorf("Relations[1].TargetPkgAlias = %q, want empty", rel1.TargetPkgAlias)
	}
	if rel1.TargetImportPath != "" {
		t.Errorf("Relations[1].TargetImportPath = %q, want empty", rel1.TargetImportPath)
	}
}

func findStructInInfos(t *testing.T, infos []*gen.StructInfo, name string) *gen.StructInfo {
	t.Helper()
	for _, info := range infos {
		if info.Name == name {
			return info
		}
	}
	t.Fatalf("struct %q not found", name)
	return nil
}

func TestParseInvalidFile(t *testing.T) {
	t.Parallel()

	_, err := gen.Parse("nonexistent.go")
	if err == nil {
		t.Fatal("expected error for invalid file, got nil")
	}
}
