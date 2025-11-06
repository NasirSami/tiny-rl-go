VERCEL_BUILD_DIR := web
WASM_TARGET := $(VERCEL_BUILD_DIR)/tinyrl.wasm
WASM_EXEC := $(VERCEL_BUILD_DIR)/wasm_exec.js
GO_BIN := $(shell command -v go 2>/dev/null)
GO_VERSION := 1.22.5
GO_TMP := /tmp/go-$(GO_VERSION)
GO_DIST := https://go.dev/dl/go$(GO_VERSION).linux-amd64.tar.gz

.PHONY: ensure-go
ensure-go:
	@if [ -z "$(GO_BIN)" ]; then \
		echo "Downloading Go $(GO_VERSION)..."; \
		rm -rf $(GO_TMP); \
		mkdir -p $(GO_TMP); \
		curl -sSL $(GO_DIST) | tar -C $(GO_TMP) -xz --strip-components=1; \
		$(GO_TMP)/bin/go version; \
	fi

.PHONY: vercel-build
vercel-build: ensure-go
	@mkdir -p $(VERCEL_BUILD_DIR)
	@if [ -z "$(GO_BIN)" ]; then \
		export GOROOT=$(GO_TMP); \
		export PATH=$$GOROOT/bin:$$PATH; \
		GOOS=js GOARCH=wasm $$GOROOT/bin/go build -o $(WASM_TARGET) ./cmd/tinyrl-wasm; \
		GO_WASM_EXEC=$$($$GOROOT/bin/go env GOROOT); \
		cp "$$GO_WASM_EXEC/misc/wasm/wasm_exec.js" $(WASM_EXEC) 2>/dev/null || cp "$$GO_WASM_EXEC/lib/wasm/wasm_exec.js" $(WASM_EXEC); \
	else \
		GOOS=js GOARCH=wasm go build -o $(WASM_TARGET) ./cmd/tinyrl-wasm; \
		GO_WASM_EXEC=$$(go env GOROOT); \
		cp "$$GO_WASM_EXEC/misc/wasm/wasm_exec.js" $(WASM_EXEC) 2>/dev/null || cp "$$GO_WASM_EXEC/lib/wasm/wasm_exec.js" $(WASM_EXEC); \
	fi
