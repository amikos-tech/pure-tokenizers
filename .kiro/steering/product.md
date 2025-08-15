# Product Overview

CGo-free Tokenizers is a library that provides tokenization functionality without requiring C dependencies. The project creates Go bindings for Rust-based tokenizers using purego for FFI, eliminating the need for CGo.

## Key Features
- Pure Go interface with Rust backend
- No C dependencies required
- Cross-platform support (Windows, macOS, Linux)
- Support for various tokenizer formats and configurations
- Memory-safe FFI using purego library

## Architecture
The project consists of:
- Rust core library that exposes C-compatible FFI functions
- Go wrapper that uses purego to call Rust functions directly
- Automatic library downloading and caching system
- Cross-platform shared library support (.so, .dylib, .dll)