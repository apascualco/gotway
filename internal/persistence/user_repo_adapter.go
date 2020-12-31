package persistence

import (
	"apascualco.com/gotway/type"
	"time"
)

type UserRepository struct{}

func (userRepositoryAdapter *UserRepository) Create(user *domain.User) error {
	userEntity := &UserEntity{}
	userEntity.Email = user.User
	userEntity.Password = user.Password
	userEntity.CreatedAt = time.Now()
	userEntity.UpdatedAt = time.Now()

	return userEntity.Create()
}

func (userRepositoryAdapter *UserRepository) GetByEmail(email string) (*domain.User, error) {
	userEntity := &UserEntity{}
	err := userEntity.GetByEmail(email)

	if err != nil {
		return nil, err
	}
	user := &domain.User{}
	user.User = userEntity.Email
	user.Password = userEntity.Password

	return user, nil
}
