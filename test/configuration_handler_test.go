package test

import (
	"path/filepath"
	"runtime"
	"testing"

	"apascualco.com/gotway/internal/configuration"
)

func TestServerPropertiesYml(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	path := filepath.Dir(filename)
	propertiesFilename := path + "/resources/server.yaml"
	config, err := configuration.ReadServerAndRoutesProperties(propertiesFilename)

	if err != nil {
		t.Errorf("Error reading server properties")
	}

	if config.Host != "localhost" {
		t.Errorf("Host should be: localhost")
	}

	if config.Port != 8080 {
		t.Errorf("Port should be: 8080")
	}

	if len(config.RoutesProperties) != 2 {
		t.Errorf("RoutesProperties len should be: 2")
	}

	routePathUno := config.RoutesProperties[0]

	if routePathUno.Path != "/one" {
		t.Errorf("RouterProperties path should be one")
	}

	if routePathUno.Uri != "localhost" {
		t.Errorf("RouteProperties uri should be: localhost")
	}

	if len(routePathUno.Headers) != 1 {
		t.Errorf("RouteProperties headers should have headers")
	}

	routePathTwo := config.RoutesProperties[1]

	if routePathTwo.Path != "/two" {
		t.Errorf("The second route id should be empty")
	}

	if routePathTwo.Uri != "127.0.0.1" {
		t.Errorf("RouteProperties uri should be: 127.0.0.1")
	}

	if len(routePathTwo.Headers) != 0 {
		t.Errorf("RouteProperties headers shouldn't have headers")
	}
}
