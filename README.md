# ormgen

A lightweight, type-safe ORM for Go — powered by code generation and generics, with zero runtime reflection.

## Features

- **Code generation** — generates type-specific query helpers from your struct definitions
- **Type-safe queries** — built on Go generics (`Query[T]`)
- **Zero reflection** — all scanning, column mapping, and preloading are resolved at compile time
- **Immutable query builder** — every builder method returns a new `Query`, safe to reuse
- **MySQL & PostgreSQL** — dialect abstraction handles placeholder style, identifier quoting, and `RETURNING`
- **Relations** — `has_many`, `has_one`, `belongs_to`, `many_to_many` with eager loading (Preload) and JOIN support
- **Scopes** — composable, reusable query fragments (`Where`, `OrderBy`, `Limit`, `Offset`, `In`)
- **Transactions** — `DB.Transaction` with automatic commit/rollback/panic-recovery

## Philosophy

ormgen is for developers who want to **write SQL-aware Go code without the boilerplate**.

|             | gorm                                | sqlc                       | ormgen                           |
|-------------|-------------------------------------|----------------------------|----------------------------------|
| Approach    | Runtime reflection                  | SQL-first codegen          | Struct-first codegen             |
| Input       | Go structs + conventions            | `.sql` files               | Go structs + tags                |
| Magic       | Hooks, soft delete, auto-timestamps | None                       | None                             |
| Debugging   | Trace callbacks                     | Read the SQL               | Read the generated Go code       |

**Why not gorm?**

gorm is powerful and batteries-included. But its implicit behaviors — auto-timestamps, soft delete when `DeletedAt`
exists, hook chains, association auto-save — can make it hard to predict what SQL actually runs. ormgen generates all
query logic as plain Go code you can open and read.

**Why not sqlc?**

sqlc is great if you prefer writing raw SQL. ormgen takes the opposite approach: you define Go structs, and the tool
generates the query layer. If you prefer thinking in Go types over `.sql` files, ormgen may be a better fit.

**What ormgen is not:**

- Not a migration tool — use other tools, or plain SQL
- Not a full-featured ORM — no auto-timestamps, no soft delete, no callback hooks
- Not magic — if something happens, it's because your code explicitly asked for it

## Installation

```bash
go install github.com/mickamy/ormgen@latest
```

Requires Go 1.24+.

## Quick Start

### 1. Define your model

```go
package model

import "time"

//go:generate ormgen -source=$GOFILE

type User struct {
    ID        int
    Name      string
    Email     string
    CreatedAt time.Time
    Posts     []Post   `rel:"has_many,foreign_key:user_id"`
    Profile   *Profile `rel:"has_one,foreign_key:user_id"`
}

type Post struct {
    ID     int
    UserID int
    Title  string
    Body   string
    User   *User `rel:"belongs_to,foreign_key:user_id"`
}
```

- Exported fields are automatically mapped to snake_case columns (`CreatedAt` -> `created_at`)
- A field named `ID` is assumed to be the primary key
- Struct, pointer-to-struct, and slice fields are automatically skipped as DB columns

### 2. Generate

```bash
go generate ./...
```

This creates `user_gen.go` alongside the source file containing:

- `Users(db) *orm.Query[User]` — factory function
- `Posts(db) *orm.Query[Post]` — factory function
- Per-type scan, column-value, set-PK, and preloader helpers

To generate into a separate package:

```bash
//go:generate ormgen -source=$GOFILE -destination=../query
```

### 3. Use

```go
package main

import (
    "context"
    "database/sql"

    _ "github.com/go-sql-driver/mysql"

    "yourapp/model"
    "yourapp/query"
    "github.com/mickamy/ormgen/orm"
    "github.com/mickamy/ormgen/scope"
)

func main() {
    sqlDB, _ := sql.Open("mysql", "root:root@tcp(127.0.0.1:3306)/mydb?parseTime=true")
    db := orm.New(sqlDB, orm.MySQL) // or orm.PostgreSQL

    ctx := context.Background()

    // Create
    u := &model.User{Name: "Alice", Email: "alice@example.com"}
    query.Users(db).Create(ctx, u)
    // u.ID is now populated

    // Find
    user, _ := query.Users(db).Where("id = ?", u.ID).First(ctx)

    // Find all with scopes
    users, _ := query.Users(db).
        Scopes(scope.Where("name LIKE ?", "A%"), scope.Limit(10)).
        OrderBy("id").
        All(ctx)

    // Preload relations
    users, _ = query.Users(db).Preload("Posts").Preload("Profile").All(ctx)

    // Join
    users, _ = query.Users(db).Join("Posts").Select("DISTINCT users.*").All(ctx)

    // Count / Exists
    count, _ := query.Users(db).Count(ctx)
    exists, _ := query.Users(db).Where("email = ?", "alice@example.com").Exists(ctx)

    // Update
    user.Name = "Alice Updated"
    query.Users(db).Update(ctx, &user)

    // Delete
    query.Users(db).Where("id = ?", u.ID).Delete(ctx)

    // Batch insert
    posts := []*model.Post{
        {UserID: u.ID, Title: "Post 1", Body: "body"},
        {UserID: u.ID, Title: "Post 2", Body: "body"},
    }
    query.Posts(db).CreateAll(ctx, posts)

    // Upsert
    query.Posts(db).Upsert(ctx, posts[0])

    // Transaction
    db.Transaction(ctx, func(tx *orm.Tx) error {
        query.Users(tx).Create(ctx, &model.User{Name: "Bob"})
        return nil // commit; return error to rollback
    })
}
```

