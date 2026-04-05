.PHONY: test test-coverage lint fmt build dev

test:
	go test -v .

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

lint:
	go vet ./...
	golangci-lint run ./...

fmt:
	go fmt ./...

build:
	go build ./examples/rest-api/...

dev:
	cd examples/rest-api && air
