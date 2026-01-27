.PHONY: lint test build build-local build-darwin build-darwin-amd64 build-windows build-linux

lint:
	golangci-lint run --fix -v

test:
	go test ./...

# Build for Lambda deployment (Linux ARM64).
build:
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bootstrap ./cmd/sync

# Build for local machine (auto-detects OS/arch).
build-local:
	go build -o giftbridge ./cmd/sync

# Build for macOS (Apple Silicon).
build-darwin:
	GOOS=darwin GOARCH=arm64 go build -o giftbridge-darwin-arm64 ./cmd/sync

# Build for macOS (Intel).
build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -o giftbridge-darwin-amd64 ./cmd/sync

# Build for Windows.
build-windows:
	GOOS=windows GOARCH=amd64 go build -o giftbridge.exe ./cmd/sync

# Build for Linux (x86_64).
build-linux:
	GOOS=linux GOARCH=amd64 go build -o giftbridge-linux-amd64 ./cmd/sync
