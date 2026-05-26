BINARY_NAME=dbsync
MAIN_FILE=./cmd/dbsync
PACKAGE_PATH=./...

.PHONY: all build run test test-integration clean fmt vet help

all: build

build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) $(MAIN_FILE)

run:
	@echo "Running $(BINARY_NAME)..."
	go run $(MAIN_FILE)

test:
	@echo "Running unit tests..."
	go test $(PACKAGE_PATH)

test-integration:
	@echo "Running unit and integration tests..."
	go test -tags=integration $(PACKAGE_PATH)

clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)

fmt:
	@echo "Formatting code..."
	go fmt $(PACKAGE_PATH)

vet:
	@echo "Running go vet..."
	go vet $(PACKAGE_PATH)

help:
	@echo "Available targets:"
	@echo "  all              - Build the binary (default)"
	@echo "  build            - Build the binary"
	@echo "  run              - Run the application"
	@echo "  test             - Run unit tests"
	@echo "  test-integration - Run unit and integration tests"
	@echo "  clean            - Remove the binary"
	@echo "  fmt              - Format the code"
	@echo "  vet              - Run go vet"
	@echo "  help             - Show this help message"
