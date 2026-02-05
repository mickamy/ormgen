package testdata

import "time"

type Inferred struct {
	ID        int       // pk inferred from field name "ID"
	Name      string    // no db tag — column inferred as "name"
	CreatedAt time.Time // no db tag — column inferred as "created_at"
	Secret    string    `db:"-"` // explicitly skipped
	internal  string    // unexported — skipped
}
