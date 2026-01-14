.PHONY: lint test build

lint:
	golangci-lint run --fix -v

test:
	go test ./...

build:
	GOOS=linux GOARCH=amd64 go build -o bootstrap ./cmd/sync
