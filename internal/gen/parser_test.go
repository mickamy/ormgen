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

		// Check time.Time field
		f = info.Fields[5]
		if f.Name != "CreatedAt" || f.Column != "created_at" || f.GoType != "time.Time" {
			t.Errorf("Fields[5] = %+v", f)
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
	if f.Name != "CreatedAt" || f.Column != "created_at" {
		t.Errorf("Fields[2] = %+v", f)
	}
}

func TestParseRelations(t *testing.T) {
	t.Parallel()

	infos, err := gen.Parse(testdataPath("relations.go"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(infos) != 2 {
		t.Fatalf("len(infos) = %d, want 2", len(infos))
	}

	t.Run("Author has_many Articles", func(t *testing.T) {
		t.Parallel()

		info := infos[0]
		if info.Name != "Author" {
			t.Fatalf("Name = %q, want %q", info.Name, "Author")
		}
		if len(info.Relations) != 1 {
			t.Fatalf("len(Relations) = %d, want 1", len(info.Relations))
		}

		rel := info.Relations[0]
		if rel.FieldName != "Articles" {
			t.Errorf("FieldName = %q, want %q", rel.FieldName, "Articles")
		}
		if rel.TargetType != "Article" {
			t.Errorf("TargetType = %q, want %q", rel.TargetType, "Article")
		}
		if rel.RelType != "has_many" {
			t.Errorf("RelType = %q, want %q", rel.RelType, "has_many")
		}
		if rel.ForeignKey != "author_id" {
			t.Errorf("ForeignKey = %q, want %q", rel.ForeignKey, "author_id")
		}
		if !rel.IsSlice {
			t.Error("IsSlice = false, want true")
		}
		if rel.IsPointer {
			t.Error("IsPointer = true, want false")
		}
	})

	t.Run("Article belongs_to Author", func(t *testing.T) {
		t.Parallel()

		info := infos[1]
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

func TestParseInvalidFile(t *testing.T) {
	t.Parallel()

	_, err := gen.Parse("nonexistent.go")
	if err == nil {
		t.Fatal("expected error for invalid file, got nil")
	}
}
