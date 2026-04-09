BINARY     := tf-lens
MODULE     := github.com/hack-monk/tf-lens
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS    := -ldflags="-s -w -X '$(MODULE)/cmd.Version=$(VERSION)'"
GOFLAGS    := CGO_ENABLED=0

# JS bundles embedded into the binary for offline export mode
CYTOSCAPE_VERSION  := 3.28.1
DAGRE_VERSION      := 0.8.5
CYTODAGRE_VERSION  := 2.5.0

BUNDLE_DIR  := internal/renderer/js
CYTO_JS     := $(BUNDLE_DIR)/cytoscape.min.js
DAGRE_JS    := $(BUNDLE_DIR)/dagre.min.js
CYTODAGRE_JS:= $(BUNDLE_DIR)/cytoscape-dagre.min.js

.PHONY: all build bundle test lint clean release help

## help: Show this help message
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'

## all: Bundle JS, then build the binary
all: bundle build

## build: Compile the tf-lens binary
build:
	$(GOFLAGS) go build $(LDFLAGS) -o $(BINARY) .

## build-linux: Cross-compile for Linux amd64 (for CI / Docker)
build-linux:
	GOOS=linux GOARCH=amd64 $(GOFLAGS) go build $(LDFLAGS) -o $(BINARY)-linux-amd64 .

## build-darwin: Cross-compile for macOS arm64 (Apple Silicon)
build-darwin:
	GOOS=darwin GOARCH=arm64 $(GOFLAGS) go build $(LDFLAGS) -o $(BINARY)-darwin-arm64 .

## build-windows: Cross-compile for Windows amd64
build-windows:
	GOOS=windows GOARCH=amd64 $(GOFLAGS) go build $(LDFLAGS) -o $(BINARY)-windows-amd64.exe .

## bundle: Download and embed Cytoscape.js + Dagre JS libraries
bundle: $(BUNDLE_DIR)
	@echo "→ Downloading Cytoscape.js $(CYTOSCAPE_VERSION)..."
	curl -fsSL "https://cdnjs.cloudflare.com/ajax/libs/cytoscape/$(CYTOSCAPE_VERSION)/cytoscape.min.js" -o $(CYTO_JS)
	@echo "→ Downloading Dagre $(DAGRE_VERSION)..."
	curl -fsSL "https://cdnjs.cloudflare.com/ajax/libs/dagre/$(DAGRE_VERSION)/dagre.min.js" -o $(DAGRE_JS)
	@echo "→ Downloading cytoscape-dagre $(CYTODAGRE_VERSION)..."
	curl -fsSL "https://cdn.jsdelivr.net/npm/cytoscape-dagre@$(CYTODAGRE_VERSION)/cytoscape-dagre.min.js" -o $(CYTODAGRE_JS)
	@echo "✅ JS bundles ready in $(BUNDLE_DIR)/"

$(BUNDLE_DIR):
	mkdir -p $(BUNDLE_DIR)

## test: Run all unit tests
test:
	go test ./... -v -count=1

## test-ci: Run tests with race detector and coverage (for CI)
test-ci:
	go test ./... -race -coverprofile=coverage.out -count=1
	go tool cover -func=coverage.out

## lint: Run golangci-lint (requires golangci-lint installed)
lint:
	golangci-lint run ./...

## check-binary-size: Fail if the binary exceeds 25 MB (spec requirement)
check-binary-size: build
	@size=$$(stat -c%s $(BINARY) 2>/dev/null || stat -f%z $(BINARY)); \
	limit=26214400; \
	echo "Binary size: $$size bytes (limit: $$limit)"; \
	if [ "$$size" -gt "$$limit" ]; then \
	  echo "❌ Binary exceeds 25 MB limit!"; exit 1; \
	else \
	  echo "✅ Binary size OK"; \
	fi

## check-icon-count: Fail if fewer than 25 core icons exist
check-icon-count:
	@count=$$(ls internal/icons/svg/*.svg 2>/dev/null | wc -l); \
	echo "Icon count: $$count (minimum: 25)"; \
	if [ "$$count" -lt 25 ]; then \
	  echo "❌ Fewer than 25 icons!"; exit 1; \
	else \
	  echo "✅ Icon count OK"; \
	fi

## clean: Remove build artefacts
clean:
	rm -f $(BINARY) $(BINARY)-* coverage.out

## install: Install to GOPATH/bin
install:
	go install $(LDFLAGS) .
