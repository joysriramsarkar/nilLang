# Nilang Programming Language Makefile
# Supported Platforms: Linux, macOS, Windows

VERSION ?= 0.1.0
BIN_DIR ?= bin
DIST_DIR ?= dist
PREFIX ?= /usr/local

GO ?= go
CGO_ENABLED ?= 0

TARGETS = nil nilc nilpkg nilpkg-server nilkey softbusd

.PHONY: all build test clean install uninstall release

all: build

build:
	@mkdir -p $(BIN_DIR)
	@echo "Building nil..."
	$(GO) build -o $(BIN_DIR)/nil ./cmd/nil
	@echo "Building nilc..."
	$(GO) build -o $(BIN_DIR)/nilc ./cmd/nilc
	@echo "Building nilpkg..."
	$(GO) build -o $(BIN_DIR)/nilpkg ./cmd/nilpkg
	@echo "Building nilpkg-server..."
	$(GO) build -o $(BIN_DIR)/nilpkg-server ./cmd/nilpkg-server
	@echo "Building nilkey..."
	$(GO) build -o $(BIN_DIR)/nilkey ./cmd/nilkey
	@echo "Building softbusd..."
	$(GO) build -o $(BIN_DIR)/softbusd ./cmd/softbusd
	@echo "✅ All binaries built into $(BIN_DIR)/"

test:
	$(GO) test -v ./...

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR) build/

install: build
	@echo "Installing binaries to $(PREFIX)/bin..."
	@mkdir -p $(PREFIX)/bin
	@for target in $(TARGETS); do \
		cp $(BIN_DIR)/$$target $(PREFIX)/bin/$$target; \
		chmod 755 $(PREFIX)/bin/$$target; \
	done
	@echo "✅ Nilang installed successfully to $(PREFIX)/bin"

uninstall:
	@for target in $(TARGETS); do \
		rm -f $(PREFIX)/bin/$$target; \
	done
	@echo "✅ Nilang uninstalled from $(PREFIX)/bin"

release:
	@bash scripts/build_releases.sh
