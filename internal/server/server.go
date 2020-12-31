package server

import (
	"github.com/gorilla/mux"
	"net/http"
)

type Server struct {
	Router *mux.Router
	Middleware Middleware
}

func New(m Middleware) *Server {
	return &Server{
		Router: mux.NewRouter(),
		Middleware: m,
	}
}

func (s *Server) AddRoute(
	path string,
	handlerFunc func(responseWriter http.ResponseWriter, request *http.Request),
	method []string) {
	s.Router.HandleFunc(path, handlerFunc).Methods(method...)
}

func (s *Server) AddRole(path string, roles []string) {
	s.Middleware.AddRole(path, roles)
}

func (s *Server) CreateHandler() http.Handler {
	s.Router.Use(s.Middleware.Middleware)
	return s.Router
}