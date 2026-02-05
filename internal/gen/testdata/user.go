package testdata

import "time"

type User struct {
	ID        int       `db:"id,primaryKey"`
	Name      string    `db:"name"`
	Email     string    `db:"email"`
	Role      string    `db:"role"`
	Active    bool      `db:"active"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	Posts     []Post    `db:"-" rel:"has_many,foreign_key:user_id"`
	internal  string    // unexported, no tag — skipped
}

type Post struct {
	ID     int    `db:"id,primaryKey"`
	UserID int    `db:"user_id"`
	Title  string `db:"title"`
}
