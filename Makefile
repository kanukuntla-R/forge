# Variables
BINARY   := forge
DIST_DIR := dist
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "unknown")
LDFLAGS  := -ldflags "-X github.com/kanukuntla-r/forge.Version=$(VERSION)"
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: build test release install clean

# Default target: local build for the current platform
build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/forge

# Run the full test suite
test:
	go test ./...

# Cross-compile binaries for all supported platforms into dist/
release:
	@rm -rf $(DIST_DIR)
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d/ -f1); \
		arch=$$(echo $$platform | cut -d/ -f2); \
		output="$(DIST_DIR)/$(BINARY)-$$os-$$arch"; \
		echo "Building $$output..."; \
		GOOS=$$os GOARCH=$$arch go build $(LDFLAGS) -o $$output ./cmd/forge; \
	done
	@echo ""
	@echo "Release binaries built in $(DIST_DIR)/:"
	@ls -la $(DIST_DIR)/

# Build and install to ~/.local/bin/
install: build
	@mkdir -p ~/.local/bin
	@cp $(BINARY) ~/.local/bin/$(BINARY)
	@echo "Installed $(BINARY) $(VERSION) to ~/.local/bin/$(BINARY)"

# Remove build artifacts
clean:
	@rm -f $(BINARY)
	@rm -rf $(DIST_DIR)
