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
	alice := &model.User{Name: "Alice", Email: "alice@example.com", CreatedAt: now}
	if err := userRepo.Create(ctx, alice); err != nil {
		log.Fatalf("create Alice: %v", err)
	}
	fmt.Printf("Created: %+v\n", *alice)

	bob := &model.User{Name: "Bob", Email: "bob@example.com", CreatedAt: now}
	if err := userRepo.Create(ctx, bob); err != nil {
		log.Fatalf("create Bob: %v", err)
	}
	fmt.Printf("Created: %+v\n", *bob)

	// SELECT (all)
	fmt.Println("\n--- SELECT ALL ---")
	all, err := userRepo.FindAll(ctx)
	if err != nil {
		log.Fatalf("find all: %v", err)
	}
	for _, u := range all {
		fmt.Printf("  %+v\n", u)
	}

	// SELECT (by ID)
	fmt.Println("\n--- SELECT BY ID ---")
	found, err := userRepo.FindByID(ctx, alice.ID)
	if err != nil {
		log.Fatalf("find by ID: %v", err)
	}
	fmt.Printf("Found: %+v\n", found)

	// UPDATE
	fmt.Println("\n--- UPDATE ---")
	alice.Name = "Alice Updated"
	alice.Email = "alice.updated@example.com"
	if err := userRepo.Update(ctx, alice); err != nil {
		log.Fatalf("update Alice: %v", err)
	}
	updated, err := userRepo.FindByID(ctx, alice.ID)
	if err != nil {
		log.Fatalf("find after update: %v", err)
	}
	fmt.Printf("Updated: %+v\n", updated)

	// DELETE
	fmt.Println("\n--- DELETE ---")
	if err := userRepo.Delete(ctx, bob.ID); err != nil {
		log.Fatalf("delete Bob: %v", err)
	}
	fmt.Printf("Deleted user with ID=%d\n", bob.ID)

	remaining, err := userRepo.FindAll(ctx)
	if err != nil {
		log.Fatalf("find all after delete: %v", err)
	}
	fmt.Printf("Remaining users: %d\n", len(remaining))
	for _, u := range remaining {
		fmt.Printf("  %+v\n", u)
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
