.PHONY: build test run clean

# Build the application
build:
	@echo "Building scheduler..."
	@mkdir -p bin
	@go build -o bin/scheduler ./cmd/scheduler

# Run tests
test: build
	@echo "Running tests..."
	@go test -v ./...

# Run the application
run: build
	@echo "Running scheduler..."
	@./bin/scheduler

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin
