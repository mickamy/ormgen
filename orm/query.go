package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/mickamy/ormgen/scope"
)

// ScanFunc scans a single row into T.
// Generated per-type by ormgen.
type ScanFunc[T any] func(rows *sql.Rows) (T, error)

// ColumnValueFunc extracts column names and their values from a *T.
// When includesPK is false the primary key column is excluded (for INSERT
// with auto-increment).
type ColumnValueFunc[T any] func(t *T, includesPK bool) (columns []string, values []any)

// SetPKFunc sets the auto-generated primary key on *T after INSERT.
// May be nil when the primary key is not auto-generated.
type SetPKFunc[T any] func(t *T, id int64)

// PreloaderFunc executes a preload query and assigns results to the parent slice.
// Generated per-relation by ormgen.
type PreloaderFunc[T any] func(ctx context.Context, db Querier, results []T) error

// JoinConfig holds the metadata needed to build a JOIN clause at runtime.
type JoinConfig struct {
	TargetTable  string
	TargetColumn string
	SourceTable  string
	SourceColumn string
}

// Query represents a pending query against a single table.
// All builder methods return a new Query; the receiver is never modified.
type Query[T any] struct {
	db          Querier
	table       string
	columns     []string
	pk          string
	scan        ScanFunc[T]
	colValPairs ColumnValueFunc[T]
	setPK       SetPKFunc[T]

	wheres   []whereClause
	orderBys []string
	joins    []string
	selects  *string
	limit    *int
	offset   *int

	joinDefs   map[string]JoinConfig
	preloaders map[string]PreloaderFunc[T]
	preloads   []string
}

type whereClause struct {
	clause string
	args   []any
}

// NewQuery is called by generated factory functions.
func NewQuery[T any](
	db Querier,
	table string,
	columns []string,
	pk string,
	scan ScanFunc[T],
	colValPairs ColumnValueFunc[T],
	setPK SetPKFunc[T],
) *Query[T] {
	return &Query[T]{
		db:          db,
		table:       table,
		columns:     columns,
		pk:          pk,
		scan:        scan,
		colValPairs: colValPairs,
		setPK:       setPK,
	}
}

// RegisterJoin registers a named join definition for use with Join/LeftJoin.
func (q *Query[T]) RegisterJoin(name string, cfg JoinConfig) {
	if q.joinDefs == nil {
		q.joinDefs = make(map[string]JoinConfig)
	}
	q.joinDefs[name] = cfg
}

// RegisterPreloader registers a named preloader for use with Preload.
func (q *Query[T]) RegisterPreloader(name string, fn PreloaderFunc[T]) {
	if q.preloaders == nil {
		q.preloaders = make(map[string]PreloaderFunc[T])
	}
	q.preloaders[name] = fn
}

// clone returns a shallow copy with slices copied to avoid aliasing.
func (q *Query[T]) clone() *Query[T] {
	q2 := *q
	q2.wheres = append([]whereClause(nil), q.wheres...)
	q2.orderBys = append([]string(nil), q.orderBys...)
	q2.joins = append([]string(nil), q.joins...)
	q2.preloads = append([]string(nil), q.preloads...)
	return &q2
}

// --- Builder methods ---

func (q *Query[T]) Where(clause string, args ...any) *Query[T] {
	q2 := q.clone()
	q2.wheres = append(q2.wheres, whereClause{clause, args})
	return q2
}

func (q *Query[T]) OrderBy(clause string) *Query[T] {
	q2 := q.clone()
	q2.orderBys = append(q2.orderBys, clause)
	return q2
}

func (q *Query[T]) Limit(n int) *Query[T] {
	q2 := q.clone()
	q2.limit = &n
	return q2
}

func (q *Query[T]) Offset(n int) *Query[T] {
	q2 := q.clone()
	q2.offset = &n
	return q2
}

func (q *Query[T]) Select(columns string) *Query[T] {
	q2 := q.clone()
	q2.selects = &columns
	return q2
}

// Join adds an INNER JOIN for the named relation.
func (q *Query[T]) Join(name string) *Query[T] {
	return q.addJoin("INNER JOIN", name)
}

// LeftJoin adds a LEFT JOIN for the named relation.
func (q *Query[T]) LeftJoin(name string) *Query[T] {
	return q.addJoin("LEFT JOIN", name)
}

