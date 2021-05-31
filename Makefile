export GO111MODULE=on

init:
	@echo "==> Downloading Go module"
	@go mod download

test:
	@echo "==> Launching tests"
	@go test -v -parallel=20 ./test/...

.PHONY: init test
