.PHONY: test

# Run all tests
test:
	go test -v ./...

# Run tests with coverage report
test-cover:
	go test -cover ./...

.PHONY: generate
generate:
	@echo ">> generating v3 service options"
	@go run cmd/generate-options/main.go
	@echo ">> generating cached repositories"
	@go run cmd/generate-cached/main.go
	@echo ">> generating gob import"
	@go run cmd/generate-gob/main.go

.PHONY: update-deps
update-deps:
	@echo ">> updating Go dependencies"
	@for m in $$(go list -mod=readonly -m -f '{{ if and (not .Indirect) (not .Main)}}{{.Path}}{{end}}' all); do \
		go get $$m; \
	done
	go mod tidy
ifneq (,$(wildcard vendor))
	go mod vendor
endif

