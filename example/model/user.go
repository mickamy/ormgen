package model

import "time"

//go:generate go tool ormgen -source=$GOFILE -destination=../query

type User struct {
	ID        int       `db:"id,primaryKey"`
	Name      string    `db:"name"`
	Email     string    `db:"email"`
	CreatedAt time.Time `db:"created_at"`
}
