# Project Structure

## Root Directory
- `Cargo.toml` - Rust package configuration with FFI library settings
- `go.mod` - Go module definition requiring Go 1.24+
- `Makefile` - Build automation with platform-specific compilation flags
- `tokenizer.json` - Sample tokenizer configuration file

## Source Organization

### Rust Core (`src/`)
- `lib.rs` - Main FFI exports and C-compatible functions
  - Tokenizer lifecycle management (create, encode, decode, free)
  - Memory-safe buffer handling with proper cleanup
  - Cross-platform pointer casting and unsafe operations
- `build.rs` - Build script for C header generation via cbindgen

### Go Bindings (Root)
- `tokenizers.go` - Main Go API with purego FFI bindings
  - Cross-platform shared library loading
  - Functional options pattern for configuration
  - Automatic library discovery and caching
- `utils.go` - Helper functions for memory management and type conversion
- `tokenizers_test.go` - Test suite (currently minimal)

## Key Patterns

### Memory Management
- Rust: Uses `Box::into_raw()` and `Box::from_raw()` for heap allocation
- Go: Implements proper cleanup with defer statements and Close() methods
- FFI: Explicit free functions for all allocated resources

### Error Handling
- Rust: Uses `expect()` for critical failures, returns null pointers for recoverable errors
- Go: Returns Go errors with context, checks for null pointers from FFI calls

### Configuration
- Functional options pattern in Go for flexible API configuration
- Struct-based options for both tokenizer and encoding parameters
- Environment variable support for library path discovery

## Build Artifacts
- `target/` - Rust compilation output and generated C headers
- Platform-specific shared libraries (`.so`, `.dylib`, `.dll`)