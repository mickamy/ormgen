package testdata

type NoPK struct {
	Name  string `db:"name"`
	Value int    `db:"value"`
}
