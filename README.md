# pure-tokenizers

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.24-blue.svg)](https://go.dev/)
[![CI Status](https://github.com/amikos-tech/pure-tokenizers/workflows/CI/badge.svg)](https://github.com/amikos-tech/pure-tokenizers/actions)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/amikos-tech/pure-tokenizers)](https://github.com/amikos-tech/pure-tokenizers/releases)

CGo-free tokenizers for Go with automatic library management.

- ✅ **No CGo required** - Pure Go implementation using purego FFI
- ✅ **Automatic downloads** - Platform-specific libraries fetched on demand
- ✅ **Cross-platform** - Windows, macOS, Linux (including ARM)
- ✅ **Production ready** - Checksum verification and ABI compatibility checks

## Quick Start

First, get a tokenizer configuration file:

```bash
# Download a tokenizer from Hugging Face
curl -o tokenizer.json https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/tokenizer.json

# Or use the example tokenizer.json included in this repository
```

Then use it in your Go code:

```go
package main

import (
    "fmt"
    "log"

    "github.com/amikos-tech/pure-tokenizers"
)

func main() {
    // Load tokenizer from file
    tokenizer, err := tokenizers.FromFile("tokenizer.json")
    if err != nil {
        log.Fatal(err)
    }
    defer tokenizer.Close()

    // Tokenize text
    encoding, err := tokenizer.Encode("Hello, world!", tokenizers.WithAddSpecialTokens())
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Tokens:", encoding.Tokens)
    fmt.Println("Token IDs:", encoding.IDs)
}
```

That's it! The library automatically downloads the correct binary for your platform on first use.

## Installation

```bash
go get github.com/amikos-tech/pure-tokenizers
```

## Features

### 🚀 Zero Configuration
The library automatically manages platform-specific binaries. No manual downloads, no build steps, no CGo.

### 🔐 Secure by Default
- SHA256 checksum verification for all downloads
- ABI version compatibility checking
- Secure HTTPS-only downloads

### 🎯 Platform Native
Optimized binaries for each platform and architecture:
- macOS (Intel & Apple Silicon)
- Linux (x86_64, ARM64, including musl)
- Windows (x86_64)

### ⚡ High Performance
Native Rust performance without CGo overhead. Direct FFI calls using purego.

## Usage Examples

### Basic Tokenization

```go
// Load a tokenizer from file
tokenizer, err := tokenizers.FromFile("path/to/tokenizer.json")
if err != nil {
    log.Fatal(err)
}
defer tokenizer.Close()

// Simple encoding
encoding, err := tokenizer.Encode("Hello, world!")

// With special tokens
encoding, err := tokenizer.Encode("Hello, world!", tokenizers.WithAddSpecialTokens())
```

### Advanced Options

```go
// Encoding with custom options
encoding, err := tokenizer.Encode("Your text here",
    tokenizers.WithAddSpecialTokens(),
    tokenizers.WithReturnTokens(),
    tokenizers.WithReturnAttentionMask(),
    tokenizers.WithReturnTypeIDs(),
)

// Create tokenizer with truncation and padding
tokenizer, err := tokenizers.FromFile("tokenizer.json",
    tokenizers.WithTruncation(512, tokenizers.TruncationDirectionRight, tokenizers.TruncationStrategyLongestFirst),
    tokenizers.WithPadding(true, tokenizers.PaddingStrategy{Tag: tokenizers.PaddingStrategyFixed, FixedSize: 512}),
)

// Access different parts of the encoding result
if encoding.Tokens != nil {
    fmt.Println("Tokens:", encoding.Tokens)
}
if encoding.IDs != nil {
    fmt.Println("Token IDs:", encoding.IDs)
}
if encoding.AttentionMask != nil {
    fmt.Println("Attention mask:", encoding.AttentionMask)
}
```

### Decoding Tokens

```go
// Decode token IDs back to text
ids := []uint32{101, 7592, 1010, 2088, 999, 102}
text, err := tokenizer.Decode(ids, true)
fmt.Println(text)  // "hello, world!"
```

### Loading from Configuration Files

```go
// Load tokenizer from a downloaded tokenizer.json file
tokenizer, err := tokenizers.FromFile("path/to/tokenizer.json")

// Load from byte configuration
configBytes, _ := os.ReadFile("tokenizer.json")
tokenizer, err := tokenizers.FromBytes(configBytes)

// Use with custom library path
tokenizer, err := tokenizers.FromFile("tokenizer.json",
    tokenizers.WithLibraryPath("/custom/path/to/libtokenizers.so"))
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TOKENIZERS_LIB_PATH` | Custom library path | Auto-detect |
| `TOKENIZERS_GITHUB_REPO` | GitHub repo for downloads | `amikos-tech/pure-tokenizers` |
| `TOKENIZERS_VERSION` | Library version to download | `latest` |
| `GITHUB_TOKEN` | GitHub API token (for rate limits) | None |

### Library Loading Options

```go
// Use a specific library path
tokenizer, err := tokenizers.FromFile("tokenizer.json",
    tokenizers.WithLibraryPath("/custom/path/to/libtokenizers.so"))

// The library loading priority:
// 1. User-provided path via WithLibraryPath()
// 2. TOKENIZERS_LIB_PATH environment variable
// 3. Cached library in platform directory
// 4. Automatic download from GitHub releases
```

### Cache Management

```go
// Get the cache directory
cachePath := tokenizers.GetCachedLibraryPath()

// Clear the cache
err := tokenizers.ClearLibraryCache()

// Download and cache a specific version
err := tokenizers.DownloadAndCacheLibraryWithVersion("v0.1.0")
```

## Platform Support

| Platform | Architecture | Binary | Status |
|----------|-------------|--------|--------|
| macOS | x86_64 | `.dylib` | ✅ |
| macOS | aarch64 (M1/M2) | `.dylib` | ✅ |
| Linux | x86_64 | `.so` | ✅ |
| Linux | aarch64 | `.so` | ✅ |
| Linux (musl) | x86_64 | `.so` | ✅ |
| Linux (musl) | aarch64 | `.so` | ✅ |
| Windows | x86_64 | `.dll` | ✅ |

## Development

### Building from Source

```bash
# Install Rust (if not already installed)
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# Clone the repository
git clone https://github.com/amikos-tech/pure-tokenizers
cd pure-tokenizers

# Build the Rust library
make build

# Run tests
make test

# Run linting
make lint-fix      # Go linting
make lint-rust     # Rust linting
```

### Project Structure

```
pure-tokenizers/
├── src/           # Rust FFI implementation
├── *.go           # Go bindings
├── download.go    # Auto-download functionality
├── library.go     # Platform-specific FFI loading
└── Makefile       # Build automation
```

### Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

Built on top of the excellent [Hugging Face Tokenizers](https://github.com/huggingface/tokenizers) library.