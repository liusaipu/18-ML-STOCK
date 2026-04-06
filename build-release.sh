#!/bin/bash
set -e

# Build release packages with timestamps
# Usage: ./build-release.sh [mac|windows|all]

PLATFORM="${1:-all}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BIN_DIR="build/bin"
mkdir -p "$BIN_DIR"

export PATH=$PATH:/usr/local/go/bin

build_mac() {
  echo "Building macOS universal binary..."
  /Users/lobster/go/bin/wails build -platform darwin/universal -clean
  cd "$BIN_DIR"
  zip -r "stock-analyzer_${TIMESTAMP}.zip" stock-analyzer.app
  cd - > /dev/null
  echo "macOS package: ${BIN_DIR}/stock-analyzer_${TIMESTAMP}.zip"
}

build_windows() {
  echo "Building Windows amd64 binary..."
  CGO_ENABLED=0 /Users/lobster/go/bin/wails build -platform windows/amd64
  cd "$BIN_DIR"
  zip -j "stock-analyzer_windows_${TIMESTAMP}.zip" stock-analyzer.exe
  cd - > /dev/null
  echo "Windows package: ${BIN_DIR}/stock-analyzer_windows_${TIMESTAMP}.zip"
}

case "$PLATFORM" in
  mac)
    build_mac
    ;;
  windows)
    build_windows
    ;;
  all)
    build_mac
    build_windows
    ;;
  *)
    echo "Usage: $0 [mac|windows|all]"
    exit 1
    ;;
esac

echo "Done."
