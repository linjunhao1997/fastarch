package repository

import (
	"context"
	"github.com/linjunhao1997/contrib/model"
	"gorm.io/gorm"
)

type UserRepositoryImpl struct {
	DB *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepositoryImpl {
	return &UserRepositoryImpl{DB: db}
}

func (r *UserRepositoryImpl) QueryById(ctx context.Context, id int) (*model.User, error) {
	var data model.User
	if err := r.DB.WithContext(ctx).Where(model.UserColumnId, id).First(&data).Error; err != nil {
		return nil, err
	}
	return &data, nil
}

func (r *UserRepositoryImpl) Create(ctx context.Context, user *model.User) error {
	if err := r.DB.WithContext(ctx).Create(user).Error; err != nil {
		return err
	}
	return nil
}
