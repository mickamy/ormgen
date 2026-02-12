package testdata

import amodel "github.com/example/auth/model"

type EndUser struct {
	ID            int                   `db:"id,primaryKey"`
	Name          string                `db:"name"`
	OAuthAccounts []amodel.OAuthAccount `rel:"has_many,foreign_key:end_user_id"`
	Email         UserEmail             `rel:"has_one,foreign_key:end_user_id"`
}

type UserEmail struct {
	ID        int    `db:"id,primaryKey"`
	EndUserID int    `db:"end_user_id"`
	Address   string `db:"address"`
}
