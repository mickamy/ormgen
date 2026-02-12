package testdata

type StringArray []string

type Repository struct {
	ID     int         `db:"id,primaryKey"`
	Name   string      `db:"name"`
	Topics StringArray `db:"topics"`
}

// NoTagCustomType tests that bare exported idents without db tag
// are still recognized as columns (convention-based).
type NoTagCustomType struct {
	ID     int `db:"id,primaryKey"`
	Name   string
	Tags   StringArray
	Owner  *User    `rel:"belongs_to,foreign_key:owner_id"`
}
