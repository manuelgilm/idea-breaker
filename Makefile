.PHONY: build test lint run-cli run-api fmt tidy

build:
	go build ./cmd/aibreak ./cmd/aibreakd

test:
	go test ./...

lint:
	golangci-lint run

run-cli:
	go run ./cmd/aibreak

run-api:
	go run ./cmd/aibreakd

fmt:
	gofmt -w ./cmd ./internal
	go mod tidy

tidy:
	go mod tidy