func (q *Query[T]) addJoin(joinType, name string) *Query[T] {
	cfg, ok := q.joinDefs[name]
	if !ok {
		return q
	}
	clause := fmt.Sprintf(
		"%s %s ON %s.%s = %s.%s",
		joinType,
		q.qi(cfg.TargetTable),
		q.qi(cfg.TargetTable), q.qi(cfg.TargetColumn),
		q.qi(cfg.SourceTable), q.qi(cfg.SourceColumn),
	)
	q2 := q.clone()
	q2.joins = append(q2.joins, clause)
	return q2
}

// Preload registers a relation to be eagerly loaded after the main query.
func (q *Query[T]) Preload(name string) *Query[T] {
	q2 := q.clone()
	q2.preloads = append(q2.preloads, name)
	return q2
}

// Scopes applies the given scope.Scope values to the query.
func (q *Query[T]) Scopes(scopes ...scope.Scope) *Query[T] {
	q2 := q.clone()
	for _, s := range scopes {
		s.Apply(q2)
	}
	return q2
}

// --- scope.Applier implementation ---

func (q *Query[T]) ApplyWhere(clause string, args []any) {
	q.wheres = append(q.wheres, whereClause{clause, args})
}

func (q *Query[T]) ApplyOrderBy(clause string) {
	q.orderBys = append(q.orderBys, clause)
}

func (q *Query[T]) ApplyLimit(n int)  { q.limit = &n }
func (q *Query[T]) ApplyOffset(n int) { q.offset = &n }

func (q *Query[T]) ApplySelect(columns string) {
	q.selects = &columns
}

var _ scope.Applier = (*Query[any])(nil)

// --- Terminal methods ---

// All executes a SELECT and returns all matching rows.
func (q *Query[T]) All(ctx context.Context) ([]T, error) {
	query, args := q.buildSelect()
	query, args = q.rewrite(query, args)

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err //nolint:wrapcheck // pass through
	}
	defer func() { _ = rows.Close() }()

	var result []T
	for rows.Next() {
		item, err := q.scan(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err //nolint:wrapcheck // pass through
	}

	for _, name := range q.preloads {
		fn, ok := q.preloaders[name]
		if !ok {
			return nil, fmt.Errorf("orm: unknown preload %q", name)
		}
		if err := fn(ctx, q.db, result); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// First executes a SELECT with LIMIT 1 and returns the first row.
// Returns ErrNotFound if no rows match.
func (q *Query[T]) First(ctx context.Context) (T, error) {
	q2 := q.Limit(1)
	items, err := q2.All(ctx)
	if err != nil {
		var zero T
		return zero, err
	}
	if len(items) == 0 {
		var zero T
		return zero, ErrNotFound
	}
	return items[0], nil
}

// Count returns the number of rows matching the current query conditions.
func (q *Query[T]) Count(ctx context.Context) (int64, error) {
	query, args := q.buildCount()
	query, args = q.rewrite(query, args)

	var count int64
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err //nolint:wrapcheck // pass through
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, errors.New("orm: COUNT returned no rows")
	}
	if err := rows.Scan(&count); err != nil {
		return 0, err //nolint:wrapcheck // pass through
	}
	return count, rows.Err() //nolint:wrapcheck // pass through
}

// Exists returns true if at least one row matches the current query conditions.
func (q *Query[T]) Exists(ctx context.Context) (bool, error) {
	count, err := q.Limit(1).Count(ctx)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Create inserts a new row. If setPK is set, the primary key is populated
// via RETURNING (PostgreSQL) or LastInsertId (MySQL).
func (q *Query[T]) Create(ctx context.Context, t *T) error {
	includesPK := q.setPK == nil
	columns, values := q.colValPairs(t, includesPK)

	query := q.buildInsert(columns)
	query, values = q.rewrite(query, values)

	d := q.db.dialect()
	if d.UseReturning() && q.setPK != nil {
		query += d.ReturningClause(q.pk)
		rows, err := q.db.QueryContext(ctx, query, values...)
		if err != nil {
			return err //nolint:wrapcheck // pass through
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			return errors.New("orm: INSERT RETURNING returned no rows")
		}
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err //nolint:wrapcheck // pass through
		}
		q.setPK(t, id)
		return rows.Err() //nolint:wrapcheck // pass through
	}

	result, err := q.db.ExecContext(ctx, query, values...)
	if err != nil {
		return err //nolint:wrapcheck // pass through
	}

	if q.setPK != nil {
		id, err := result.LastInsertId()
		if err != nil {
			return err //nolint:wrapcheck // pass through
		}
		q.setPK(t, id)
	}
	return nil
}

// CreateAll inserts multiple rows in a single INSERT statement.
// If setPK is set, primary keys are populated for each row.
func (q *Query[T]) CreateAll(ctx context.Context, items []*T) error {
	if len(items) == 0 {
		return nil
	}

	includesPK := q.setPK == nil
	columns, _ := q.colValPairs(items[0], includesPK)

	var allValues []any
	for _, item := range items {
		_, vals := q.colValPairs(item, includesPK)
		allValues = append(allValues, vals...)
	}

	query := q.buildBatchInsert(columns, len(items))
	query, allValues = q.rewrite(query, allValues)

	d := q.db.dialect()
	if d.UseReturning() && q.setPK != nil {
		query += d.ReturningClause(q.pk)
		rows, err := q.db.QueryContext(ctx, query, allValues...)
		if err != nil {
			return err //nolint:wrapcheck // pass through
		}
		defer func() { _ = rows.Close() }()
		for i := 0; rows.Next(); i++ {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err //nolint:wrapcheck // pass through
			}
			q.setPK(items[i], id)
		}
		return rows.Err() //nolint:wrapcheck // pass through
	}

	result, err := q.db.ExecContext(ctx, query, allValues...)
	if err != nil {
		return err //nolint:wrapcheck // pass through
	}

	if q.setPK != nil {
		firstID, err := result.LastInsertId()
		if err != nil {
			return err //nolint:wrapcheck // pass through
		}
		for i, item := range items {
			q.setPK(item, firstID+int64(i))
		}
	}
	return nil
}

