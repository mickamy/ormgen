//go:build integration

package orm_test

import (
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/mickamy/ormgen/orm"
	"github.com/mickamy/ormgen/scope"
)

type User struct {
	ID    int
	Name  string
	Email string
}

var usersColumns = []string{"id", "name", "email"}

func scanUser(rows *sql.Rows) (User, error) {
	cols, _ := rows.Columns()
	var v User
	dest := make([]any, len(cols))
	for i, col := range cols {
		switch col {
		case "id":
			dest[i] = &v.ID
		case "name":
			dest[i] = &v.Name
		case "email":
			dest[i] = &v.Email
		default:
			dest[i] = new(any)
		}
	}
	err := rows.Scan(dest...)
	return v, err
}

func userColumnValuePairs(v *User, includesPK bool) ([]string, []any) {
	if includesPK {
		return []string{"id", "name", "email"},
			[]any{v.ID, v.Name, v.Email}
	}
	return []string{"name", "email"},
		[]any{v.Name, v.Email}
}

func setUserPK(v *User, id int64) {
	v.ID = int(id)
}

func Users(db orm.Querier) *orm.Query[User] {
	return orm.NewQuery[User](db, "users", usersColumns, "id", scanUser, userColumnValuePairs, setUserPK)
}

type dialectSetup struct {
	name    string
	driver  string
	dsn     string
	dialect orm.Dialect
	createTable string
}

var dialects = []dialectSetup{
	{
		name:    "MySQL",
		driver:  "mysql",
		dsn:     "root:root@tcp(127.0.0.1:3306)/ormgen_test?parseTime=true",
		dialect: orm.MySQL,
		createTable: `CREATE TABLE IF NOT EXISTS users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL
		)`,
	},
	{
		name:    "PostgreSQL",
		driver:  "pgx",
		dsn:     "postgres://postgres:postgres@127.0.0.1:5432/ormgen_test?sslmode=disable",
		dialect: orm.PostgreSQL,
		createTable: `CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL
		)`,
	},
}

