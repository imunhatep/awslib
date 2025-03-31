.PHONY: test

# Run all tests
test:
	go test -v ./...

# Run tests with coverage report
test-cover:
	go test -cover ./...