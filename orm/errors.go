package orm

import "errors"

// ErrNotFound is returned when a query expects exactly one row but finds none.
var ErrNotFound = errors.New("orm: not found")

// ErrNoReturningResult is returned when INSERT RETURNING produces no rows.
var ErrNoReturningResult = errors.New("orm: INSERT RETURNING returned no rows")

// ErrMissingPrimaryKey is returned when Update is called without a primary key value.
var ErrMissingPrimaryKey = errors.New("orm: primary key value is required for Update")

// ErrDeleteWithoutWhere is returned when Delete is called without any WHERE clause.
var ErrDeleteWithoutWhere = errors.New("orm: Delete without WHERE clause is not allowed")
