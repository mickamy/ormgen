package testdata

type Author struct {
	ID   int
	Name string
	// has_many: Author has many Articles
	Articles []Article `rel:"has_many,foreign_key:author_id"`
	// has_one: Author has one Profile
	Profile *Profile `rel:"has_one,foreign_key:author_id"`
}

type Profile struct {
	ID       int
	AuthorID int
	Bio      string
}

type Article struct {
	ID       int
	AuthorID int
	Title    string
	// belongs_to: Article belongs to Author
	Author *Author `rel:"belongs_to,foreign_key:author_id"`
}
