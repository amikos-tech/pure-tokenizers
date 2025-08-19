# CGo-free Tokenizers

> No `C` was harmed in the making of this library.

A Go library that provides tokenization functionality without requiring CGo dependencies. This project creates Go bindings for Rust-based tokenizers using purego for FFI, eliminating the need for CGo while maintaining high performance.

## Features

- **Pure Go interface** with Rust backend performance
- **No CGo dependencies** - uses purego for FFI
- **Cross-platform support** (Windows, macOS, Linux)
- **Automatic library downloading** from GitHub releases
- **Checksum verification** for security
- **Memory-safe FFI** operations
- **Flexible configuration** options

## Installation

```bash
go get github.com/amikos-tech/pure-tokenizers
```

## Quick Start

### Automatic Library Download

The library can automatically download the appropriate platform-specific shared library:

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/amikos-tech/pure-tokenizers"
)

func main() {
    // Create tokenizer with automatic library download (unless already cached)
    tokenizer, err := tokenizers.FromFile("tokenizer.json")
    if err != nil {
        log.Fatal(err)
    }
    defer tokenizer.Close()

	res, err := tokenizer.Encode("Hello, world!")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Tokens:", res.Tokens)
}
```

### Manual Library Path

If you prefer to manage the library yourself:

```go
// get library from https://github.com/amikos-tech/pure-tokenizers/releases
tokenizer, err := tokenizers.FromFile("tokenizer.json", 
    tokenizers.WithLibraryPath("/path/to/libtokenizers.so"))
```

## Library Management

### Automatic Download

The library automatically downloads platform-specific binaries from GitHub releases:

- **macOS**: `libtokenizers-{arch}-apple-darwin.tar.gz`
- **Linux**: `libtokenizers-{arch}-unknown-linux-gnu.tar.gz`  
- **Linux Musl (Alpine)**: `libtokenizers-{arch}-unknown-linux-musl.tar.gz`
- **Windows**: `libtokenizers-{arch}-pc-windows-msvc.tar.gz`

### Cache Locations

Downloaded libraries are cached in platform-appropriate directories:

- **macOS**: `~/Library/Caches/tokenizers/lib/`
- **Linux**: `~/.cache/tokenizers/lib/` or `$XDG_CACHE_HOME/tokenizers/lib/`
- **Windows**: `%APPDATA%/tokenizers/lib/`

### Manual Cache Management

```go
// Download and cache library explicitly
err := tokenizers.DownloadAndCacheLibrary()

// Download specific version
err := tokenizers.DownloadAndCacheLibraryWithVersion("v0.1.0")

// Get cache path
path := tokenizers.GetCachedLibraryPath()

// Clear cache
err := tokenizers.ClearLibraryCache()
```

## Configuration

### Environment Variables

- `TOKENIZERS_LIB_PATH`: Override library path
- `TOKENIZERS_GITHUB_REPO`: Custom GitHub repository (default: `amikos-tech/pure-tokenizers`)
- `TOKENIZERS_VERSION`: Specific version to download (default: `latest`)

### Library Loading Priority

1. Explicit user-provided path via `WithLibraryPath()`
2. `TOKENIZERS_LIB_PATH` environment variable
3. Cached library (if valid)
4. Automatic download to cache

## Security

- **Checksum verification**: All downloads are verified against SHA256 checksums
- **HTTPS downloads**: All network requests use HTTPS
- **Memory safety**: Proper cleanup of FFI resources

## Development

### Building from Source

```bash
# Build Rust library
make build

# Run tests
make test

# Run with coverage
make test-v2
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

