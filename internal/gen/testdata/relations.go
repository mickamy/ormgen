package testdata

type Author struct {
	ID   int
	Name string
	// has_many: Author has many Articles
	Articles []Article `rel:"has_many,foreign_key:author_id"`
}

type Article struct {
	ID       int
	AuthorID int
	Title    string
	// belongs_to: Article belongs to Author
	Author *Author `rel:"belongs_to,foreign_key:author_id"`
}
