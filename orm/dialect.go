package orm

import "fmt"

// Dialect abstracts SQL differences between database engines.
type Dialect interface {
	// Placeholder returns the bind parameter placeholder for the given
	// 1-based index. MySQL returns "?" regardless of index; PostgreSQL
	// returns "$1", "$2", etc.
	Placeholder(index int) string

	// QuoteIdent quotes an identifier (table name, column name) to safely
	// handle SQL reserved words. MySQL uses backticks; PostgreSQL uses
	// double quotes.
	QuoteIdent(name string) string

	// UseReturning reports whether INSERT should use a RETURNING clause
	// to retrieve the auto-generated primary key (PostgreSQL) rather
	// than relying on LastInsertId (MySQL).
	UseReturning() bool

	// ReturningClause returns the RETURNING clause appended to INSERT
	// statements. Returns an empty string for dialects that do not
	// support RETURNING (MySQL).
	ReturningClause(pk string) string
}

// MySQL is the Dialect for MySQL / MariaDB.
var MySQL Dialect = mysqlDialect{}

// PostgreSQL is the Dialect for PostgreSQL.
var PostgreSQL Dialect = postgresDialect{}

type mysqlDialect struct{}

func (mysqlDialect) Placeholder(_ int) string        { return "?" }
func (mysqlDialect) QuoteIdent(name string) string   { return "`" + name + "`" }
func (mysqlDialect) UseReturning() bool              { return false }
func (mysqlDialect) ReturningClause(_ string) string { return "" }

type postgresDialect struct{}

func (postgresDialect) Placeholder(index int) string     { return fmt.Sprintf("$%d", index) }
func (postgresDialect) QuoteIdent(name string) string    { return `"` + name + `"` }
func (postgresDialect) UseReturning() bool               { return true }
func (postgresDialect) ReturningClause(pk string) string { return ` RETURNING "` + pk + `"` }
