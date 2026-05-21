.PHONY: test test-verbose test-coverage test-watch help generate-api generate-registration-api generate-liveapi-client

# Regenerate OpenAPI artifacts from spec files.
# Run after any change to api/openapi/*.yaml.
generate-api: generate-registration-api generate-liveapi-client

# Regenerate Politburo registration server stubs.
generate-registration-api:
	cd api/openapi && go tool oapi-codegen -config registration.cfg.yaml registration.yaml

# Regenerate Infinite Flight Live API upstream client and models.
generate-liveapi-client:
	cd api/openapi && go tool oapi-codegen -config liveapi.cfg.yaml liveapi.yaml

# Default target
help:
	@echo "Politburo Test Commands:"
	@echo "  make test           - Run all tests with report"
	@echo "  make test-verbose   - Run tests with verbose output"
	@echo "  make test-coverage  - Run tests and generate coverage report"
	@echo "  make test-watch     - Watch for changes and run tests"
	@echo "  make test-unit      - Run only unit tests (fast)"

# Run tests with custom report
test:
	@./test.sh

# Run tests with verbose output
test-verbose:
	@go test -v ./...

# Run tests with coverage and open HTML report
test-coverage:
	@echo "Generating coverage report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run only unit tests (exclude integration tests if any)
test-unit:
	@go test -short -v ./...

# Watch for changes and run tests (requires entr)
test-watch:
	@if command -v entr > /dev/null; then \
		find . -name "*.go" | entr -c make test; \
	else \
		echo "entr not installed. Install with: apt-get install entr (Ubuntu) or brew install entr (Mac)"; \
	fi

# Run specific package tests
test-api:
	@go test -v ./internal/api/...

test-services:
	@go test -v ./internal/services/...

test-providers:
	@go test -v ./internal/providers/...
