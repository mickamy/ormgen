package orm_test

import (
	"database/sql"
	"testing"

	"github.com/mickamy/ormgen/orm"
	"github.com/mickamy/ormgen/scope"
)

type testUser struct {
	ID   int
	Name string
}

var testUserColumns = []string{"id", "name"}

func scanTestUser(_ *sql.Rows) (testUser, error) {
	return testUser{}, nil
}

func testUserColValPairs(u *testUser, includesPK bool) ([]string, []any) {
	if includesPK {
		return []string{"id", "name"}, []any{u.ID, u.Name}
	}
	return []string{"name"}, []any{u.Name}
}

func setTestUserPK(u *testUser, id int64) {
	u.ID = int(id)
}

func newTestQuery(tq *orm.TestQuerier) *orm.Query[testUser] {
	return orm.NewQuery[testUser](tq, "users", testUserColumns, "id", scanTestUser, testUserColValPairs, setTestUserPK)
}

// --- SELECT (MySQL) ---

func TestBuildSelectAll(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)

	_, _ = q.All(t.Context())

	got := tq.LastQuery()
	want := "SELECT `id`, `name` FROM `users`"
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
}

func TestBuildSelectWhere(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)

	_, _ = q.Where("name = ?", "alice").All(t.Context())

	got := tq.LastQuery()
	want := "SELECT `id`, `name` FROM `users` WHERE name = ?"
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
	if len(got.Args) != 1 || got.Args[0] != "alice" {
		t.Errorf("Args = %v", got.Args)
	}
}

func TestBuildSelectMultipleWhere(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)

	_, _ = q.Where("name = ?", "alice").Where("id > ?", 10).All(t.Context())

	got := tq.LastQuery()
	want := "SELECT `id`, `name` FROM `users` WHERE name = ? AND id > ?"
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
	if len(got.Args) != 2 {
		t.Errorf("Args = %v, want 2 args", got.Args)
	}
}

func TestBuildSelectOrderBy(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)

	_, _ = q.OrderBy("name ASC").All(t.Context())

	got := tq.LastQuery()
	want := "SELECT `id`, `name` FROM `users` ORDER BY name ASC"
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
}

func TestBuildSelectLimitOffset(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)

	_, _ = q.Limit(10).Offset(20).All(t.Context())

	got := tq.LastQuery()
	want := "SELECT `id`, `name` FROM `users` LIMIT 10 OFFSET 20"
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
}

func TestBuildSelectCustomColumns(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)

	_, _ = q.Select("id").All(t.Context())

	got := tq.LastQuery()
	want := "SELECT id FROM `users`"
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
}

func TestBuildSelectFull(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)

	_, _ = q.
		Where("name = ?", "alice").
		OrderBy("id DESC").
		Limit(5).
		Offset(10).
		All(t.Context())

	got := tq.LastQuery()
	want := "SELECT `id`, `name` FROM `users` WHERE name = ? ORDER BY id DESC LIMIT 5 OFFSET 10"
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
}

// --- Scopes ---

func TestBuildSelectWithScopes(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)

	_, _ = q.Scopes(
		scope.Where("name = ?", "alice"),
		scope.OrderBy("id DESC"),
		scope.Limit(5),
		scope.Offset(10),
	).All(t.Context())

	got := tq.LastQuery()
	want := "SELECT `id`, `name` FROM `users` WHERE name = ? ORDER BY id DESC LIMIT 5 OFFSET 10"
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
}

// --- Immutability ---

func TestQueryImmutability(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	base := newTestQuery(tq)

	_ = base.Where("name = ?", "alice")
	_ = base.OrderBy("id")
	_ = base.Limit(10)
	_ = base.Offset(5)

	_, _ = base.All(t.Context())

	got := tq.LastQuery()
	want := "SELECT `id`, `name` FROM `users`"
	if got.SQL != want {
		t.Errorf("base query was mutated: SQL = %q", got.SQL)
	}
}

// --- INSERT ---

func TestBuildInsertMySQL(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)

	u := testUser{Name: "alice"}
	_ = q.Create(t.Context(), &u)

	got := tq.LastQuery()
	want := "INSERT INTO `users` (`name`) VALUES (?)"
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
	if len(got.Args) != 1 || got.Args[0] != "alice" {
		t.Errorf("Args = %v", got.Args)
	}
}

func TestBuildInsertPostgreSQL(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.PostgreSQL)
	q := newTestQuery(tq)

	u := testUser{Name: "alice"}
	_ = q.Create(t.Context(), &u)

	got := tq.LastQuery()
	want := `INSERT INTO "users" ("name") VALUES ($1) RETURNING "id"`
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
}

// --- UPDATE ---

func TestBuildUpdate(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)

	u := testUser{ID: 1, Name: "bob"}
	_ = q.Update(t.Context(), &u)

	got := tq.LastQuery()
	want := "UPDATE `users` SET `name` = ? WHERE `id` = ?"
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
	if len(got.Args) != 2 || got.Args[0] != "bob" || got.Args[1] != 1 {
		t.Errorf("Args = %v", got.Args)
	}
}

func TestBuildUpdatePostgreSQL(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.PostgreSQL)
	q := newTestQuery(tq)

	u := testUser{ID: 1, Name: "bob"}
	_ = q.Update(t.Context(), &u)

	got := tq.LastQuery()
	want := `UPDATE "users" SET "name" = $1 WHERE "id" = $2`
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
}

// --- DELETE ---

func TestBuildDelete(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)

	_ = q.Where("id = ?", 1).Delete(t.Context())

	got := tq.LastQuery()
	want := "DELETE FROM `users` WHERE id = ?"
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
}

func TestDeleteWithoutWhereReturnsError(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)

	err := q.Delete(t.Context())
	if err == nil {
		t.Fatal("expected error for Delete without WHERE, got nil")
	}
}

// --- Rewrite (PostgreSQL placeholders) ---

func TestRewritePostgreSQLSelect(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.PostgreSQL)
	q := newTestQuery(tq)

	_, _ = q.Where("name = ?", "alice").Where("id > ?", 10).All(t.Context())

	got := tq.LastQuery()
	want := `SELECT "id", "name" FROM "users" WHERE name = $1 AND id > $2`
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
}

// --- First ---

func TestFirstAddsLimit(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)

	_, _ = q.First(t.Context())

	got := tq.LastQuery()
	want := "SELECT `id`, `name` FROM `users` LIMIT 1"
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
}
