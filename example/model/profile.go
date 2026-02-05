package model

//go:generate go tool ormgen -source=$GOFILE -destination=../query

type Profile struct {
	ID     int
	UserID int
	Bio    string
}