## Struct Tags

### `db` tag — column mapping

| Tag                | Behavior                                                      |
|--------------------|---------------------------------------------------------------|
| *(no tag)*         | Column inferred from field name (`CreatedAt` -> `created_at`) |
| `db:"col_name"`    | Explicit column name                                          |
| `db:",primaryKey"` | Mark as primary key (default: field named `ID`)               |
| `db:"-"`           | Exclude from DB columns                                       |

### `rel` tag — relations

| Relation     | Field type | Tag                                                                             |
|--------------|------------|---------------------------------------------------------------------------------|
| has_many     | `[]Post`   | `rel:"has_many,foreign_key:user_id"`                                            |
| has_one      | `*Profile` | `rel:"has_one,foreign_key:user_id"`                                             |
| belongs_to   | `*User`    | `rel:"belongs_to,foreign_key:user_id"`                                          |
| many_to_many | `[]Tag`    | `rel:"many_to_many,join_table:user_tags,foreign_key:user_id,references:tag_id"` |

## Query API

### Builder methods (return new `Query[T]`)

| Method                   | Description                  |
|--------------------------|------------------------------|
| `Where(clause, args...)` | Add WHERE condition          |
| `OrderBy(clause)`        | Add ORDER BY                 |
| `Limit(n)`               | Set LIMIT                    |
| `Offset(n)`              | Set OFFSET                   |
| `Select(columns)`        | Override SELECT columns      |
| `Join(name)`             | INNER JOIN on named relation |
| `LeftJoin(name)`         | LEFT JOIN on named relation  |
| `Preload(name)`          | Eager load named relation    |
| `Scopes(scopes...)`      | Apply reusable scope objects |

### Terminal methods (execute query)

| Method                 | Description                                                |
|------------------------|------------------------------------------------------------|
| `All(ctx)`             | `([]T, error)` — fetch all matching rows                   |
| `First(ctx)`           | `(T, error)` — fetch first row (`orm.ErrNotFound` if none) |
| `Count(ctx)`           | `(int64, error)` — count matching rows                     |
| `Exists(ctx)`          | `(bool, error)` — check if any row matches                 |
| `Create(ctx, *T)`      | Insert and populate PK                                     |
| `CreateAll(ctx, []*T)` | Batch insert and populate PKs                              |
| `Upsert(ctx, *T)`      | Insert or update on PK conflict                            |
| `Update(ctx, *T)`      | Update by PK                                               |
| `Delete(ctx)`          | Delete matching rows (requires WHERE)                      |

## Scopes

Scopes are composable, reusable query fragments:

```go
import "github.com/mickamy/ormgen/scope"

// Reusable scope
active := scope.Where("active = ?", true)
recent := scope.OrderBy("created_at DESC")
page := scope.Combine(scope.Limit(20), scope.Offset(0))

users, _ := query.Users(db).Scopes(active, recent).Scopes(page...).All(ctx)

// Generic In
ids := []int{1, 2, 3}
users, _ := query.Users(db).Scopes(scope.In("id", ids)).All(ctx)
```

### Why scopes matter — the Repository pattern

Without scopes, repositories tend to grow like this:

```go
func (r *UserRepo) Get(ctx context.Context, id string) (User, error)
func (r *UserRepo) GetWithPosts(ctx context.Context, id string) (User, error)
func (r *UserRepo) GetWithProfile(ctx context.Context, id string) (User, error)
func (r *UserRepo) GetForSignIn(ctx context.Context, id string) (User, error)
func (r *UserRepo) ListAdmins(ctx context.Context, page int) ([]User, error)
func (r *UserRepo) ListRecent(ctx context.Context, limit int) ([]User, error)
// ... and so on for every combination
```

With scopes, a single method covers all of these:

```go
type UserRepo struct{ db orm.Querier }

func (r *UserRepo) Get(ctx context.Context, id int, scopes ...scope.Scope) (User, error) {
    return query.Users(r.db).Where("id = ?", id).Scopes(scopes...).First(ctx)
}

func (r *UserRepo) List(ctx context.Context, scopes ...scope.Scope) ([]User, error) {
    return query.Users(r.db).Scopes(scopes...).OrderBy("id").All(ctx)
}
```

The caller decides how to filter:

```go
// Just the user
user, _ := repo.Get(ctx, 1)

// List with filtering + pagination
users, _ := repo.List(ctx,
    scope.Where("role = ?", "admin"),
    scope.OrderBy("created_at DESC"),
    scope.Limit(20),
    scope.Offset(page * 20),
)
```

## CLI

```
ormgen -source=<path> [-destination=<dir>] [-version]
```

| Flag           | Description                                |
|----------------|--------------------------------------------|
| `-source`      | Source `.go` file (required)               |
| `-destination` | Output directory (default: same as source) |
| `-version`     | Print version                              |

Table names are auto-inferred: `User` -> `users`, `UserProfile` -> `user_profiles`.

## Development

```bash
# Start MySQL & PostgreSQL
make up-d

# Run unit tests
make test

# Run integration tests (requires running DBs)
make itest

# Lint
make lint
```

## License

[MIT](./LICENSE)
