package domain

import "net/http"

type RestConfiguration struct {
	PATH     string
	METHODS  []string
	ROLES    []string
	FUNCTION func(responseWriter http.ResponseWriter, request *http.Request)
}
