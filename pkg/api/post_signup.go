package api

import (
	"apascualco.com/gotway/internal/persistence"
	core_service "apascualco.com/gotway/internal/services"
	domain "apascualco.com/gotway/type"
	"encoding/json"
	"net/http"
)

func UserSignup() domain.RestConfiguration {
	return domain.RestConfiguration{
		PATH:     "/api/v1/signup",
		METHODS:  []string{http.MethodPost},
		FUNCTION: signup,
	}
}

type UserSignupRequest struct {
	Password string `json:"password"`
	User     string `json:"user"`
}

func signup(responseWriter http.ResponseWriter, request *http.Request) {
	signUp := core_service.NewSignUp(persistence.UserRepository{})

	userSignupRequest := &UserSignupRequest{}

	if json.NewDecoder(request.Body).Decode(userSignupRequest) != nil {
		responseWriter.WriteHeader(http.StatusBadRequest)
		return
	}

	created, err := signUp.SignUp(userSignupRequest.User, userSignupRequest.Password)

	if err != nil || !created {
		responseWriter.WriteHeader(http.StatusBadRequest)
		return
	}
	responseWriter.WriteHeader(http.StatusOK)
}
