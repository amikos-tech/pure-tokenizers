# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a CGo-free tokenizers library that provides Go bindings for Rust-based tokenizers using purego for FFI. The project creates platform-specific shared libraries (.so/.dylib/.dll) from Rust code and loads them dynamically in Go without requiring CGo.

## Release Process

### Separate Release Cycles
The project uses separate release cycles for Rust and Go:
- **Rust releases**: Tagged with `rust-vX.Y.Z` (e.g., `rust-v0.1.0`)
- **Go releases**: Tagged with `vX.Y.Z` (e.g., `v0.1.0`)

### Creating Releases
1. **Rust Library Release**:
   ```bash
   git tag rust-v0.1.0
   git push origin rust-v0.1.0
   ```
   This triggers the `rust-release.yml` workflow which builds and releases library artifacts.

2. **Go Module Release**:
   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```
   This triggers the `go-release.yml` workflow which creates a Go module release.
   Note: Ensure a compatible Rust library release exists first.

### CI Workflows
- **rust-ci.yml**: Runs on Rust code changes (src/**, Cargo.toml)
- **go-ci.yml**: Runs on Go code changes (**.go, go.mod)
- **ci.yml**: Main integration tests running on all changes
- **rust-release.yml**: Builds and releases Rust library artifacts
- **go-release.yml**: Creates Go module releases

## Build Commands

### Rust Library Build
```bash
# Build debug version (local development)
make build-debug

# Build release version with zigbuild
make build

# Build for all platforms (cross-compilation)
make build-all-targets

# Create release assets with checksums
make create-release-assets
```

### Testing
```bash
# Run full test suite with coverage (builds library first)
make test

# Run tests with specific library path
make test-lib-path

# Run tests for CI (expects library already built)
make test-ci

# Run Rust tests
make test-rust

# Test download functionality
make test-download

# Run a single Go test
go test -v -run TestFunctionName
```

### Linting
```bash
# Fix Go lint issues
make lint-fix

# Check Rust code with clippy
make lint-rust

# Format Rust code
make fmt-rust
```

## Architecture

### Library Loading Flow
The system follows a priority order for loading the tokenizer library:
1. User-provided path via `WithLibraryPath()` option
2. `TOKENIZERS_LIB_PATH` environment variable
3. Cached library in platform-specific directory
4. Automatic download from GitHub releases to cache

### Core Components

**Go Layer (tokenizers.go)**
- Main `Tokenizer` struct managing FFI calls
- Encoding/decoding operations with configurable options
- Truncation and padding support
- ABI version compatibility checking

**FFI Bridge (library.go, library_windows.go)**
- Platform-specific library loading using purego
- No CGo dependencies - pure Go implementation

**Download System (download.go)**
- Automatic platform detection and asset selection
- GitHub releases integration with checksum verification
- Intelligent caching in OS-appropriate directories

**Rust Layer (src/lib.rs)**
- C-compatible FFI exports using tokenizers crate
- Memory-safe buffer management
- Error code propagation

### Platform Support

The library detects and handles:
- **macOS**: x86_64, aarch64 (M1/M2) → .dylib files
- **Linux**: x86_64, aarch64 → .so files (gnu and musl variants)
- **Windows**: x86_64 → .dll files

### Cache Locations
- **macOS**: `~/Library/Caches/tokenizers/lib/`
- **Linux**: `~/.cache/tokenizers/lib/` or `$XDG_CACHE_HOME/tokenizers/lib/`
- **Windows**: `%APPDATA%/tokenizers/lib/`

## Environment Variables

- `TOKENIZERS_LIB_PATH`: Override library path
- `TOKENIZERS_GITHUB_REPO`: Custom GitHub repository (default: `amikos-tech/pure-tokenizers`)
- `TOKENIZERS_VERSION`: Specific version to download (default: `latest`)
- `GITHUB_TOKEN` or `GH_TOKEN`: GitHub authentication for API requests

## Error Handling

The library uses numeric error codes defined in tokenizers.go:
- `SUCCESS (0)`: Operation successful
- `ErrInvalidUTF8 (-1)`: Invalid UTF-8 in input
- `ErrEncodingFailed (-2)`: Tokenization failed
- `ErrTokenizerCreationFailed (-6)`: Failed to create tokenizer
- Additional error codes for various failure scenarios

All errors are wrapped with context using `pkg/errors` for better debugging.

## Development Setup

```bash
# Install all required tools
make dev-setup

# Check environment configuration
make check-env

# Clean build artifacts and caches
make clean
```

## Key Implementation Details

- **ABI Compatibility**: Version checking ensures Go/Rust interface compatibility (`AbiCompatibilityConstraint = "^0.1.x"`)
- **Memory Safety**: Proper cleanup of FFI resources with defer statements
- **Buffer Management**: Zero-copy where possible, explicit memory management for C strings
- **Cross-platform**: Uses runtime detection for platform-specific library names and paths