.PHONY: test lint vet build tidy check

# Run all tests
test:
	go test ./... -v -timeout 60s

# Run tests excluding integration tests (no sidecar required)
test-unit:
	go test ./... -v -timeout 60s -short

# Run go vet
vet:
	go vet ./...

# Run golangci-lint (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
lint:
	golangci-lint run ./...

# Tidy go.mod
tidy:
	go mod tidy

# Full local check: vet + test
check: vet test

# Build (verify compilation)
build:
	go build ./...
