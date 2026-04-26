#!/bin/bash
set -e

# Build release packages with version tag
# Usage: ./build-release.sh [mac|windows|all]
# The version is read from wails.json

PLATFORM="${1:-all}"
BIN_DIR="build/bin"
mkdir -p "$BIN_DIR"

# Read version from wails.json
VERSION=$(grep -o '"productVersion": "[^"]*"' wails.json | cut -d'"' -f4)
if [ -z "$VERSION" ]; then
    echo "Error: Could not read version from wails.json"
    exit 1
fi

# Verify frontend Settings.tsx version matches
SETTINGS_VERSION=$(grep -o "const version = '[^']*'" frontend/src/Settings.tsx | cut -d"'" -f2)
if [ "$VERSION" != "$SETTINGS_VERSION" ]; then
    echo "Error: Version mismatch! wails.json=$VERSION, frontend/src/Settings.tsx=$SETTINGS_VERSION"
    echo "Please sync the version in frontend/src/Settings.tsx before building."
    exit 1
fi

echo "Building release packages for version: $VERSION"

export PATH=$PATH:/usr/local/go/bin

build_mac() {
  echo "Building macOS universal binary..."
  /Users/lobster/go/bin/wails build -platform darwin/universal -clean

  # 复制 ml_models 和 scripts 到构建目录
  echo "Copying ml_models and scripts to build directory..."
  cp -r "$(pwd)/ml_models" "$BIN_DIR/"
  cp -r "$(pwd)/scripts" "$BIN_DIR/"

  cd "$BIN_DIR"
  zip -r "stockfinlens-macos-universal-v${VERSION}.zip" stockfinlens.app ml_models/ scripts/
  cd - > /dev/null
  echo "macOS package: ${BIN_DIR}/stockfinlens-macos-universal-v${VERSION}.zip"
}

build_windows() {
  echo "Building Windows amd64 binary..."
  # Windows 构建需要 CGO 来支持 WebView2，但交叉编译时可能有限制
  # 使用默认设置，让 Wails 自动处理
  /Users/lobster/go/bin/wails build -platform windows/amd64 -clean
  
  # 复制 ml_models 和 scripts 到构建目录（Windows 需要这些文件）
  echo "Copying ml_models and scripts to build directory..."
  cp -r "$(pwd)/ml_models" "$BIN_DIR/"
  cp -r "$(pwd)/scripts" "$BIN_DIR/"
  
  cd "$BIN_DIR"
  zip -r "stockfinlens-windows-amd64-v${VERSION}.zip" stockfinlens.exe ml_models/ scripts/
  cd - > /dev/null
  echo "Windows package: ${BIN_DIR}/stockfinlens-windows-amd64-v${VERSION}.zip"
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
echo ""
echo "Release packages:"
ls -lh ${BIN_DIR}/*.zip 2>/dev/null || true
