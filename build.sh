#!/usr/bin/env bash
set -e

echo "Compiling..."

# Ensure we're in the script's directory
cd "$(dirname "$0")"

# Create the output directory if it doesn't exist
mkdir -p bin

# Compile for Linux (x86_64)
GOOS=linux GOARCH=amd64 go build -o bin/linux/agc main.go

# Compile for macOS (ARM)
GOOS=darwin GOARCH=arm64 go build -o bin/darwin/agc main.go

echo "All binaries successfully compiled!"
