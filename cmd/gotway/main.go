package main

import (
	"apascualco.com/gotway/internal/persistence"
	"apascualco.com/gotway/internal/server"
	"apascualco.com/gotway/pkg/api"
	"log"
	"net/http"
	"sync"
)

func main() {
	persistence.LaunchDDL()
	middleware := server.Middleware{}
	s := server.New(middleware)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		log.Fatal(http.ListenAndServe(":8080", s.CreateHandler()))
	}()
	loadLogin(s)
	loadSignup(s)
	wg.Wait()
}

func loadSignup(s *server.Server) {
	restConfiguration := api.UserSignup()
	s.AddRoute(restConfiguration.PATH, restConfiguration.FUNCTION, restConfiguration.METHODS)
	s.AddRole(restConfiguration.PATH, restConfiguration.ROLES)
}

func loadLogin(s *server.Server) {
	restConfiguration := api.PostLoginJwt()
	s.AddRoute(restConfiguration.PATH, restConfiguration.FUNCTION, restConfiguration.METHODS)
	s.AddRole(restConfiguration.PATH, restConfiguration.ROLES)
}
