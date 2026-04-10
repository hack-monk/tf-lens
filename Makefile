BINARY  := tf-lens
MODULE  := github.com/hack-monk/tf-lens
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags="-s -w -X '$(MODULE)/cmd.Version=$(VERSION)'"
GOFLAGS := CGO_ENABLED=0

# ── JS bundle versions ────────────────────────────────────────────────────────
CYTOSCAPE_VERSION   := 3.28.1
DAGRE_VERSION       := 0.8.5
CYTODAGRE_VERSION   := 2.5.0
HTMLLABEL_VERSION   := 1.2.1

BUNDLE_DIR     := internal/renderer/js
CYTO_JS        := $(BUNDLE_DIR)/cytoscape.min.js
DAGRE_JS       := $(BUNDLE_DIR)/dagre.min.js
CYTODAGRE_JS   := $(BUNDLE_DIR)/cytoscape-dagre.min.js
HTMLLABEL_JS   := $(BUNDLE_DIR)/cytoscape-node-html-label.min.js

.PHONY: all build bundle bundle-check test test-ci lint \
        build-linux build-darwin build-darwin-amd64 build-windows build-all \
        check-binary-size check-icon-count clean clean-bundle install help

## help: Show available targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'

# ── Primary targets ───────────────────────────────────────────────────────────

## all: Download JS bundles then build offline-capable binary (recommended)
all: bundle build

## build: Compile the tf-lens binary (uses embedded JS if bundle was run)
build:
	$(GOFLAGS) go build $(LDFLAGS) -o $(BINARY) .

## bundle: Download and embed all required JS libraries for offline export
bundle:
	@mkdir -p $(BUNDLE_DIR)
	@echo "→ Downloading Cytoscape.js $(CYTOSCAPE_VERSION)..."
	@curl -fsSL "https://cdnjs.cloudflare.com/ajax/libs/cytoscape/$(CYTOSCAPE_VERSION)/cytoscape.min.js" -o $(CYTO_JS)
	@echo "→ Downloading Dagre $(DAGRE_VERSION)..."
	@curl -fsSL "https://cdnjs.cloudflare.com/ajax/libs/dagre/$(DAGRE_VERSION)/dagre.min.js" -o $(DAGRE_JS)
	@echo "→ Downloading cytoscape-dagre $(CYTODAGRE_VERSION)..."
	@curl -fsSL "https://cdn.jsdelivr.net/npm/cytoscape-dagre@$(CYTODAGRE_VERSION)/cytoscape-dagre.min.js" -o $(CYTODAGRE_JS)
	@echo "→ Downloading cytoscape-node-html-label $(HTMLLABEL_VERSION)..."
	@curl -fsSL "https://cdn.jsdelivr.net/npm/cytoscape-node-html-label@$(HTMLLABEL_VERSION)/dist/cytoscape-node-html-label.min.js" -o $(HTMLLABEL_JS)
	@echo ""
	@echo "✅ JS bundles ready ($(BUNDLE_DIR)/):"
	@ls -lh $(BUNDLE_DIR)/*.js | awk '{print "   " $$5 "  " $$9}'
	@echo ""
	@echo "→ Run 'make build' to compile an offline-capable binary."

## bundle-check: Verify all JS bundles are present
bundle-check:
	@missing=0; \
	for f in $(CYTO_JS) $(DAGRE_JS) $(CYTODAGRE_JS) $(HTMLLABEL_JS); do \
	  if [ ! -f "$$f" ]; then echo "❌ Missing: $$f"; missing=1; fi; \
	done; \
	if [ $$missing -eq 0 ]; then echo "✅ All JS bundles present"; fi; \
	exit $$missing

# ── Cross-compilation ─────────────────────────────────────────────────────────

## build-linux: Cross-compile for Linux amd64
build-linux:
	GOOS=linux GOARCH=amd64 $(GOFLAGS) go build $(LDFLAGS) -o $(BINARY)-linux-amd64 .

## build-darwin: Cross-compile for macOS arm64 (Apple Silicon)
build-darwin:
	GOOS=darwin GOARCH=arm64 $(GOFLAGS) go build $(LDFLAGS) -o $(BINARY)-darwin-arm64 .

## build-darwin-amd64: Cross-compile for macOS Intel
build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(GOFLAGS) go build $(LDFLAGS) -o $(BINARY)-darwin-amd64 .

## build-windows: Cross-compile for Windows amd64
build-windows:
	GOOS=windows GOARCH=amd64 $(GOFLAGS) go build $(LDFLAGS) -o $(BINARY)-windows-amd64.exe .

## build-all: Build all platform binaries
build-all: build-linux build-darwin build-darwin-amd64 build-windows

# ── Testing ───────────────────────────────────────────────────────────────────

## test: Run all unit tests
test:
	go test ./... -v -count=1

## test-ci: Run tests with race detector and coverage (for CI)
test-ci:
	go test ./... -race -coverprofile=coverage.out -count=1
	go tool cover -func=coverage.out

# ── Quality checks ────────────────────────────────────────────────────────────

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## check-binary-size: Fail if binary exceeds 25 MB
check-binary-size: build
	@size=$$(stat -c%s $(BINARY) 2>/dev/null || stat -f%z $(BINARY)); \
	limit=26214400; \
	echo "Binary size: $$(echo $$size | awk '{printf "%.1f MB", $$1/1048576}') (limit: 25 MB)"; \
	if [ "$$size" -gt "$$limit" ]; then echo "❌ Binary exceeds 25 MB!"; exit 1; \
	else echo "✅ Binary size OK"; fi

## check-icon-count: Fail if fewer than 25 icons exist
check-icon-count:
	@count=$$(ls internal/icons/svg/*.svg 2>/dev/null | wc -l | tr -d ' '); \
	echo "Icon count: $$count (minimum: 25)"; \
	if [ "$$count" -lt 25 ]; then echo "❌ Fewer than 25 icons!"; exit 1; \
	else echo "✅ Icon count OK"; fi

# ── Housekeeping ──────────────────────────────────────────────────────────────

## clean: Remove compiled binaries and test artifacts
clean:
	rm -f $(BINARY) $(BINARY)-* coverage.out

## clean-bundle: Remove downloaded JS bundles
clean-bundle:
	rm -f $(BUNDLE_DIR)/*.js
	@echo "Bundles removed. Run 'make bundle' to restore."

## install: Install tf-lens to GOPATH/bin
install:
	go install $(LDFLAGS) .