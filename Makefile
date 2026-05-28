BINARY_NAME=dbsync
MAIN_FILE=./cmd/dbsync
PACKAGE_PATH=./...

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
GOFLAGS := -trimpath

.PHONY: all build run test test-integration clean fmt vet help \
        build-linux build-windows build-all snapshot release-check

all: build

build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) $(MAIN_FILE)

run:
	@echo "Running $(BINARY_NAME)..."
	go run $(MAIN_FILE)

test:
	@echo "Running unit tests..."
	go test $(PACKAGE_PATH)

test-integration:
	@echo "Running unit and integration tests..."
	go test -tags=integration $(PACKAGE_PATH)

build-linux:
	@echo "Building $(BINARY_NAME) for linux/amd64 ($(VERSION))..."
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
		go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o dist/$(BINARY_NAME)_linux_amd64 $(MAIN_FILE)

build-windows:
	@echo "Building $(BINARY_NAME) for windows/amd64 ($(VERSION))..."
	@mkdir -p dist
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
		go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o dist/$(BINARY_NAME)_windows_amd64.exe $(MAIN_FILE)

build-all: build-linux build-windows
	@echo "Cross-platform builds done. See ./dist/"

snapshot:
	@echo "Running GoReleaser snapshot..."
	goreleaser release --snapshot --clean

release-check:
	@goreleaser check

clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)
	rm -rf dist/

fmt:
	@echo "Formatting code..."
	go fmt $(PACKAGE_PATH)

vet:
	@echo "Running go vet..."
	go vet $(PACKAGE_PATH)

help:
	@echo "Available targets:"
	@echo "  all              - Build the binary (default)"
	@echo "  build            - Build the binary with version injection"
	@echo "  run              - Run the application"
	@echo "  test             - Run unit tests"
	@echo "  test-integration - Run unit and integration tests"
	@echo "  build-linux      - Cross-compile for Linux amd64"
	@echo "  build-windows    - Cross-compile for Windows amd64"
	@echo "  build-all        - Build for both Linux and Windows"
	@echo "  snapshot         - Run GoReleaser snapshot"
	@echo "  release-check    - Run GoReleaser check"
	@echo "  clean            - Remove binaries and dist/ directory"
	@echo "  fmt              - Format the code"
	@echo "  vet              - Run go vet"
	@echo "  help             - Show this help message"
