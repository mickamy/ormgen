package orm_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

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

// --- Timestamp tests ---

type testArticle struct {
	ID        int
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

var testArticleColumns = []string{"id", "title", "created_at", "updated_at"}

func scanTestArticle(_ *sql.Rows) (testArticle, error) {
	return testArticle{}, nil
}

func testArticleColValPairs(a *testArticle, includesPK bool) ([]string, []any) {
	if includesPK {
		return []string{"id", "title", "created_at", "updated_at"},
			[]any{a.ID, a.Title, a.CreatedAt, a.UpdatedAt}
	}
	return []string{"title", "created_at", "updated_at"},
		[]any{a.Title, a.CreatedAt, a.UpdatedAt}
}

func setTestArticlePK(a *testArticle, id int64) {
	a.ID = int(id)
}

func setTestArticleCreatedAt(a *testArticle, now time.Time) {
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
}

func setTestArticleUpdatedAt(a *testArticle, now time.Time) {
	a.UpdatedAt = now
}

func newTestArticleQuery(tq *orm.TestQuerier) *orm.Query[testArticle] {
	q := orm.NewQuery[testArticle](tq, "articles", testArticleColumns, "id", scanTestArticle, testArticleColValPairs, setTestArticlePK)
	q.RegisterTimestamps([]string{"created_at"}, setTestArticleCreatedAt, []string{"updated_at"}, setTestArticleUpdatedAt)
	return q
}

type fixedClock struct {
	t time.Time
}

func (c fixedClock) Now() time.Time { return c.t }

func TestUpsertExcludesCreatedAtFromUpdate(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestArticleQuery(tq)

	a := testArticle{ID: 1, Title: "hello"}
	_ = q.Upsert(t.Context(), &a)

	got := tq.LastQuery()
	// UPDATE clause should NOT contain created_at
	if strings.Contains(got.SQL, "ON DUPLICATE KEY UPDATE") {
		updatePart := got.SQL[strings.Index(got.SQL, "ON DUPLICATE KEY UPDATE"):]
		if strings.Contains(updatePart, "created_at") {
			t.Errorf("UPDATE clause should not contain created_at: %s", got.SQL)
		}
		if !strings.Contains(updatePart, "updated_at") {
			t.Errorf("UPDATE clause should contain updated_at: %s", got.SQL)
		}
	} else {
		t.Errorf("expected ON DUPLICATE KEY UPDATE in SQL: %s", got.SQL)
	}
}

func TestUpsertExcludesCreatedAtPostgreSQL(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.PostgreSQL)
	q := newTestArticleQuery(tq)

	a := testArticle{ID: 1, Title: "hello"}
	_ = q.Upsert(t.Context(), &a)

	got := tq.LastQuery()
	// DO UPDATE SET should NOT contain created_at
	if strings.Contains(got.SQL, "DO UPDATE SET") {
		updatePart := got.SQL[strings.Index(got.SQL, "DO UPDATE SET"):]
		if strings.Contains(updatePart, "created_at") {
			t.Errorf("UPDATE SET should not contain created_at: %s", got.SQL)
		}
		if !strings.Contains(updatePart, "updated_at") {
			t.Errorf("UPDATE SET should contain updated_at: %s", got.SQL)
		}
	} else {
		t.Errorf("expected DO UPDATE SET in SQL: %s", got.SQL)
	}
}

func TestCreateAutoSetsTimestamps(t *testing.T) {
	t.Parallel()

	fixed := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	ctx := orm.WithClock(t.Context(), fixedClock{t: fixed})

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestArticleQuery(tq)

	a := testArticle{Title: "hello"}
	_ = q.Create(ctx, &a)

	if a.CreatedAt != fixed {
		t.Errorf("CreatedAt = %v, want %v", a.CreatedAt, fixed)
	}
	if a.UpdatedAt != fixed {
		t.Errorf("UpdatedAt = %v, want %v", a.UpdatedAt, fixed)
	}
}

func TestCreatePreservesExistingCreatedAt(t *testing.T) {
	t.Parallel()

	existing := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	fixed := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	ctx := orm.WithClock(t.Context(), fixedClock{t: fixed})

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestArticleQuery(tq)

	a := testArticle{Title: "hello", CreatedAt: existing}
	_ = q.Create(ctx, &a)

	if a.CreatedAt != existing {
		t.Errorf("CreatedAt = %v, want %v (should not be overwritten)", a.CreatedAt, existing)
	}
	if a.UpdatedAt != fixed {
		t.Errorf("UpdatedAt = %v, want %v", a.UpdatedAt, fixed)
	}
}

func TestUpdateOnlySetsUpdatedAt(t *testing.T) {
	t.Parallel()

	existing := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	fixed := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	ctx := orm.WithClock(t.Context(), fixedClock{t: fixed})

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestArticleQuery(tq)

	a := testArticle{ID: 1, Title: "hello", CreatedAt: existing}
	_ = q.Update(ctx, &a)

	if a.CreatedAt != existing {
		t.Errorf("CreatedAt = %v, want %v (Update should not touch createdAt)", a.CreatedAt, existing)
	}
	if a.UpdatedAt != fixed {
		t.Errorf("UpdatedAt = %v, want %v", a.UpdatedAt, fixed)
	}
}

// --- scope.Join / scope.LeftJoin / scope.Preload via Scopes ---

func TestBuildSelectWithScopeJoin(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)
	q.RegisterJoin("Posts", orm.JoinConfig{
		TargetTable:  "posts",
		TargetColumn: "user_id",
		SourceTable:  "users",
		SourceColumn: "id",
	})

	_, _ = q.Scopes(scope.Join("Posts")).All(t.Context())

	got := tq.LastQuery()
	if !strings.Contains(got.SQL, "INNER JOIN") {
		t.Errorf("SQL should contain INNER JOIN: %q", got.SQL)
	}
	want := "SELECT `users`.`id`, `users`.`name` FROM `users` INNER JOIN `posts` ON `posts`.`user_id` = `users`.`id`"
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
}

