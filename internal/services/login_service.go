package core_service

import (
	"apascualco.com/gotway/internal/persistence"
	"apascualco.com/gotway/internal/server"
	domain "apascualco.com/gotway/type"
	"golang.org/x/crypto/bcrypt"
)

type Login struct {
	UserRepository persistence.UserRepository
}

func NewLogin(userRepository persistence.UserRepository) *Login {
	login := &Login{}
	login.UserRepository = userRepository
	return login
}

func (login *Login) Login(userId string, password string) (string, error) {

	user, userRepositoryErr := login.UserRepository.GetByEmail(userId)
	if userRepositoryErr != nil {
		return "", userRepositoryErr
	}

	credentialErr := validateCredentials(password, *user)
	if credentialErr != nil {
		return "", credentialErr
	}

	return server.BuildToken(userId, []string{"admin", "luser"})

}

func validateCredentials(password string, user domain.User) error {
	return bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
}