// Upsert inserts a row or updates it on primary key conflict.
// All non-PK columns are updated on conflict.
// The primary key must be set on t before calling Upsert.
func (q *Query[T]) Upsert(ctx context.Context, t *T) error {
	columns, values := q.colValPairs(t, true) // always include PK

	query := q.buildUpsert(columns)
	query, values = q.rewrite(query, values)

	d := q.db.dialect()
	if d.UseReturning() && q.setPK != nil {
		query += d.ReturningClause(q.pk)
		rows, err := q.db.QueryContext(ctx, query, values...)
		if err != nil {
			return err //nolint:wrapcheck // pass through
		}
		defer func() { _ = rows.Close() }()
		if rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err //nolint:wrapcheck // pass through
			}
			q.setPK(t, id)
		}
		return rows.Err() //nolint:wrapcheck // pass through
	}

	_, err := q.db.ExecContext(ctx, query, values...)
	return err //nolint:wrapcheck // pass through
}

// Update updates the row identified by the primary key of t.
// All non-PK columns are SET.
func (q *Query[T]) Update(ctx context.Context, t *T) error {
	allCols, allVals := q.colValPairs(t, true)

	var setCols []string
	var setVals []any
	var pkVal any
	for i, col := range allCols {
		if col == q.pk {
			pkVal = allVals[i]
		} else {
			setCols = append(setCols, col)
			setVals = append(setVals, allVals[i])
		}
	}
	if pkVal == nil {
		return errors.New("orm: primary key value is required for Update")
	}

	setVals = append(setVals, pkVal)
	query := q.buildUpdate(setCols)
	query, setVals = q.rewrite(query, setVals)

	_, err := q.db.ExecContext(ctx, query, setVals...)
	return err //nolint:wrapcheck // pass through
}

// Delete deletes rows matching the accumulated WHERE clauses.
// Returns an error if no WHERE clauses are set (safety guard).
func (q *Query[T]) Delete(ctx context.Context) error {
	if len(q.wheres) == 0 {
		return errors.New("orm: Delete without WHERE clause is not allowed")
	}
	query, args := q.buildDelete()
	query, args = q.rewrite(query, args)

	_, err := q.db.ExecContext(ctx, query, args...)
	return err //nolint:wrapcheck // pass through
}

// --- SQL building ---

// qi quotes an identifier (table/column name) using the dialect.
func (q *Query[T]) qi(name string) string {
	return q.db.dialect().QuoteIdent(name)
}

// quoteColumns joins column names with dialect-aware quoting.
func (q *Query[T]) quoteColumns(cols []string) string {
	quoted := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = q.qi(c)
	}
	return strings.Join(quoted, ", ")
}

