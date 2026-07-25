#!/usr/bin/env bash
set -e

echo "Compiling..."

# Ensure we're in the script's directory
cd "$(dirname "$0")"

if command -v node >/dev/null 2>&1 && command -v npm >/dev/null 2>&1; then
  echo "Building web UI..."
  (
    cd web
    npm ci
    npm run build
  )
else
  echo "Node.js/npm not found; using committed web/dist."
fi

# Create the output directory if it doesn't exist
mkdir -p bin/linux bin/darwin

# Compile for Linux (x86_64)
GOOS=linux GOARCH=amd64 go build -o bin/linux/agc main.go

# Compile for macOS (ARM)
GOOS=darwin GOARCH=arm64 go build -o bin/darwin/agc main.go

echo "All binaries successfully compiled!"
