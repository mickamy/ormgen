package orm

import (
	"context"
	"database/sql"
	"errors"
)

// TestQuerier is a mock Querier that records executed queries.
// Exported for use in orm_test package.
type TestQuerier struct {
	D       Dialect
	Queries []TestQuery
}

// TestQuery holds a captured query string and its args.
type TestQuery struct {
	SQL  string
	Args []any
}

// NewTestQuerier creates a TestQuerier with the given Dialect.
func NewTestQuerier(d Dialect) *TestQuerier {
	return &TestQuerier{D: d}
}

func (tq *TestQuerier) QueryContext(_ context.Context, query string, args ...any) (*sql.Rows, error) {
	tq.Queries = append(tq.Queries, TestQuery{query, args})
	return nil, errors.New("mock: not implemented")
}

func (tq *TestQuerier) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	tq.Queries = append(tq.Queries, TestQuery{query, args})
	return testResult{}, nil
}

func (tq *TestQuerier) dialect() Dialect { return tq.D }

var _ Querier = (*TestQuerier)(nil)

// LastQuery returns the most recently captured query, or panics if empty.
func (tq *TestQuerier) LastQuery() TestQuery {
	return tq.Queries[len(tq.Queries)-1]
}

type testResult struct{}

func (testResult) LastInsertId() (int64, error) { return 0, nil }
func (testResult) RowsAffected() (int64, error) { return 0, nil }
