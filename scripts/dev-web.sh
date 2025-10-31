#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WEB_DIR="$ROOT_DIR/web"

PORT=${1:-8080}

echo "[dev-web] building wasm bundle..."
GOOS=js GOARCH=wasm go build -o "$WEB_DIR/tinyrl.wasm" "$ROOT_DIR/cmd/tinyrl-wasm"

echo "[dev-web] serving $WEB_DIR on port $PORT"
echo "           press Ctrl+C to stop"

cd "$WEB_DIR"
python3 -m http.server "$PORT"
