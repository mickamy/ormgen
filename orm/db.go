package orm

import (
	"context"
	"database/sql"
	"log/slog"
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
	raw   *sql.DB
	d     Dialect
	debug bool
}

// New wraps a *sql.DB with the given Dialect.
func New(db *sql.DB, d Dialect) *DB {
	return &DB{raw: db, d: d}
}

// Debug returns a new *DB that logs every query via slog.DebugContext.
// The original DB is not modified.
func (db *DB) Debug() *DB {
	return &DB{raw: db.raw, d: db.d, debug: true}
}

func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if db.debug {
		slog.DebugContext(ctx, "ormgen", "query", query, "args", args)
	}
	return db.raw.QueryContext(ctx, query, args...) //nolint:wrapcheck // thin wrapper
}

func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if db.debug {
		slog.DebugContext(ctx, "ormgen", "query", query, "args", args)
	}
	return db.raw.ExecContext(ctx, query, args...) //nolint:wrapcheck // thin wrapper
}

// Begin starts a transaction.
func (db *DB) Begin(ctx context.Context) (*Tx, error) {
	tx, err := db.raw.BeginTx(ctx, nil)
	if err != nil {
		return nil, err //nolint:wrapcheck // thin wrapper
	}
	return &Tx{raw: tx, d: db.d, debug: db.debug}, nil
}

// Transaction executes fn within a transaction.
// If fn returns nil the transaction is committed.
// If fn returns an error or panics the transaction is rolled back.
func (db *DB) Transaction(ctx context.Context, fn func(tx *Tx) error) (err error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	err = fn(tx)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Close closes the underlying *sql.DB.
func (db *DB) Close() error { return db.raw.Close() } //nolint:wrapcheck // thin wrapper

func (db *DB) dialect() Dialect { return db.d }

// Tx wraps *sql.Tx with a Dialect and satisfies Querier.
type Tx struct {
	raw   *sql.Tx
	d     Dialect
	debug bool
}

func (tx *Tx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if tx.debug {
		slog.DebugContext(ctx, "ormgen", "query", query, "args", args)
	}
	return tx.raw.QueryContext(ctx, query, args...) //nolint:wrapcheck // thin wrapper
}

func (tx *Tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if tx.debug {
		slog.DebugContext(ctx, "ormgen", "query", query, "args", args)
	}
	return tx.raw.ExecContext(ctx, query, args...) //nolint:wrapcheck // thin wrapper
}

// Commit commits the transaction.
func (tx *Tx) Commit() error { return tx.raw.Commit() } //nolint:wrapcheck // thin wrapper

// Rollback rolls back the transaction.
func (tx *Tx) Rollback() error { return tx.raw.Rollback() } //nolint:wrapcheck // thin wrapper

func (tx *Tx) dialect() Dialect { return tx.d }