func (q *Query[T]) buildSelect() (string, []any) {
	var b strings.Builder
	b.WriteString("SELECT ")

	if q.selects != nil {
		b.WriteString(*q.selects)
	} else {
		b.WriteString(q.quoteColumns(q.columns))
	}

	b.WriteString(" FROM ")
	b.WriteString(q.qi(q.table))

	for _, j := range q.joins {
		b.WriteByte(' ')
		b.WriteString(j)
	}

	args := q.appendWhere(&b)

	if len(q.orderBys) > 0 {
		b.WriteString(" ORDER BY ")
		b.WriteString(strings.Join(q.orderBys, ", "))
	}

	if q.limit != nil {
		fmt.Fprintf(&b, " LIMIT %d", *q.limit)
	}
	if q.offset != nil {
		fmt.Fprintf(&b, " OFFSET %d", *q.offset)
	}

	return b.String(), args
}

func (q *Query[T]) buildCount() (string, []any) {
	var b strings.Builder
	b.WriteString("SELECT COUNT(*) FROM ")
	b.WriteString(q.qi(q.table))

	for _, j := range q.joins {
		b.WriteByte(' ')
		b.WriteString(j)
	}

	args := q.appendWhere(&b)

	if q.limit != nil {
		fmt.Fprintf(&b, " LIMIT %d", *q.limit)
	}
	if q.offset != nil {
		fmt.Fprintf(&b, " OFFSET %d", *q.offset)
	}

	return b.String(), args
}

func (q *Query[T]) buildInsert(columns []string) string {
	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		q.qi(q.table),
		q.quoteColumns(columns),
		strings.Join(placeholders, ", "),
	)
}

func (q *Query[T]) buildBatchInsert(columns []string, rowCount int) string {
	ph := make([]string, len(columns))
	for i := range ph {
		ph[i] = "?"
	}
	oneRow := "(" + strings.Join(ph, ", ") + ")"

	rows := make([]string, rowCount)
	for i := range rows {
		rows[i] = oneRow
	}

	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES %s",
		q.qi(q.table),
		q.quoteColumns(columns),
		strings.Join(rows, ", "),
	)
}

func (q *Query[T]) buildUpsert(columns []string) string {
	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "INSERT INTO %s (%s) VALUES (%s)",
		q.qi(q.table),
		q.quoteColumns(columns),
		strings.Join(placeholders, ", "),
	)

	var updateCols []string
	for _, col := range columns {
		if col != q.pk {
			updateCols = append(updateCols, col)
		}
	}

	d := q.db.dialect()
	if _, ok := d.(mysqlDialect); ok {
		sets := make([]string, len(updateCols))
		for i, col := range updateCols {
			sets[i] = fmt.Sprintf("%s = VALUES(%s)", q.qi(col), q.qi(col))
		}
		fmt.Fprintf(&b, " ON DUPLICATE KEY UPDATE %s", strings.Join(sets, ", "))
	} else {
		sets := make([]string, len(updateCols))
		for i, col := range updateCols {
			sets[i] = fmt.Sprintf("%s = EXCLUDED.%s", q.qi(col), q.qi(col))
		}
		fmt.Fprintf(&b, " ON CONFLICT (%s) DO UPDATE SET %s", q.qi(q.pk), strings.Join(sets, ", "))
	}

	return b.String()
}

func (q *Query[T]) buildUpdate(setCols []string) string {
	sets := make([]string, len(setCols))
	for i, col := range setCols {
		sets[i] = q.qi(col) + " = ?"
	}
	return fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = ?",
		q.qi(q.table),
		strings.Join(sets, ", "),
		q.qi(q.pk),
	)
}

func (q *Query[T]) buildDelete() (string, []any) {
	var b strings.Builder
	b.WriteString("DELETE FROM ")
	b.WriteString(q.qi(q.table))
	args := q.appendWhere(&b)
	return b.String(), args
}

func (q *Query[T]) appendWhere(b *strings.Builder) []any {
	if len(q.wheres) == 0 {
		return nil
	}

	var args []any
	b.WriteString(" WHERE ")
	for i, w := range q.wheres {
		if i > 0 {
			b.WriteString(" AND ")
		}
		b.WriteString(w.clause)
		args = append(args, w.args...)
	}
	return args
}

// rewrite converts ? placeholders to dialect-specific placeholders.
// For MySQL this is a no-op. For PostgreSQL, ? becomes $1, $2, etc.
func (q *Query[T]) rewrite(query string, args []any) (string, []any) {
	d := q.db.dialect()
	if _, ok := d.(mysqlDialect); ok {
		return query, args
	}

	var b strings.Builder
	b.Grow(len(query))
	idx := 1
	for i := range len(query) {
		if query[i] == '?' {
			b.WriteString(d.Placeholder(idx))
			idx++
		} else {
			b.WriteByte(query[i])
		}
	}
	return b.String(), args
}
