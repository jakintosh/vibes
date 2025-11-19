.PHONY: build test run clean test-record

# Build the application
build:
	@echo "Building assistant..."
	@go build -o bin/assistant ./cmd/assistant

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run the application
run:
	@echo "Running assistant..."
	@go run ./cmd/assistant

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/

# Run tests and record new VCR cassettes
# Requires GOOGLE_API_KEY to be set
test-record:
	@echo "Removing VCR cassettes..."
	@rm -rf internal/llm/google/fixtures
	@echo "Running tests to record new cassettes..."
	@go test -v ./...
