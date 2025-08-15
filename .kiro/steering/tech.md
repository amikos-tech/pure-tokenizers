# Technology Stack

## Languages & Frameworks
- **Rust**: Core tokenization library with C FFI exports
- **Go**: High-level API and bindings using purego for CGo-free FFI
- **C Headers**: Generated via cbindgen for FFI interface

## Key Dependencies

### Rust Dependencies
- `tokenizers` (v0.21.4): Core tokenization functionality
- `libc` (v0.2.174): C standard library bindings
- `cbindgen` (v0.29.0): C header generation
- `criterion` (v0.5.1): Benchmarking framework

### Go Dependencies  
- `github.com/ebitengine/purego` (v0.8.4): CGo-free FFI library
- Go 1.24+ required

## Build System

### Rust Build
```bash
# Standard build with platform-specific flags for macOS
CFLAGS="-I/opt/homebrew/opt/libiconv/include" CXXFLAGS="-I/opt/homebrew/opt/libiconv/include" RUSTFLAGS="-L/opt/homebrew/opt/libiconv/lib -C link-arg=-L/opt/homebrew/opt/libiconv/lib" cargo zigbuild
```

### Go Testing
```bash
# Install test runner
make gotestsum-bin

# Run comprehensive tests with coverage
make test-v2
```

### Linting
```bash
# Fix Go linting issues
make lint-fix
```

## Library Configuration
- Rust crate builds as both `cdylib` and `staticlib`
- Cross-platform shared library naming convention
- Automatic library discovery and caching system