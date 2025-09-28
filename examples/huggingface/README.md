# HuggingFace Tokenizer Examples

This example demonstrates how to use the HuggingFace tokenizer loading functionality in pure-tokenizers.

## Features Demonstrated

1. **Loading public models** - Download and use models like BERT, GPT-2, DistilBERT
2. **Custom cache directory** - Control where models are cached locally
3. **Authentication** - Access private or gated models with HF tokens
4. **Offline mode** - Use cached models without network access
5. **Cache management** - Query and clear model caches

## Running the Example

### Basic Usage

```bash
go run main.go
```

### With Authentication

To test authenticated model loading, set your HuggingFace token:

```bash
export HF_TOKEN=your_huggingface_token_here
go run main.go
```

Get your token from: https://huggingface.co/settings/tokens

## Supported Models

You can load any tokenizer from HuggingFace that uses the `tokenizer.json` format. Popular examples include:

- `bert-base-uncased` - BERT base model
- `gpt2` - GPT-2 model
- `distilbert-base-uncased` - DistilBERT
- `sentence-transformers/all-MiniLM-L6-v2` - Sentence transformer
- `google/flan-t5-base` - FLAN-T5 model

## Cache Locations

Models are cached locally for offline use:

- **macOS**: `~/Library/Caches/tokenizers/lib/hf/models/`
- **Linux**: `~/.cache/tokenizers/lib/hf/models/` or `$XDG_CACHE_HOME/tokenizers/lib/hf/models/`
- **Windows**: `%APPDATA%/tokenizers/lib/hf/models/`

## Environment Variables

- `HF_TOKEN` - Your HuggingFace authentication token
- `HF_HOME` - Override the default cache directory

## Error Handling

The example includes comprehensive error handling for common scenarios:
- Network timeouts
- Authentication failures
- Rate limiting
- Model not found
- Cache permission issues

## Advanced Options

### Loading Specific Revisions

```go
tok, err := tokenizers.FromHuggingFace("bert-base-uncased",
    tokenizers.WithHFRevision("refs/pr/1"),
)
```

### Custom Timeouts

```go
tok, err := tokenizers.FromHuggingFace("large-model",
    tokenizers.WithHFTimeout(60 * time.Second),
)
```

### Combining Options

```go
tok, err := tokenizers.FromHuggingFace("private-model",
    tokenizers.WithHFToken(token),
    tokenizers.WithHFCacheDir("/custom/cache"),
    tokenizers.WithHFTimeout(30 * time.Second),
    tokenizers.WithHFRevision("v2.0"),
)
```