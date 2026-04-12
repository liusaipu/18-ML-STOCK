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
  # Windows 构建需要 CGO 来支持 WebView2，但交叉编译时可能有限制
  # 使用默认设置，让 Wails 自动处理
  /Users/lobster/go/bin/wails build -platform windows/amd64 -clean
  
  # 复制 ml_models 到构建目录（Windows 需要这些文件）
  echo "Copying ml_models to build directory..."
  cp -r "$(pwd)/ml_models" "$BIN_DIR/"
  
  cd "$BIN_DIR"
  zip -r "stock-analyzer_windows_${TIMESTAMP}.zip" stock-analyzer.exe ml_models/
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
