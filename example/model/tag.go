package model

//go:generate go tool ormgen -source=$GOFILE -destination=../query

type Tag struct {
	ID   int
	Name string
}
