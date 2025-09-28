# HuggingFace Integration Guide

This guide covers the comprehensive HuggingFace Hub integration in pure-tokenizers, including loading tokenizers, authentication, caching, and troubleshooting.

## Table of Contents
- [Overview](#overview)
- [Supported Models](#supported-models)
- [Basic Usage](#basic-usage)
- [Authentication](#authentication)
- [Configuration Options](#configuration-options)
- [Cache System](#cache-system)
- [Migration from Python](#migration-from-python)
- [Advanced Usage](#advanced-usage)
- [Troubleshooting](#troubleshooting)
- [Performance Considerations](#performance-considerations)

## Overview

Pure-tokenizers provides seamless integration with HuggingFace Hub, allowing you to load tokenizers directly from any HuggingFace model repository without manual downloads or file management.

### Key Benefits
- **Zero Configuration**: Automatically downloads and caches tokenizers
- **Offline Support**: Cached tokenizers work without internet connection
- **Authentication**: Access private and gated models
- **Version Control**: Load specific model revisions (branches/tags/commits)
- **Smart Caching**: Efficient storage with automatic cache management

## Supported Models

Pure-tokenizers supports any model on HuggingFace Hub that includes a `tokenizer.json` file. This includes:

### Popular Models
- **BERT Family**: `bert-base-uncased`, `bert-large-cased`, `distilbert-base-uncased`
- **GPT Family**: `gpt2`, `gpt2-medium`, `gpt2-large`, `gpt2-xl`
- **T5 Family**: `google/flan-t5-base`, `google/flan-t5-large`
- **Sentence Transformers**: `sentence-transformers/all-MiniLM-L6-v2`
- **RoBERTa**: `roberta-base`, `roberta-large`
- **BART**: `facebook/bart-base`, `facebook/bart-large`
- **Llama**: `meta-llama/Llama-2-7b-hf` (requires authentication)

### Model ID Format
Model IDs follow the pattern `owner/model-name` or just `model-name` for official models:
- `bert-base-uncased` (official model)
- `google/flan-t5-base` (organization model)
- `username/custom-model` (user model)

## Basic Usage

### Loading a Public Model
```go
package main

import (
    "fmt"
    "log"
    "github.com/amikos-tech/pure-tokenizers"
)

func main() {
    // Load a public model tokenizer
    tokenizer, err := tokenizers.FromHuggingFace("bert-base-uncased")
    if err != nil {
        log.Fatal(err)
    }
    defer tokenizer.Close()

    // Tokenize text
    text := "Hello, how are you?"
    encoding, err := tokenizer.Encode(text, tokenizers.WithAddSpecialTokens())
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Tokens: %v\n", encoding.Tokens)
    fmt.Printf("Token IDs: %v\n", encoding.IDs)
}
```

### Loading with Options
```go
tokenizer, err := tokenizers.FromHuggingFace("gpt2",
    tokenizers.WithHFRevision("main"),              // Specific branch/tag
    tokenizers.WithHFTimeout(30 * time.Second),     // Custom timeout
    tokenizers.WithHFCacheDir("/custom/cache"),     // Custom cache location
)
```

## Authentication

### Setting Up Authentication

HuggingFace uses tokens for authentication. Get your token from [HuggingFace Settings](https://huggingface.co/settings/tokens).

#### Environment Variable (Recommended)
```bash
export HF_TOKEN=hf_xxxxxxxxxxxxxxxxxxxxxxxxx
```

The library automatically reads the `HF_TOKEN` environment variable.

#### Programmatic Authentication
```go
tokenizer, err := tokenizers.FromHuggingFace("meta-llama/Llama-2-7b-hf",
    tokenizers.WithHFToken("hf_xxxxxxxxxxxxxxxxxxxxxxxxx"),
)
```

### Accessing Gated Models

Some models (like Llama) require accepting terms on HuggingFace before access:

1. Visit the model page (e.g., https://huggingface.co/meta-llama/Llama-2-7b-hf)
2. Accept the license terms
3. Use your HF token for authentication

```go
// Ensure you have accepted the model's terms on HuggingFace
tokenizer, err := tokenizers.FromHuggingFace("meta-llama/Llama-2-7b-hf",
    tokenizers.WithHFToken(os.Getenv("HF_TOKEN")),
)
if err != nil {
    // Common errors:
    // - "401 Unauthorized": Invalid token
    // - "403 Forbidden": Terms not accepted
    log.Fatal(err)
}
```

## Configuration Options

### All Available Options

```go
tokenizer, err := tokenizers.FromHuggingFace("model-id",
    // Authentication
    tokenizers.WithHFToken(token),

    // Version control
    tokenizers.WithHFRevision("main"),       // branch, tag, or commit hash

    // Cache management
    tokenizers.WithHFCacheDir("/path/to/cache"),
    tokenizers.WithHFOfflineMode(true),      // Use cached only, no downloads

    // Network configuration
    tokenizers.WithHFTimeout(60 * time.Second),

    // Library configuration (if needed)
    tokenizers.WithLibraryPath("/path/to/libtokenizers.so"),
)
```

### Option Details

#### WithHFToken
Provides authentication for private or gated models.
```go
tokenizers.WithHFToken("hf_xxxxxxxxx")
```

#### WithHFRevision
Loads a specific version of the model. Can be:
- Branch name: `"main"`, `"development"`
- Tag: `"v1.0.0"`
- Commit hash: `"abc123def456"`
```go
tokenizers.WithHFRevision("v2.0.0")
```

#### WithHFCacheDir
Overrides the default cache directory.
```go
tokenizers.WithHFCacheDir("/custom/cache/path")
```

#### WithHFOfflineMode
Forces offline mode - only uses cached tokenizers, no network requests.
```go
tokenizers.WithHFOfflineMode(true)
```

#### WithHFTimeout
Sets custom timeout for downloads (default: 30 seconds).
```go
tokenizers.WithHFTimeout(60 * time.Second)
```

## Cache System

### Cache Locations

Tokenizers are cached in platform-specific directories for optimal performance:

#### Default Locations
- **macOS**: `~/Library/Caches/tokenizers/lib/hf/models/`
- **Linux**: `~/.cache/tokenizers/lib/hf/models/` or `$XDG_CACHE_HOME/tokenizers/lib/hf/models/`
- **Windows**: `%APPDATA%/tokenizers/lib/hf/models/`

#### Cache Structure
```
~/.cache/tokenizers/lib/hf/models/
├── bert-base-uncased/
│   ├── main/
│   │   └── tokenizer.json
│   └── metadata.json
├── gpt2/
│   ├── main/
│   │   └── tokenizer.json
│   └── metadata.json
└── meta-llama--Llama-2-7b-hf/  # Note: "/" replaced with "--"
    ├── main/
    │   └── tokenizer.json
    └── metadata.json
```

### Cache Management

#### Query Cache Information
```go
// Get cache info for a specific model
info, err := tokenizers.GetHFCacheInfo("bert-base-uncased")
if err != nil {
    log.Fatal(err)
}

// info contains:
// - "path": Full path to cached tokenizer
// - "size": File size in bytes
// - "modified": Last modification time
// - "revision": Cached revision
fmt.Printf("Cache info: %+v\n", info)
```

#### Clear Cache

```go
// Clear cache for specific model
err := tokenizers.ClearHFModelCache("bert-base-uncased")

// Clear entire HuggingFace cache
err := tokenizers.ClearHFCache()
```

#### Offline Mode

Use cached tokenizers without network access:

```go
// This will fail if the model is not already cached
tokenizer, err := tokenizers.FromHuggingFace("bert-base-uncased",
    tokenizers.WithHFOfflineMode(true),
)
```

### HuggingFace Hub Cache Integration

Pure-tokenizers can also read from the standard HuggingFace cache if present:

```go
// Set HF_HOME to use existing HuggingFace cache
os.Setenv("HF_HOME", "/path/to/huggingface/cache")

// The library will check this cache before downloading
tokenizer, err := tokenizers.FromHuggingFace("model-id")
```

## Migration from Python

### Python (Transformers Library)
```python
from transformers import AutoTokenizer

# Load tokenizer
tokenizer = AutoTokenizer.from_pretrained("bert-base-uncased")

# With authentication
tokenizer = AutoTokenizer.from_pretrained(
    "private-model",
    token="hf_xxxxx"
)

# With specific revision
tokenizer = AutoTokenizer.from_pretrained(
    "model-id",
    revision="v2.0.0"
)

# Tokenize
tokens = tokenizer("Hello world", return_tensors="pt")
```

### Go (Pure-Tokenizers)
```go
import "github.com/amikos-tech/pure-tokenizers"

// Load tokenizer
tokenizer, err := tokenizers.FromHuggingFace("bert-base-uncased")

// With authentication
tokenizer, err := tokenizers.FromHuggingFace("private-model",
    tokenizers.WithHFToken("hf_xxxxx"),
)

// With specific revision
tokenizer, err := tokenizers.FromHuggingFace("model-id",
    tokenizers.WithHFRevision("v2.0.0"),
)

// Tokenize
encoding, err := tokenizer.Encode("Hello world",
    tokenizers.WithAddSpecialTokens(),
)
```

### Key Differences
1. **Error Handling**: Go requires explicit error checking
2. **Resource Management**: Use `defer tokenizer.Close()` in Go
3. **Options**: Go uses functional options pattern
4. **Return Format**: Go returns structured `Encoding` type

## Advanced Usage

### Batch Processing
```go
func processBatch(tokenizer *tokenizers.Tokenizer, texts []string) error {
    for _, text := range texts {
        encoding, err := tokenizer.Encode(text,
            tokenizers.WithAddSpecialTokens(),
            tokenizers.WithReturnAttentionMask(),
        )
        if err != nil {
            return err
        }

        // Process encoding
        processEncoding(encoding)
    }
    return nil
}
```

### Custom Retry Logic
```go
func loadWithRetry(modelID string, maxRetries int) (*tokenizers.Tokenizer, error) {
    var lastErr error

    for i := 0; i < maxRetries; i++ {
        tokenizer, err := tokenizers.FromHuggingFace(modelID,
            tokenizers.WithHFTimeout(30 * time.Second),
        )
        if err == nil {
            return tokenizer, nil
        }

        lastErr = err

        // Check if error is retryable
        if strings.Contains(err.Error(), "rate limit") {
            time.Sleep(time.Duration(i+1) * 5 * time.Second)
            continue
        }

        return nil, err
    }

    return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}
```

### Preloading Models
```go
// Preload models during initialization
func init() {
    models := []string{
        "bert-base-uncased",
        "gpt2",
        "distilbert-base-uncased",
    }

    for _, model := range models {
        go func(m string) {
            tok, err := tokenizers.FromHuggingFace(m)
            if err != nil {
                log.Printf("Failed to preload %s: %v", m, err)
                return
            }
            tok.Close()
            log.Printf("Preloaded %s", m)
        }(model)
    }
}
```

## Troubleshooting

### Common Issues and Solutions

#### 1. Authentication Errors

**Error**: `401 Unauthorized`
```
Solution: Check your HF token is valid
- Verify token at https://huggingface.co/settings/tokens
- Ensure token has read permissions
- Check token is correctly set in environment or code
```

**Error**: `403 Forbidden`
```
Solution: Accept model terms on HuggingFace
- Visit the model page on HuggingFace
- Accept the license/terms
- Wait a few minutes for propagation
```

#### 2. Network Issues

**Error**: `timeout` or `connection refused`
```
Solution: Check network and proxy settings
- Verify internet connectivity
- Check if behind corporate proxy
- Increase timeout with WithHFTimeout
- Use offline mode if model is cached
```

#### 3. Cache Issues

**Error**: `permission denied`
```
Solution: Check cache directory permissions
- Verify write permissions to cache directory
- Use WithHFCacheDir to specify writable location
- Clear corrupted cache with ClearHFModelCache
```

#### 4. Model Not Found

**Error**: `404 Not Found`
```
Solution: Verify model ID and availability
- Check model exists on HuggingFace
- Verify correct model ID format (owner/model)
- Check if model is public or requires authentication
```

#### 5. Rate Limiting

**Error**: `429 Too Many Requests`
```
Solution: Handle rate limits
- Implement exponential backoff
- Cache models locally for reuse
- Consider using offline mode when possible
- Spread requests over time
```

### Debug Tips

#### Enable Verbose Logging
```go
import "log"

// Set debug logging
log.SetFlags(log.LstdFlags | log.Lshortfile)

// Log all operations
tokenizer, err := tokenizers.FromHuggingFace("model-id")
if err != nil {
    log.Printf("Failed to load tokenizer: %+v", err)
}
```

#### Check Cache Status
```go
// Verify what's cached
info, err := tokenizers.GetHFCacheInfo("model-id")
if err != nil {
    log.Printf("Model not cached: %v", err)
} else {
    log.Printf("Model cached at: %s", info["path"])
}
```

#### Test Connectivity
```go
// Test with a small, public model first
testTokenizer, err := tokenizers.FromHuggingFace("bert-base-uncased")
if err != nil {
    log.Fatal("Cannot connect to HuggingFace: ", err)
}
testTokenizer.Close()
```

## Performance Considerations

### Caching Strategy
- Models are cached after first download
- Subsequent loads are near-instantaneous
- Cache is persistent across application restarts

### Memory Management
```go
// Always close tokenizers when done
tokenizer, err := tokenizers.FromHuggingFace("model-id")
if err != nil {
    return err
}
defer tokenizer.Close()  // Important: releases memory
```

### Concurrent Usage
```go
// Tokenizers are thread-safe for reading
var tokenizer *tokenizers.Tokenizer

func init() {
    var err error
    tokenizer, err = tokenizers.FromHuggingFace("bert-base-uncased")
    if err != nil {
        log.Fatal(err)
    }
}

// Safe to use from multiple goroutines
func processText(text string) (*tokenizers.Encoding, error) {
    return tokenizer.Encode(text)
}
```

### Best Practices
1. **Cache frequently used models**: Load once, reuse many times
2. **Use offline mode in production**: Avoid network dependencies
3. **Implement proper error handling**: Network calls can fail
4. **Set appropriate timeouts**: Balance between reliability and speed
5. **Clean up resources**: Always use `defer tokenizer.Close()`

## See Also

- [Examples](../examples/huggingface/) - Working code examples
- [Cache Management](CACHE_MANAGEMENT.md) - Detailed cache documentation
- [HuggingFace Hub](https://huggingface.co/models) - Browse available models
- [API Reference](https://pkg.go.dev/github.com/amikos-tech/pure-tokenizers) - Complete API documentation