.PHONY: test test-race test-cover bench vet lint clean help

## help: Show this help message
help:
	@echo "LocalBase — Development Commands"
	@echo "================================="
	@echo ""
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':'

## test: Run all tests
test:
	go test -v ./...

## test-race: Run tests with race detector
test-race:
	go test -v -race ./...

## test-cover: Run tests and generate coverage report
test-cover:
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out
	@echo ""
	@echo "To view HTML coverage report: go tool cover -html=coverage.out"

## bench: Run benchmarks
bench:
	go test -bench=. -benchmem -count=3 -run=^$$ ./...

## vet: Run go vet
vet:
	go vet ./...

## lint: Run golangci-lint (install: https://golangci-lint.run/welcome/install/)
lint:
	golangci-lint run ./...

## example: Run the example application
example:
	go run ./cmd/example

## clean: Remove build artifacts and test data
clean:
	rm -f coverage.out
	rm -rf example_data/
	go clean -testcache

## all: Run vet, tests with race detection, and benchmarks
all: vet test-race bench
