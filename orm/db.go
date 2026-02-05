package orm

import (
	"context"
	"database/sql"
)

// Querier is the common interface for DB and Tx.
// Generated factory functions accept this so that queries work with both.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	dialect() Dialect
}

// DB wraps *sql.DB with a Dialect and satisfies Querier.
type DB struct {
	raw *sql.DB
	d   Dialect
}

// New wraps a *sql.DB with the given Dialect.
func New(db *sql.DB, d Dialect) *DB {
	return &DB{raw: db, d: d}
}

func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.raw.QueryContext(ctx, query, args...) //nolint:wrapcheck // thin wrapper
}

func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.raw.ExecContext(ctx, query, args...) //nolint:wrapcheck // thin wrapper
}

// Begin starts a transaction.
func (db *DB) Begin(ctx context.Context) (*Tx, error) {
	tx, err := db.raw.BeginTx(ctx, nil)
	if err != nil {
		return nil, err //nolint:wrapcheck // thin wrapper
	}
	return &Tx{raw: tx, d: db.d}, nil
}

func (db *DB) dialect() Dialect { return db.d }

// Tx wraps *sql.Tx with a Dialect and satisfies Querier.
type Tx struct {
	raw *sql.Tx
	d   Dialect
}

func (tx *Tx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return tx.raw.QueryContext(ctx, query, args...) //nolint:wrapcheck // thin wrapper
}

func (tx *Tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return tx.raw.ExecContext(ctx, query, args...) //nolint:wrapcheck // thin wrapper
}

// Commit commits the transaction.
func (tx *Tx) Commit() error { return tx.raw.Commit() } //nolint:wrapcheck // thin wrapper

// Rollback rolls back the transaction.
func (tx *Tx) Rollback() error { return tx.raw.Rollback() } //nolint:wrapcheck // thin wrapper

func (tx *Tx) dialect() Dialect { return tx.d }
