# Nilang Programming Language Makefile
# Supported Platforms: Linux, macOS, Windows

VERSION ?= 0.1.0
BIN_DIR ?= bin
DIST_DIR ?= dist
PREFIX ?= /usr/local

GO ?= go
CGO_ENABLED ?= 0

TARGETS = nil nilc nil-bootstrap nil-lsp nil-runner nilpkg nilpkg-server nilkey softbusd

.PHONY: all build bootstrap bootstrap-verify test clean install uninstall release

all: build

build:
	@mkdir -p $(BIN_DIR)
	@echo "Building all Nilang binaries into $(BIN_DIR)/..."
	$(GO) build -o $(BIN_DIR)/ ./cmd/...
	@echo "✅ All binaries built into $(BIN_DIR)/"

bootstrap: build
	$(BIN_DIR)/nil-bootstrap build bootstrap build/nil-compiler.json

bootstrap-verify: build
	$(BIN_DIR)/nil-bootstrap verify bootstrap

test:
	$(GO) test -v ./...

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR) build/

install: build
	@echo "Installing binaries to $(PREFIX)/bin..."
	@mkdir -p $(PREFIX)/bin
	@for target in $(TARGETS); do \
		cp $(BIN_DIR)/$$target $(PREFIX)/bin/$$target; \
		chmod 755 $(PREFIX)/bin/$$target 2>/dev/null || true; \
	done
	@echo "✅ Nilang installed successfully to $(PREFIX)/bin"

uninstall:
	@for target in $(TARGETS); do \
		rm -f $(PREFIX)/bin/$$target; \
	done
	@echo "✅ Nilang uninstalled from $(PREFIX)/bin"

release:
	@bash scripts/build_releases.sh
