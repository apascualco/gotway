package core_service

import (
	"apascualco.com/gotway/internal/server"
	"github.com/dgrijalva/jwt-go"
)

func ParseToken(userToken string) (jwt.MapClaims, error) {
	return server.ParseToken(userToken)
}
