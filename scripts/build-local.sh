#!/bin/bash

# Build script for local development
set -e

echo "🔧 Building CGo-free Tokenizers locally..."

# Detect platform
PLATFORM=$(uname -s)
ARCH=$(uname -m)

case $PLATFORM in
    "Darwin")
        LIB_EXT=".dylib"
        TARGET="$(uname -m)-apple-darwin"
        ;;
    "Linux")
        LIB_EXT=".so"
        TARGET="$(uname -m)-unknown-linux-gnu"
        ;;
    "MINGW"*|"CYGWIN"*|"MSYS"*)
        LIB_EXT=".dll"
        TARGET="$(uname -m)-pc-windows-msvc"
        ;;
    *)
        echo "❌ Unsupported platform: $PLATFORM"
        exit 1
        ;;
esac

echo "📋 Platform: $PLATFORM-$ARCH"
echo "🎯 Target: $TARGET"
echo "📚 Library extension: $LIB_EXT"

# Build Rust library
echo "🦀 Building Rust library..."
cargo build --release

# Set library path for Go tests
LIB_PATH="$(pwd)/target/release/libtokenizers$LIB_EXT"
export TOKENIZERS_LIB_PATH="$LIB_PATH"

echo "📍 Library path: $LIB_PATH"

# Verify library exists
if [ ! -f "$LIB_PATH" ]; then
    echo "❌ Library not found at: $LIB_PATH"
    exit 1
fi

echo "✅ Library built successfully!"

# Run tests if requested
if [ "$1" = "test" ]; then
    echo "🧪 Running tests..."
    go test -v ./...
    echo "✅ All tests passed!"
fi

# Show library info
echo "📊 Library info:"
file "$LIB_PATH" || echo "file command not available"
ls -lh "$LIB_PATH"

echo "🎉 Build completed successfully!"
echo "💡 To run tests manually: TOKENIZERS_LIB_PATH='$LIB_PATH' go test -v ./..."