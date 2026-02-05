package repo

import (
	"context"

	"github.com/mickamy/ormgen/example/model"
	"github.com/mickamy/ormgen/example/query"
	"github.com/mickamy/ormgen/orm"
	"github.com/mickamy/ormgen/scope"
)

// UserRepository wraps generated query functions with a repository pattern.
type UserRepository struct {
	db orm.Querier
}

func NewUserRepository(db orm.Querier) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u *model.User) error {
	return query.Users(r.db).Create(ctx, u)
}

func (r *UserRepository) FindByID(ctx context.Context, id int) (model.User, error) {
	return query.Users(r.db).Where("id = ?", id).First(ctx)
}

func (r *UserRepository) FindAll(ctx context.Context, scopes ...scope.Scope) ([]model.User, error) {
	return query.Users(r.db).Scopes(scopes...).OrderBy("id").All(ctx)
}

func (r *UserRepository) Update(ctx context.Context, u *model.User) error {
	return query.Users(r.db).Update(ctx, u)
}

func (r *UserRepository) Delete(ctx context.Context, id int) error {
	return query.Users(r.db).Where("id = ?", id).Delete(ctx)
}