func setupDB(t *testing.T, ds dialectSetup) orm.Querier {
	t.Helper()

	sqlDB, err := sql.Open(ds.driver, ds.dsn)
	if err != nil {
		t.Fatalf("open %s: %v", ds.name, err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if _, err := sqlDB.Exec(ds.createTable); err != nil {
		t.Fatalf("create table %s: %v", ds.name, err)
	}

	// Clean up before each test.
	if _, err := sqlDB.Exec("DELETE FROM users"); err != nil {
		t.Fatalf("truncate %s: %v", ds.name, err)
	}

	return orm.New(sqlDB, ds.dialect)
}

func TestCRUD(t *testing.T) {
	for _, ds := range dialects {
		t.Run(ds.name, func(t *testing.T) {
			db := setupDB(t, ds)
			ctx := t.Context()

			// Create
			u := &User{Name: "Alice", Email: "alice@example.com"}
			if err := Users(db).Create(ctx, u); err != nil {
				t.Fatalf("Create: %v", err)
			}
			if u.ID == 0 {
				t.Fatal("expected ID to be set after Create")
			}

			// First
			got, err := Users(db).Where("id = ?", u.ID).First(ctx)
			if err != nil {
				t.Fatalf("First: %v", err)
			}
			if got.Name != "Alice" || got.Email != "alice@example.com" {
				t.Errorf("First = %+v", got)
			}

			// Update
			u.Name = "Bob"
			if err := Users(db).Update(ctx, u); err != nil {
				t.Fatalf("Update: %v", err)
			}
			got, err = Users(db).Where("id = ?", u.ID).First(ctx)
			if err != nil {
				t.Fatalf("First after Update: %v", err)
			}
			if got.Name != "Bob" {
				t.Errorf("Name = %q, want %q", got.Name, "Bob")
			}

			// Delete
			if err := Users(db).Where("id = ?", u.ID).Delete(ctx); err != nil {
				t.Fatalf("Delete: %v", err)
			}
			_, err = Users(db).Where("id = ?", u.ID).First(ctx)
			if err != orm.ErrNotFound {
				t.Errorf("expected ErrNotFound after Delete, got %v", err)
			}
		})
	}
}

func TestAll(t *testing.T) {
	for _, ds := range dialects {
		t.Run(ds.name, func(t *testing.T) {
			db := setupDB(t, ds)
			ctx := t.Context()

			for i := range 3 {
				u := &User{Name: fmt.Sprintf("user%d", i), Email: fmt.Sprintf("user%d@example.com", i)}
				if err := Users(db).Create(ctx, u); err != nil {
					t.Fatalf("Create: %v", err)
				}
			}

			users, err := Users(db).OrderBy("id").All(ctx)
			if err != nil {
				t.Fatalf("All: %v", err)
			}
			if len(users) != 3 {
				t.Fatalf("len(All) = %d, want 3", len(users))
			}

			// Limit + Offset
			users, err = Users(db).OrderBy("id").Limit(2).Offset(1).All(ctx)
			if err != nil {
				t.Fatalf("All with Limit/Offset: %v", err)
			}
			if len(users) != 2 {
				t.Fatalf("len = %d, want 2", len(users))
			}
			if users[0].Name != "user1" {
				t.Errorf("users[0].Name = %q, want %q", users[0].Name, "user1")
			}
		})
	}
}

func TestScopes(t *testing.T) {
	for _, ds := range dialects {
		t.Run(ds.name, func(t *testing.T) {
			db := setupDB(t, ds)
			ctx := t.Context()

			for i := range 5 {
				u := &User{Name: fmt.Sprintf("user%d", i), Email: fmt.Sprintf("user%d@example.com", i)}
				if err := Users(db).Create(ctx, u); err != nil {
					t.Fatalf("Create: %v", err)
				}
			}

			paginate := scope.Combine(scope.Limit(2), scope.Offset(1))
			users, err := Users(db).Scopes(paginate...).OrderBy("id").All(ctx)
			if err != nil {
				t.Fatalf("All with Scopes: %v", err)
			}
			if len(users) != 2 {
				t.Fatalf("len = %d, want 2", len(users))
			}
		})
	}
}

func TestTransaction(t *testing.T) {
	for _, ds := range dialects {
		t.Run(ds.name, func(t *testing.T) {
			db := setupDB(t, ds)
			ctx := t.Context()

			ormDB, ok := db.(*orm.DB)
			if !ok {
				t.Fatal("expected *orm.DB")
			}

			// Commit
			tx, err := ormDB.Begin(ctx)
			if err != nil {
				t.Fatalf("Begin: %v", err)
			}
			u := &User{Name: "TxUser", Email: "tx@example.com"}
			if err := Users(tx).Create(ctx, u); err != nil {
				t.Fatalf("Create in tx: %v", err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatalf("Commit: %v", err)
			}
			got, err := Users(db).Where("id = ?", u.ID).First(ctx)
			if err != nil {
				t.Fatalf("First after commit: %v", err)
			}
			if got.Name != "TxUser" {
				t.Errorf("Name = %q, want %q", got.Name, "TxUser")
			}

			// Rollback
			tx2, err := ormDB.Begin(ctx)
			if err != nil {
				t.Fatalf("Begin: %v", err)
			}
			u2 := &User{Name: "RollbackUser", Email: "rollback@example.com"}
			if err := Users(tx2).Create(ctx, u2); err != nil {
				t.Fatalf("Create in tx2: %v", err)
			}
			if err := tx2.Rollback(); err != nil {
				t.Fatalf("Rollback: %v", err)
			}
			_, err = Users(db).Where("name = ?", "RollbackUser").First(ctx)
			if err != orm.ErrNotFound {
				t.Errorf("expected ErrNotFound after rollback, got %v", err)
			}
		})
	}
}
