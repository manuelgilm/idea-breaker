.PHONY: build build-web test lint run-cli run-api web-build desktop-dev desktop-build fmt tidy

build:
	go build ./cmd/aibreak ./cmd/aibreakd

# Build the SPA and copy it where aibreakd embeds it (internal/webui/dist).
web-build:
	npm --prefix cmd/aibreak-desktop/frontend install
	npm --prefix cmd/aibreak-desktop/frontend run build
	cp -r cmd/aibreak-desktop/frontend/dist/. internal/webui/dist/

# Compile aibreakd with the SPA embedded.
build-web: web-build
	go build ./cmd/aibreakd

test:
	go test ./...

lint:
	golangci-lint run

run-cli:
	go run ./cmd/aibreak

run-api:
	go run ./cmd/aibreakd

# Linux builds require the webkit2_41 tag (webkit2gtk-4.1 dev files);
# Wails defaults to webkit2gtk-4.0, which modern distros no longer ship.
desktop-dev:
	cd cmd/aibreak-desktop && wails dev -tags "webkit2_41"

desktop-build:
	cd cmd/aibreak-desktop && wails build -tags "webkit2_41"

fmt:
	gofmt -w ./cmd ./internal
	go mod tidy

tidy:
	go mod tidy
