package configuration

import (
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

type ServerProperties struct {
	Host             string            `yaml:"host"`
	Port             int               `yaml:"port"`
	RoutesProperties []RouteProperties `yaml:"routes"`
}

type RouteProperties struct {
	Path    string            `yaml:"path"`
	Uri     string            `yaml:"uri"`
	Headers map[string]string `yaml:"headers,omitempty"`
}

func ReadServerAndRoutesProperties(path string) (ServerProperties, error) {

	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
		return ServerProperties{}, err
	}
	defer file.Close()
	d := yaml.NewDecoder(file)
	var serverProperties ServerProperties
	err = d.Decode(&serverProperties)
	if err != nil {
		return ServerProperties{}, err
	}
	return serverProperties, nil
}
