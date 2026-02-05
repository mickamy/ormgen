package model

//go:generate go tool ormgen -source=$GOFILE -destination=../query

type Post struct {
	ID     int
	UserID int
	Title  string
	Body   string
	User   *User `db:"-" rel:"belongs_to,foreign_key:user_id"`
}
