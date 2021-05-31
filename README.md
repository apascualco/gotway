# ⚙️  Gotway ![](https://travis-ci.com/apascualco/gotway.svg?branch=master)

Gateway in go

## 📦 Versions and dependencies

![](https://img.shields.io/badge/GO-1.16-blue?logo=Go&logoColor=white)

![](https://img.shields.io/badge/dgrijalva%2Fjwt--go-v3.2.0%2Bincompatible-blue?logo=Go&logoColor=white)
![](https://img.shields.io/badge/gorilla%2Fmux-v1.8.0-blue?logo=Go&logoColor=white)
![](https://img.shields.io/badge/mattn%2Fgo--sqlite3-v1.14.4-blue?logo=Go&logoColor=white)
![](https://img.shields.io/badge/x%2Fcrypto-v0.0.0--20201016220609--9e8e0b390897-blue?logo=Go&logoColor=white)
![](https://img.shields.io/badge/gopkg.in%2Fyaml.v2-v2.4.0-blue?logo=Go&logoColor=white)

# How to run
go build cmd/gotway/main.go
./main -p -p=config/server.yml

# 🛠️  How to configure

```yaml
port: 8080
routes:
  - path: "/one"
    uri: http://localhost:8081/helloone
    headers:
      token: token-default
  - path: "/two"
    uri: http://localhost:8081/helloone
```

# How to do a real test 

The first step, to test in real environment, launch a test rest endpoint like:

```Go
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()
	r.PathPrefix("/").HandlerFunc(PrintPathEndpoint)
	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(":8081", r))
}

func PrintPathEndpoint(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Path: %s", r.URL.Path)
}
```

Run the test application and configure the gotway, example:

```yaml
port: 8080
routes:
  - path: "/one"
    uri: http://localhost:8081/helloone
    headers:
      token: token-default
  - path: "/two"
    uri: http://localhost:8081/helloone
```

Build and launch gotway and test it

Result directly test application

![test application](https://www.apascualco.com/wp-content/uploads/2021/05/image.png)

Result with gotway

![gotway](https://www.apascualco.com/wp-content/uploads/2021/05/image-1.png)

## 🔐 Login
curl --header "Content-Type: application/json" --data '{"user":"apascuaslco@gmail.com","password":"12345"}' -v http://localhost:8080/api/v1/signup
curl --header "Content-Type: application/json" --data '{"user":"apascuaslco@gmail.com","password":"12345"}' -v http://localhost:8080/api/v1/login
curl -H "Content-Type; application/json" -H "Authorization: Bearer " -v http://localhost:8080/api/v1/hello
