package api

import (
	"apascualco.com/gotway/internal/persistence"
	core_service "apascualco.com/gotway/internal/services"
	domain "apascualco.com/gotway/type"
	"encoding/json"
	"fmt"
	"net/http"
	)

func PostLoginJwt() domain.RestConfiguration {
	return domain.RestConfiguration{
		PATH:     "/api/v1/login",
		METHODS:  []string{http.MethodPost},
		FUNCTION: login,
	}
}

type LoginResponse struct {
	Token string `json: "Token"`
}

type UserLoginRequest struct {
	Password string `json:"password"`
	User     string `json:"user"`
}

func login(responseWriter http.ResponseWriter, request *http.Request) {

	login := core_service.NewLogin(persistence.UserRepository{})

	userLoginRequest := &UserLoginRequest{}

	if json.NewDecoder(request.Body).Decode(userLoginRequest) != nil {
		responseWriter.WriteHeader(http.StatusBadRequest)
		return
	}

	token, err := login.Login(userLoginRequest.User, userLoginRequest.Password)

	if err != nil {
		fmt.Println("JOIN")
		fmt.Println(err)
		responseWriter.WriteHeader(http.StatusBadRequest)
	}
	loginResponse := LoginResponse{
		Token: token,
	}
	json.NewEncoder(responseWriter).Encode(loginResponse)
}
