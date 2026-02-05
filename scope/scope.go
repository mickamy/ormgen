package scope

import "strings"

// Applier is implemented by query builders to receive scope fragments.
// This interface lives in the scope package so that orm can import scope
// without creating circular dependencies.
type Applier interface {
	ApplyWhere(clause string, args []any)
	ApplyOrderBy(clause string)
	ApplyLimit(n int)
	ApplyOffset(n int)
	ApplySelect(columns string)
}

type scopeKind int

const (
	kindWhere scopeKind = iota
	kindOrderBy
	kindLimit
	kindOffset
	kindSelect
)

// Scope represents a single query condition fragment.
// Scopes are immutable and safe to reuse across queries.
type Scope struct {
	kind   scopeKind
	clause string
	args   []any
	n      int
}

// Apply dispatches this Scope to the given Applier.
func (s Scope) Apply(a Applier) {
	switch s.kind {
	case kindWhere:
		a.ApplyWhere(s.clause, s.args)
	case kindOrderBy:
		a.ApplyOrderBy(s.clause)
	case kindLimit:
		a.ApplyLimit(s.n)
	case kindOffset:
		a.ApplyOffset(s.n)
	case kindSelect:
		a.ApplySelect(s.clause)
	}
}

// Where returns a Scope that adds a WHERE clause fragment.
//
//	scope.Where("age > ?", 18)
//	scope.Where("name = ? AND role = ?", "alice", "admin")
func Where(clause string, args ...any) Scope {
	return Scope{kind: kindWhere, clause: clause, args: args}
}

// OrderBy returns a Scope that sets the ORDER BY clause.
//
//	scope.OrderBy("created_at DESC")
func OrderBy(clause string) Scope {
	return Scope{kind: kindOrderBy, clause: clause}
}

// Limit returns a Scope that sets the LIMIT.
func Limit(n int) Scope {
	return Scope{kind: kindLimit, n: n}
}

// Offset returns a Scope that sets the OFFSET.
func Offset(n int) Scope {
	return Scope{kind: kindOffset, n: n}
}

// Select returns a Scope that overrides the SELECT column list.
//
//	scope.Select("id", "name")
func Select(columns ...string) Scope {
	return Scope{kind: kindSelect, clause: strings.Join(columns, ", ")}
}

// In returns a WHERE scope with an IN clause, expanding the slice into
// individual placeholders. No reflection is used; generics handle the
// type conversion.
//
//	scope.In("id", []int{1, 2, 3})  // → WHERE id IN (?, ?, ?)
func In[T any](column string, values []T) Scope {
	if len(values) == 0 {
		return Where("1 = 0")
	}
	placeholders := repeatJoin("?", len(values))
	args := make([]any, len(values))
	for i, v := range values {
		args[i] = v
	}
	return Where(column+" IN ("+placeholders+")", args...)
}

// Scopes is a named slice of Scope, useful for conditionally building
// up a set of scopes.
//
//	var s scope.Scopes
//	if onlyActive {
//	    s = s.Append(Active)
//	}
//	s = s.Append(Paginate(page, perPage))
//	Users(db).Scopes(s...).All(ctx)
type Scopes []Scope

// Append adds scopes and returns a new Scopes. The receiver is not modified.
func (ss Scopes) Append(scopes ...Scope) Scopes {
	return append(append(Scopes(nil), ss...), scopes...)
}

// Merge concatenates two Scopes and returns a new Scopes.
// Neither receiver nor argument is modified.
func (ss Scopes) Merge(other Scopes) Scopes {
	return append(append(Scopes(nil), ss...), other...)
}

// Combine creates a Scopes from the given scopes.
//
//	scope.Combine(scope.Limit(10), scope.Offset(20))
func Combine(scopes ...Scope) Scopes {
	return Scopes(scopes)
}

func repeatJoin(s string, count int) string {
	if count <= 0 {
		return ""
	}
	parts := make([]string, count)
	for i := range parts {
		parts[i] = s
	}
	return strings.Join(parts, ", ")
}
