# UniFi Gate Opener Makefile

APP_NAME := unifi-gate-opener
VERSION := 1.0.0
BUILD_DIR := build
MAIN_FILE := cmd/main.go

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod

# Build flags
LDFLAGS := -ldflags="-s -w -X main.Version=$(VERSION)"

# Platforms
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build clean test deps run

# Default target
all: build

# Install dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Build for current platform
build: deps
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(APP_NAME) $(MAIN_FILE)

# Build for all platforms
build-all: deps
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} \
		$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-$${platform%/*}-$${platform#*/} $(MAIN_FILE); \
	done

# Build for Linux (most common for servers)
build-linux: deps
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 $(MAIN_FILE)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-arm64 $(MAIN_FILE)

# Build for macOS
build-darwin: deps
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 $(MAIN_FILE)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin-arm64 $(MAIN_FILE)

# Build for Windows
build-windows: deps
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe $(MAIN_FILE)

# Run the application
run: build
	./$(APP_NAME)

# Run tests
test:
	$(GOTEST) -v ./...

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

# Create release archives
release: build-all
	@mkdir -p $(BUILD_DIR)/releases
	@for platform in $(PLATFORMS); do \
		if [ "$${platform#*/}" = "windows/amd64" ]; then \
			zip -j $(BUILD_DIR)/releases/$(APP_NAME)-$(VERSION)-$${platform%/*}-$${platform#*/}.zip \
				$(BUILD_DIR)/$(APP_NAME)-$${platform%/*}-$${platform#*/}.exe README.md; \
		else \
			tar -czf $(BUILD_DIR)/releases/$(APP_NAME)-$(VERSION)-$${platform%/*}-$${platform#*/}.tar.gz \
				-C $(BUILD_DIR) $(APP_NAME)-$${platform%/*}-$${platform#*/} -C .. README.md; \
		fi \
	done

# Install locally
install: build
	@echo "Installing $(APP_NAME) to /usr/local/bin..."
	@sudo cp $(BUILD_DIR)/$(APP_NAME) /usr/local/bin/

# Uninstall
uninstall:
	@echo "Removing $(APP_NAME) from /usr/local/bin..."
	@sudo rm -f /usr/local/bin/$(APP_NAME)

# Help
help:
	@echo "UniFi Gate Opener - Makefile targets:"
	@echo "  make build       - Build for current platform"
	@echo "  make build-all   - Build for all platforms"
	@echo "  make build-linux - Build for Linux (amd64, arm64)"
	@echo "  make build-darwin- Build for macOS (amd64, arm64)"
	@echo "  make build-windows - Build for Windows (amd64)"
	@echo "  make run         - Build and run the application"
	@echo "  make test        - Run tests"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make release     - Create release archives for all platforms"
	@echo "  make install     - Install to /usr/local/bin (requires sudo)"
	@echo "  make deps        - Download and tidy dependencies"