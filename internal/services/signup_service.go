package core_service

import (
	"apascualco.com/gotway/internal/persistence"
	domain "apascualco.com/gotway/type"
	"golang.org/x/crypto/bcrypt"
	"net/mail"
)

type SignUp struct {
	UserRepository persistence.UserRepository
}

func NewSignUp(userRepository persistence.UserRepository) *SignUp {
	signUp := &SignUp{}
	signUp.UserRepository = userRepository
	return signUp
}

func (signUp *SignUp) SignUp(userId string, password string) (bool, error) {

	address, mailErr := mail.ParseAddress(userId)
	if mailErr != nil {
		return false, mailErr
	}
	hashedPasswpord, bcryptErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if bcryptErr != nil {
		return false, bcryptErr
	}

	user := &domain.User{}
	user.Password = string(hashedPasswpord)
	user.User = address.Address

	err := signUp.UserRepository.Create(user)
	if err != nil {
		return false, err
	}

	return true, nil
}
