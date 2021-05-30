package api

import (
	"fmt"
	"io"

	"net/http"

	"apascualco.com/gotway/internal/configuration"
	domain "apascualco.com/gotway/type"
)

type RouteTunnel struct {
	RouteProperties configuration.RouteProperties
}

func Tunnel(r configuration.RouteProperties, p string, m []string) domain.RestConfiguration {
	routeTunnel := RouteTunnel{
		RouteProperties: r,
	}
	return domain.RestConfiguration{
		PATH:     p,
		METHODS:  m,
		FUNCTION: routeTunnel.tunnel,
	}
}

func (r RouteTunnel) tunnel(responseWriter http.ResponseWriter, request *http.Request) {
	response, err := http.Get(r.RouteProperties.Uri)
	if err != nil {
		fmt.Print(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Print(err)
	}
	fmt.Fprintf(responseWriter, string(body))
}
