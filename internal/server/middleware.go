package server

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

type Middleware struct {
	Roles map[string][]string
}

func (m *Middleware) AddRole(path string, roles []string) {
	if m.Roles == nil {
		m.Roles = make(map[string][]string)
	}
	m.Roles[path] = roles
}

func (m *Middleware) Middleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {

		endpointRoles := m.Roles[request.URL.Path]
		if len(endpointRoles) > 0 {
			userRoles, err := getUserRoleByRequest(request)
			if err != nil && !anyRoleMatch(endpointRoles, userRoles) {
				responseWriter.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(responseWriter, request)
	})
}

func parseFloat64ToTime(expireTime float64) time.Time {
	sec, dec := math.Modf(expireTime)
	return time.Unix(int64(sec), int64(dec*(1e9)))
}

func getUserRoleByRequest(request *http.Request) ([]interface{}, error) {

	userToken := getTokenFromRequest(request)
	claims, err := ParseToken(userToken)
	if err != nil {
		return nil, err
	}
	expireTokenTime := parseFloat64ToTime(claims["exp"].(float64))
	userRoles := claims["roles"].([]interface{})
	if expireTokenTime.Before(time.Now()) || len(userRoles) == 0 {
		return nil, errors.New("error parsing token")
	}
	return userRoles, nil
}

func getTokenFromRequest(request *http.Request) string {
	authorization := request.Header.Get("Authorization")
	if authorization != "" {
		authorizationParts := strings.Fields(authorization)
		if len(authorizationParts) == 2 {
			tokenType := authorizationParts[0]
			token := authorizationParts[1]
			if strings.EqualFold(tokenType, "bearer") && len(token) > 0 {
				return token
			}
		}
	}
	return ""
}

func anyRoleMatch(endpointRoles []string, userRoles []interface{}) bool {
	for _, endpointRole := range endpointRoles {
		for _, userRole := range userRoles {
			if strings.EqualFold(endpointRole, fmt.Sprintf("%v", userRole)) {
				return true
			}
		}
	}
	return false
}
