.PHONY: build test run clean

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
