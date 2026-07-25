package repository

import (
	"context"
	"github.com/linjunhao1997/contrib/model"
	_ "gorm.io/gorm"
)

type UserRepository interface {
	UserRepositoryCustom
	QueryById(ctx context.Context, id int) (*model.User, error)
	Create(ctx context.Context, user *model.User) error
}

type UserRepositoryCustom interface {
	MySelf(ctx context.Context)
}
