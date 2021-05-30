package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"

	"apascualco.com/gotway/internal/configuration"
	"apascualco.com/gotway/internal/persistence"
	"apascualco.com/gotway/internal/server"
	"apascualco.com/gotway/pkg/api"
)

func getProperties(p *string) configuration.ServerProperties {
	serverProperties, err := configuration.ReadServerAndRoutesProperties(*p)
	if err != nil {
		log.Fatal(err)
	}
	return serverProperties
}

func main() {
	path := flag.String("p", "", "properties file path yml")
	flag.Parse()
	serverProperties := getProperties(path)

	persistence.LaunchDDL()
	middleware := server.Middleware{}
	s := server.New(middleware)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		addr := fmt.Sprintf("%s:%d", serverProperties.Host, serverProperties.Port)
		log.Printf("Server addr: %s", addr)
		log.Fatal(http.ListenAndServe(addr, s.CreateHandler()))
	}()
	loadLogin(s)
	loadSignup(s)
	loadRoutes(s, serverProperties.RoutesProperties)
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

func loadRoutes(s *server.Server, r []configuration.RouteProperties) {
	for _, e := range r {
		restConfiguration := api.Tunnel(e, e.Path, []string{http.MethodGet})
		s.AddRoute(restConfiguration.PATH, restConfiguration.FUNCTION, restConfiguration.METHODS)
		s.AddRole(restConfiguration.PATH, restConfiguration.ROLES)
	}
}
