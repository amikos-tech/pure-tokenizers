# Cache Management Guide

This guide provides comprehensive information about managing the tokenizer cache system in pure-tokenizers.

## Table of Contents
- [Overview](#overview)
- [Cache Types](#cache-types)
- [Directory Structure](#directory-structure)
- [Advanced Usage](#advanced-usage)
- [Troubleshooting](#troubleshooting)

## Overview

pure-tokenizers uses a two-tier caching system:
1. **Library Cache**: Platform-specific tokenizer library binaries
2. **HuggingFace Cache**: Tokenizer configurations from HuggingFace models

Both caches are designed to work transparently but can be managed manually when needed.

## Cache Types

### Library Cache

Stores the compiled tokenizer library (`.so`, `.dylib`, or `.dll` files).

**Default Locations:**
- macOS: `~/Library/Caches/tokenizers/lib/`
- Linux: `~/.cache/tokenizers/lib/`
- Windows: `%APPDATA%/tokenizers/lib/`

### HuggingFace Cache

Stores tokenizer.json files downloaded from HuggingFace Hub.

**Default Locations:**
- macOS: `~/Library/Caches/tokenizers/lib/hf/`
- Linux: `~/.cache/tokenizers/lib/hf/`
- Windows: `%APPDATA%/tokenizers/lib/hf/`

## Directory Structure

### Complete Cache Hierarchy

```
tokenizers/
├── lib/
│   ├── libtokenizers.dylib         # Platform-specific library
│   └── hf/                          # HuggingFace cache root
│       └── models/
│           ├── bert-base-uncased/
│           │   ├── main/
│           │   │   └── tokenizer.json
│           │   ├── v1.0.0/         # Tagged version
│           │   │   └── tokenizer.json
│           │   └── refs/
│           │       └── pr/
│           │           └── 123/    # Pull request revision
│           │               └── tokenizer.json
│           └── organization--model-name/
│               └── main/
│                   └── tokenizer.json
```

### Naming Conventions

- **Model IDs**: Forward slashes in organization names are replaced with double dashes
  - `google/flan-t5-base` → `google--flan-t5-base`
  - `meta-llama/Llama-2-7b-hf` → `meta-llama--Llama-2-7b-hf`
- **Revisions**: Can be branches, tags, or commit hashes
  - Default: `main`
  - Tags: `v1.0.0`, `v2.1.3`
  - Branches: `dev`, `experimental`
  - Commits: `a1b2c3d4` (short hash)

## Advanced Usage

### Pre-populating Cache for Air-Gapped Environments

For environments without internet access, you can pre-populate the cache:

**⚠️ Security Warning**: When manually downloading tokenizers for air-gapped environments, always verify the integrity of the downloaded files. Check file hashes against known good values and download only from trusted sources (official HuggingFace repositories). See the [Verification](#verification) section for examples of integrity checking.

```bash
#!/bin/bash
# Script to pre-populate cache for offline use

MODELS=(
    "bert-base-uncased"
    "gpt2"
    "sentence-transformers/all-MiniLM-L6-v2"
)

# Determine cache directory based on OS
if [[ "$OSTYPE" == "darwin"* ]]; then
    CACHE_DIR="$HOME/Library/Caches/tokenizers/lib/hf/models"
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/tokenizers/lib/hf/models"
else
    echo "Unsupported OS"
    exit 1
fi

# Download tokenizers
for MODEL in "${MODELS[@]}"; do
    # Replace / with -- for directory name
    DIR_NAME="${MODEL//\//--}"
    MODEL_DIR="$CACHE_DIR/$DIR_NAME/main"

    echo "Downloading $MODEL..."
    mkdir -p "$MODEL_DIR"

    # Download tokenizer.json
    curl -L "https://huggingface.co/$MODEL/resolve/main/tokenizer.json" \
         -o "$MODEL_DIR/tokenizer.json"

    echo "Saved to $MODEL_DIR/tokenizer.json"
done

echo "Cache pre-population complete!"
```

### Cache Versioning

When using specific model revisions:

```go
package main

import (
    "github.com/amikos-tech/pure-tokenizers"
)

func main() {
    // Cache different versions separately
    versions := []string{"main", "v1.0.0", "v2.0.0"}

    for _, version := range versions {
        tokenizer, err := tokenizers.FromHuggingFace("bert-base-uncased",
            tokenizers.WithHFRevision(version))
        if err != nil {
            panic(err)
        }
        tokenizer.Close()
    }

    // Each version is cached at:
    // ~/Library/Caches/tokenizers/lib/hf/models/bert-base-uncased/{version}/tokenizer.json
}
```

### Custom Cache Directory

You can override the default cache location:

```go
// Using environment variables
os.Setenv("HF_HOME", "/custom/hf/home")

// Or programmatically
tokenizer, err := tokenizers.FromHuggingFace("bert-base-uncased",
    tokenizers.WithHFCacheDir("/custom/cache/path"))
```

### Cache Inspection

Check what's in your cache:

```go
package main

import (
    "fmt"
    "os"
    "path/filepath"
    "github.com/amikos-tech/pure-tokenizers"
)

func inspectCache() {
    // Get cache directory
    cacheDir := tokenizers.GetHFCacheDir()
    modelsDir := filepath.Join(cacheDir, "models")

    // Walk through cache
    err := filepath.Walk(modelsDir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        if filepath.Base(path) == "tokenizer.json" {
            relPath, _ := filepath.Rel(modelsDir, path)
            fmt.Printf("Found: %s (size: %d bytes)\n", relPath, info.Size())
        }

        return nil
    })

    if err != nil {
        fmt.Printf("Error walking cache: %v\n", err)
    }
}
```

### Selective Cache Clearing

Clear cache for specific models, revisions, or patterns:

#### Using Glob Patterns (Recommended)

The library provides a safe, cross-platform API for pattern-based cache clearing:

```go
// Clear all BERT model variants
cleared, err := tokenizers.ClearHFCachePattern("bert-*")
if err != nil {
    log.Fatalf("Failed to clear cache: %v", err)
}
fmt.Printf("Cleared %d cache entries\n", cleared)

// Clear all models from a specific organization
cleared, err = tokenizers.ClearHFCachePattern("huggingface/*")

// Clear all organization-prefixed models
cleared, err = tokenizers.ClearHFCachePattern("*/*")

// Use wildcards and character matching
cleared, err = tokenizers.ClearHFCachePattern("gpt2-*")  // gpt2-medium, gpt2-large, etc.
cleared, err = tokenizers.ClearHFCachePattern("bert-?ase-*")  // bert-base-*, bert-case-*
```

**Supported Pattern Syntax:**
- `*` - Matches any sequence of characters
- `?` - Matches any single character
- `[abc]` - Matches any character in the set
- `[a-z]` - Matches any character in the range

**Security:** Patterns are validated to prevent directory traversal attacks (`..`) and absolute paths.

#### Manual Cleanup (Shell)

```bash
# Clear all versions of a specific model
rm -rf ~/.cache/tokenizers/lib/hf/models/bert-base-uncased/

# Clear only a specific revision
rm -rf ~/.cache/tokenizers/lib/hf/models/bert-base-uncased/v1.0.0/

# Clear models matching a pattern (less safe, use API when possible)
rm -rf ~/.cache/tokenizers/lib/hf/models/google--*
```

## Troubleshooting

### Common Issues

#### Cache Permission Errors

If you encounter permission errors:

```bash
# Fix permissions on cache directory
chmod -R 755 ~/.cache/tokenizers/
```

#### Corrupted Cache

If a cached tokenizer is corrupted:

```go
// Force re-download by clearing cache first
err := tokenizers.ClearHFModelCache("bert-base-uncased")
if err != nil {
    // Handle error
}

// Now download fresh copy
tokenizer, err := tokenizers.FromHuggingFace("bert-base-uncased")
```

#### Cache Size Management

Monitor and manage cache size:

```bash
# Check total cache size
du -sh ~/.cache/tokenizers/

# Find large tokenizer files
find ~/.cache/tokenizers -name "tokenizer.json" -size +10M -exec ls -lh {} \;

# Clear old files (not accessed in 30 days)
find ~/.cache/tokenizers -name "tokenizer.json" -atime +30 -delete
```

#### Debugging Cache Behavior

Enable verbose logging to debug cache issues:

```go
import (
    "log"
    "os"
)

// Set up logging
log.SetFlags(log.LstdFlags | log.Lshortfile)
os.Setenv("TOKENIZERS_DEBUG", "1")  // If implemented in your version

// Try to load tokenizer
tokenizer, err := tokenizers.FromHuggingFace("bert-base-uncased")
if err != nil {
    log.Printf("Failed to load tokenizer: %v", err)
}
```

### Network Issues

For environments with proxy requirements:

```bash
# Set proxy for downloads
export HTTP_PROXY=http://proxy.example.com:8080
export HTTPS_PROXY=http://proxy.example.com:8080

# For HuggingFace with authentication
export HF_TOKEN=your_token_here
```

### Verification

Verify cache integrity:

```go
package main

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "os"
)

func verifyCacheFile(path string, expectedHash string) bool {
    file, err := os.Open(path)
    if err != nil {
        return false
    }
    defer file.Close()

    hash := sha256.New()
    if _, err := io.Copy(hash, file); err != nil {
        return false
    }

    computedHash := hex.EncodeToString(hash.Sum(nil))
    return computedHash == expectedHash
}
```

## Best Practices

1. **Regular Cleanup**: Periodically clean unused models from cache
2. **Version Pinning**: Use specific revisions in production for reproducibility
3. **Offline Mode**: Test offline mode before deploying to air-gapped environments
4. **Cache Monitoring**: Monitor cache size in production environments
5. **Backup**: For critical applications, backup validated cache directories

## Environment Variables Reference

| Variable | Description | Default |
|----------|-------------|---------|
| `HF_HOME` | HuggingFace home directory | Platform-specific |
| `HF_HUB_CACHE` | HuggingFace hub cache location | `$HF_HOME/hub` |
| `XDG_CACHE_HOME` | Linux cache directory | `~/.cache` |
| `HF_TOKEN` | HuggingFace authentication token | None |
| `TOKENIZERS_LIB_PATH` | Override library path | Auto-detected |