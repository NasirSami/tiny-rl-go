VERCEL_BUILD_DIR := web
WASM_TARGET := $(VERCEL_BUILD_DIR)/tinyrl.wasm
WASM_EXEC := $(VERCEL_BUILD_DIR)/wasm_exec.js

.PHONY: vercel-build
vercel-build:
	mkdir -p $(VERCEL_BUILD_DIR)
	GOOS=js GOARCH=wasm go build -o $(WASM_TARGET) ./cmd/tinyrl-wasm
	@if ! cp "$(shell go env GOROOT)/misc/wasm/wasm_exec.js" $(WASM_EXEC) 2>/dev/null; then \
		cp "$(shell go env GOROOT)/lib/wasm/wasm_exec.js" $(WASM_EXEC); \
	fi
