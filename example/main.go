package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/mickamy/ormgen/example/model"
	"github.com/mickamy/ormgen/example/repo"
	"github.com/mickamy/ormgen/orm"
	"github.com/mickamy/ormgen/scope"
)

var createTableMySQL = `CREATE TABLE users (
	id INT AUTO_INCREMENT PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	email VARCHAR(255) NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

var createTablePostgreSQL = `CREATE TABLE users (
	id SERIAL PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	email VARCHAR(255) NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`

func main() {
	dialect := flag.String("dialect", "mysql", "database dialect (mysql or postgres)")
	flag.Parse()

	ctx := context.Background()

	db, createTableSQL := openDB(*dialect)

	// CREATE TABLE
	fmt.Println("--- CREATE TABLE ---")
	if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS users"); err != nil {
		log.Fatalf("drop table: %v", err)
	}
	if _, err := db.ExecContext(ctx, createTableSQL); err != nil {
		log.Fatalf("create table: %v", err)
	}
	fmt.Println("Table 'users' created.")

	userRepo := repo.NewUserRepository(db)

	// INSERT
	fmt.Println("\n--- INSERT ---")
	now := time.Now()
	names := []string{"Alice", "Bob", "Charlie", "Diana", "Eve"}
	users := make([]*model.User, len(names))
	for i, name := range names {
		u := &model.User{
			Name:      name,
			Email:     fmt.Sprintf("%s@example.com", name),
			CreatedAt: now,
		}
		if err := userRepo.Create(ctx, u); err != nil {
			log.Fatalf("create %s: %v", name, err)
		}
		users[i] = u
		fmt.Printf("Created: ID=%d Name=%s\n", u.ID, u.Name)
	}

	// SELECT (all)
	fmt.Println("\n--- SELECT ALL ---")
	all, err := userRepo.FindAll(ctx)
	if err != nil {
		log.Fatalf("find all: %v", err)
	}
	for _, u := range all {
		fmt.Printf("  ID=%d Name=%s Email=%s\n", u.ID, u.Name, u.Email)
	}

	// SELECT (by ID)
	fmt.Println("\n--- SELECT BY ID ---")
	found, err := userRepo.FindByID(ctx, users[0].ID)
	if err != nil {
		log.Fatalf("find by ID: %v", err)
	}
	fmt.Printf("Found: ID=%d Name=%s\n", found.ID, found.Name)

	// UPDATE
	fmt.Println("\n--- UPDATE ---")
	users[0].Name = "Alice Updated"
	users[0].Email = "alice.updated@example.com"
	if err := userRepo.Update(ctx, users[0]); err != nil {
		log.Fatalf("update: %v", err)
	}
	updated, err := userRepo.FindByID(ctx, users[0].ID)
	if err != nil {
		log.Fatalf("find after update: %v", err)
	}
	fmt.Printf("Updated: ID=%d Name=%s Email=%s\n", updated.ID, updated.Name, updated.Email)

	// DELETE
	fmt.Println("\n--- DELETE ---")
	if err := userRepo.Delete(ctx, users[1].ID); err != nil {
		log.Fatalf("delete: %v", err)
	}
	fmt.Printf("Deleted user with ID=%d\n", users[1].ID)

	// SCOPES
	fmt.Println("\n--- SCOPES ---")

	// Paginate with Limit + Offset
	fmt.Println("Paginate (limit=2, offset=1):")
	paginate := scope.Combine(scope.Limit(2), scope.Offset(1))
	page, err := userRepo.FindAll(ctx, paginate...)
	if err != nil {
		log.Fatalf("paginate: %v", err)
	}
	for _, u := range page {
		fmt.Printf("  ID=%d Name=%s\n", u.ID, u.Name)
	}

	// Filter with Where
	fmt.Println("Where (name LIKE 'A%%'):")
	filtered, err := userRepo.FindAll(ctx, scope.Where("name LIKE ?", "A%"))
	if err != nil {
		log.Fatalf("where: %v", err)
	}
	for _, u := range filtered {
		fmt.Printf("  ID=%d Name=%s\n", u.ID, u.Name)
	}

	// Filter with In
	fmt.Println("In (id IN ...):")
	ids := []int{users[0].ID, users[2].ID, users[4].ID}
	inResult, err := userRepo.FindAll(ctx, scope.In("id", ids))
	if err != nil {
		log.Fatalf("in: %v", err)
	}
	for _, u := range inResult {
		fmt.Printf("  ID=%d Name=%s\n", u.ID, u.Name)
	}
}

func openDB(dialect string) (*orm.DB, string) {
	switch dialect {
	case "mysql":
		sqlDB, err := sql.Open("mysql", "root:root@tcp(127.0.0.1:3306)/ormgen_test?parseTime=true")
		if err != nil {
			log.Fatalf("open mysql: %v", err)
		}
		return orm.New(sqlDB, orm.MySQL), createTableMySQL
	case "postgres":
		sqlDB, err := sql.Open("pgx", "postgres://postgres:postgres@127.0.0.1:5432/ormgen_test?sslmode=disable")
		if err != nil {
			log.Fatalf("open postgres: %v", err)
		}
		return orm.New(sqlDB, orm.PostgreSQL), createTablePostgreSQL
	default:
		log.Fatalf("unknown dialect: %s (use 'mysql' or 'postgres')", dialect)
		return nil, ""
	}
}
