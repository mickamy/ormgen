package testdata

import "time"

type WithTimestamps struct {
	ID        int       `db:"id,primaryKey"`
	Name      string    `db:"name"`
	CreatedAt time.Time // convention
	UpdatedAt time.Time // convention
}

type WithCustomTimestampCols struct {
	ID         int       `db:"id,primaryKey"`
	InsertedAt time.Time `db:"inserted_at,createdAt"`
	ModifiedAt time.Time `db:"modified_at,updatedAt"`
}

type WithTagAndConvention struct {
	ID        int       `db:"id,primaryKey"`
	CreatedAt time.Time `db:"created_at"` // convention still applies with tag
	UpdatedAt time.Time `db:"updated_at"` // convention still applies with tag
}
