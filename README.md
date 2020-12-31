curl --header "Content-Type: application/json" --data '{"user":"apascuaslco@gmail.com","password":"12345"}' -v http://localhost:8080/api/v1/signup
curl --header "Content-Type: application/json" --data '{"user":"apascuaslco@gmail.com","password":"12345"}' -v http://localhost:8080/api/v1/login
curl -H "Content-Type; application/json" -H "Authorization: Bearer " -v http://localhost:8080/api/v1/hello