func TestBuildSelectWithScopeLeftJoin(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)
	q.RegisterJoin("Posts", orm.JoinConfig{
		TargetTable:  "posts",
		TargetColumn: "user_id",
		SourceTable:  "users",
		SourceColumn: "id",
	})

	_, _ = q.Scopes(scope.LeftJoin("Posts")).All(t.Context())

	got := tq.LastQuery()
	if !strings.Contains(got.SQL, "LEFT JOIN") {
		t.Errorf("SQL should contain LEFT JOIN: %q", got.SQL)
	}
	want := "SELECT `users`.`id`, `users`.`name` FROM `users` LEFT JOIN `posts` ON `posts`.`user_id` = `users`.`id`"
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
}

func TestBuildSelectWithJoinSelectColumns(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)
	q.RegisterJoin("Author", orm.JoinConfig{
		TargetTable:   "authors",
		TargetColumn:  "id",
		SourceTable:   "users",
		SourceColumn:  "author_id",
		SelectColumns: []string{"id", "name"},
	})

	_, _ = q.Join("Author").All(t.Context())

	got := tq.LastQuery()
	want := "SELECT `users`.`id`, `users`.`name`, `authors`.`id` AS `Author__id`, `authors`.`name` AS `Author__name` FROM `users` INNER JOIN `authors` ON `authors`.`id` = `users`.`author_id`"
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
}

func TestBuildSelectWithJoinSelectColumnsPostgreSQL(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.PostgreSQL)
	q := newTestQuery(tq)
	q.RegisterJoin("Author", orm.JoinConfig{
		TargetTable:   "authors",
		TargetColumn:  "id",
		SourceTable:   "users",
		SourceColumn:  "author_id",
		SelectColumns: []string{"id", "name"},
	})

	_, _ = q.Join("Author").All(t.Context())

	got := tq.LastQuery()
	want := `SELECT "users"."id", "users"."name", "authors"."id" AS "Author__id", "authors"."name" AS "Author__name" FROM "users" INNER JOIN "authors" ON "authors"."id" = "users"."author_id"`
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
}

func TestBuildSelectWithJoinNoSelectColumns(t *testing.T) {
	t.Parallel()

	// When SelectColumns is nil (e.g. has_many), no extra columns are added.
	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)
	q.RegisterJoin("Posts", orm.JoinConfig{
		TargetTable:  "posts",
		TargetColumn: "user_id",
		SourceTable:  "users",
		SourceColumn: "id",
	})

	_, _ = q.Join("Posts").All(t.Context())

	got := tq.LastQuery()
	want := "SELECT `users`.`id`, `users`.`name` FROM `users` INNER JOIN `posts` ON `posts`.`user_id` = `users`.`id`"
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
}

func TestBuildSelectWithScopePreload(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)
	q.RegisterPreloader("Posts", func(_ context.Context, _ orm.Querier, _ []testUser) error {
		return nil
	})

	// Scopes(scope.Preload("Posts")) should not affect the generated SQL;
	// preloads are executed after the main query.
	_, _ = q.Scopes(scope.Preload("Posts")).All(t.Context())

	got := tq.LastQuery()
	want := "SELECT `id`, `name` FROM `users`"
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q (preload should not alter SQL)", got.SQL, want)
	}
}

// --- Updates ---

func TestUpdates(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)

	err := q.Where("id = ?", 123).Updates(t.Context(), map[string]any{
		"name": "new name",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := tq.LastQuery()
	want := "UPDATE `users` SET `name` = ? WHERE id = ?"
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
	if len(got.Args) != 2 || got.Args[0] != "new name" || got.Args[1] != 123 {
		t.Errorf("Args = %v", got.Args)
	}
}

func TestUpdatesPostgreSQL(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.PostgreSQL)
	q := newTestQuery(tq)

	err := q.Where("id = ?", 123).Updates(t.Context(), map[string]any{
		"name": "new name",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := tq.LastQuery()
	want := `UPDATE "users" SET "name" = $1 WHERE id = $2`
	if got.SQL != want {
		t.Errorf("SQL = %q, want %q", got.SQL, want)
	}
}

func TestUpdatesAutoUpdatedAt(t *testing.T) {
	t.Parallel()

	fixed := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	ctx := orm.WithClock(t.Context(), fixedClock{t: fixed})

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestArticleQuery(tq)

	err := q.Where("id = ?", 1).Updates(ctx, map[string]any{
		"title": "updated title",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := tq.LastQuery()
	// Should contain both title and updated_at in SET clause
	if !strings.Contains(got.SQL, "`updated_at` = ?") {
		t.Errorf("SQL should contain updated_at: %q", got.SQL)
	}
	if !strings.Contains(got.SQL, "`title` = ?") {
		t.Errorf("SQL should contain title: %q", got.SQL)
	}

	// Verify updated_at value is the fixed time
	foundUpdatedAt := false
	for _, arg := range got.Args {
		if ts, ok := arg.(time.Time); ok && ts.Equal(fixed) {
			foundUpdatedAt = true
			break
		}
	}
	if !foundUpdatedAt {
		t.Errorf("Args should contain fixed time %v: %v", fixed, got.Args)
	}
}

func TestUpdatesWithoutWhereReturnsError(t *testing.T) {
	t.Parallel()

	tq := orm.NewTestQuerier(orm.MySQL)
	q := newTestQuery(tq)

	err := q.Updates(t.Context(), map[string]any{"name": "oops"})
	if err == nil {
		t.Fatal("expected error for Updates without WHERE, got nil")
	}
}
