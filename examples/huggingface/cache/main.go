package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	tokenizers "github.com/amikos-tech/pure-tokenizers"
)

func main() {
	fmt.Println("=== HuggingFace Cache Management Examples ===\n")

	// Example 1: Download and cache a model
	if err := downloadAndCache(); err != nil {
		log.Printf("Download example failed: %v\n", err)
	}

	// Example 2: Query cache information
	if err := queryCacheInfo(); err != nil {
		log.Printf("Cache info example failed: %v\n", err)
	}

	// Example 3: Use custom cache directory
	if err := customCacheDirectory(); err != nil {
		log.Printf("Custom cache example failed: %v\n", err)
	}

	// Example 4: Offline mode
	if err := offlineMode(); err != nil {
		log.Printf("Offline mode example failed: %v\n", err)
	}

	// Example 5: Clear specific model cache
	if err := clearModelCache(); err != nil {
		log.Printf("Clear cache example failed: %v\n", err)
	}
}

func downloadAndCache() error {
	fmt.Println("1. Downloading and Caching Models")
	fmt.Println("=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=")

	modelID := "bert-base-uncased"

	// First download - will fetch from HuggingFace
	fmt.Printf("Downloading %s (first time)...\n", modelID)
	start := time.Now()

	tokenizer1, err := tokenizers.FromHuggingFace(modelID)
	if err != nil {
		return fmt.Errorf("failed to download model: %w", err)
	}
	tokenizer1.Close()

	downloadTime := time.Since(start)
	fmt.Printf("  Download completed in %v\n", downloadTime)

	// Second load - will use cache
	fmt.Printf("Loading %s (from cache)...\n", modelID)
	start = time.Now()

	tokenizer2, err := tokenizers.FromHuggingFace(modelID)
	if err != nil {
		return fmt.Errorf("failed to load from cache: %w", err)
	}
	defer tokenizer2.Close()

	cacheTime := time.Since(start)
	fmt.Printf("  Cache load completed in %v\n", cacheTime)
	fmt.Printf("  Speed improvement: %.1fx faster\n\n", float64(downloadTime)/float64(cacheTime))

	return nil
}

func queryCacheInfo() error {
	fmt.Println("2. Querying Cache Information")
	fmt.Println("=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=")

	models := []string{
		"bert-base-uncased",
		"gpt2",
		"distilbert-base-uncased",
	}

	for _, modelID := range models {
		info, err := tokenizers.GetHFCacheInfo(modelID)
		if err != nil {
			fmt.Printf("  %s: Not cached\n", modelID)
			continue
		}

		fmt.Printf("  %s:\n", modelID)
		for key, value := range info {
			fmt.Printf("    %s: %v\n", key, value)
		}
	}
	fmt.Println()

	return nil
}

func customCacheDirectory() error {
	fmt.Println("3. Custom Cache Directory")
	fmt.Println("=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=")

	// Create a temporary custom cache directory
	tempDir, err := os.MkdirTemp("", "tokenizers-custom-cache-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("Using custom cache directory: %s\n", tempDir)

	// Load model with custom cache
	tokenizer, err := tokenizers.FromHuggingFace("distilbert-base-uncased",
		tokenizers.WithHFCacheDir(tempDir),
		tokenizers.WithHFTimeout(30*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to load with custom cache: %w", err)
	}
	defer tokenizer.Close()

	// Verify the file was cached in custom directory
	expectedPath := filepath.Join(tempDir, "models", "distilbert-base-uncased", "main", "tokenizer.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		// Try alternative path structure
		expectedPath = filepath.Join(tempDir, "distilbert-base-uncased", "main", "tokenizer.json")
	}

	if _, err := os.Stat(expectedPath); err == nil {
		fmt.Printf("  Model cached at: %s\n", expectedPath)

		// Get file size
		fileInfo, _ := os.Stat(expectedPath)
		fmt.Printf("  Cache file size: %d bytes\n", fileInfo.Size())
	}

	// Use the tokenizer
	text := "Custom cache directory works!"
	encoding, err := tokenizer.Encode(text)
	if err != nil {
		return fmt.Errorf("failed to encode: %w", err)
	}

	fmt.Printf("  Successfully tokenized: %d tokens\n\n", len(encoding.IDs))

	return nil
}

func offlineMode() error {
	fmt.Println("4. Offline Mode")
	fmt.Println("=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=")

	modelID := "bert-base-uncased"

	// First, ensure model is cached
	fmt.Printf("Ensuring %s is cached...\n", modelID)
	tokenizer1, err := tokenizers.FromHuggingFace(modelID)
	if err != nil {
		return fmt.Errorf("failed to cache model: %w", err)
	}
	tokenizer1.Close()
	fmt.Printf("  Model cached successfully\n")

	// Now try offline mode
	fmt.Printf("Loading %s in offline mode...\n", modelID)
	tokenizer2, err := tokenizers.FromHuggingFace(modelID,
		tokenizers.WithHFOfflineMode(true),
	)
	if err != nil {
		return fmt.Errorf("failed in offline mode: %w", err)
	}
	defer tokenizer2.Close()

	fmt.Printf("  Successfully loaded from cache (no network access)\n")

	// Test with uncached model (should fail)
	fmt.Printf("Trying to load uncached model in offline mode...\n")
	_, err = tokenizers.FromHuggingFace("facebook/bart-large",
		tokenizers.WithHFOfflineMode(true),
	)
	if err != nil {
		fmt.Printf("  Expected error (model not cached): %v\n", err)
	} else {
		fmt.Printf("  Model was already cached\n")
	}
	fmt.Println()

	return nil
}

func clearModelCache() error {
	fmt.Println("5. Clear Model Cache")
	fmt.Println("=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=")

	modelID := "distilbert-base-uncased"

	// Check if model is cached
	info, err := tokenizers.GetHFCacheInfo(modelID)
	if err != nil {
		fmt.Printf("Model %s is not cached\n", modelID)
		// Download it first
		fmt.Printf("Downloading %s to demonstrate cache clearing...\n", modelID)
		tok, err := tokenizers.FromHuggingFace(modelID)
		if err != nil {
			return fmt.Errorf("failed to download model: %w", err)
		}
		tok.Close()

		info, _ = tokenizers.GetHFCacheInfo(modelID)
	}

	if info != nil {
		fmt.Printf("Model %s is cached at: %v\n", modelID, info["path"])
		fmt.Printf("  Cache size: %v bytes\n", info["size"])
	}

	// Clear the cache
	fmt.Printf("Clearing cache for %s...\n", modelID)
	err = tokenizers.ClearHFModelCache(modelID)
	if err != nil {
		fmt.Printf("  Note: Cache clearing may require permissions: %v\n", err)
	} else {
		fmt.Printf("  Cache cleared successfully\n")

		// Verify it's gone
		_, err = tokenizers.GetHFCacheInfo(modelID)
		if err != nil {
			fmt.Printf("  Verified: Model cache removed\n")
		}
	}

	fmt.Println()

	// Note about clearing all cache
	fmt.Println("Note: To clear ALL HuggingFace cache (use with caution):")
	fmt.Println("  err := tokenizers.ClearHFCache()")
	fmt.Println()

	return nil
}