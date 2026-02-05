package model

import "time"

//go:generate go tool ormgen -source=$GOFILE -destination=../query

type User struct {
	ID        int
	Name      string
	Email     string
	CreatedAt time.Time
	Posts     []Post `db:"-" rel:"has_many,foreign_key:user_id"`
}
