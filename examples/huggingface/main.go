package main

import (
	"fmt"
	"log"
	"os"
	"time"

	tokenizers "github.com/amikos-tech/pure-tokenizers"
)

func main() {
	// Example 1: Load a public model from HuggingFace
	fmt.Println("=== Example 1: Loading BERT tokenizer from HuggingFace ===")
	if err := loadPublicModel(); err != nil {
		log.Printf("Error loading public model: %v\n", err)
	}

	// Example 2: Load with custom cache directory
	fmt.Println("\n=== Example 2: Custom cache directory ===")
	if err := loadWithCustomCache(); err != nil {
		log.Printf("Error with custom cache: %v\n", err)
	}

	// Example 3: Load with authentication (for private/gated models)
	fmt.Println("\n=== Example 3: Loading with authentication ===")
	if err := loadWithAuth(); err != nil {
		log.Printf("Error loading with auth: %v\n", err)
	}

	// Example 4: Offline mode (use cached models only)
	fmt.Println("\n=== Example 4: Offline mode ===")
	if err := loadOffline(); err != nil {
		log.Printf("Error in offline mode: %v\n", err)
	}

	// Example 5: Cache management
	fmt.Println("\n=== Example 5: Cache management ===")
	if err := manageCaches(); err != nil {
		log.Printf("Error managing cache: %v\n", err)
	}
}

func loadPublicModel() error {
	// Load BERT tokenizer from HuggingFace
	tok, err := tokenizers.FromHuggingFace("bert-base-uncased")
	if err != nil {
		return fmt.Errorf("failed to load tokenizer: %w", err)
	}
	defer tok.Close()

	// Tokenize some text
	text := "Hello, how are you doing today?"
	encoding, err := tok.Encode(text, tokenizers.WithAddSpecialTokens())
	if err != nil {
		return fmt.Errorf("failed to encode: %w", err)
	}

	fmt.Printf("Text: %s\n", text)
	fmt.Printf("Tokens: %v\n", encoding.Tokens)
	fmt.Printf("Token IDs: %v\n", encoding.IDs)
	fmt.Printf("Number of tokens: %d\n", len(encoding.IDs))

	return nil
}

func loadWithCustomCache() error {
	// Create a temporary directory for this example
	cacheDir := "/tmp/tokenizers-cache"

	// Load GPT-2 with custom cache directory
	tok, err := tokenizers.FromHuggingFace("gpt2",
		tokenizers.WithHFCacheDir(cacheDir),
		tokenizers.WithHFTimeout(30*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to load tokenizer: %w", err)
	}
	defer tok.Close()

	text := "The quick brown fox jumps over the lazy dog."
	encoding, err := tok.Encode(text)
	if err != nil {
		return fmt.Errorf("failed to encode: %w", err)
	}

	fmt.Printf("Custom cache directory: %s\n", cacheDir)
	fmt.Printf("Model: GPT-2\n")
	fmt.Printf("Text: %s\n", text)
	fmt.Printf("Number of tokens: %d\n", len(encoding.IDs))

	return nil
}

func loadWithAuth() error {
	// Get token from environment variable
	token := os.Getenv("HF_TOKEN")
	if token == "" {
		fmt.Println("No HF_TOKEN found in environment. Skipping authenticated example.")
		fmt.Println("To run this example, set HF_TOKEN environment variable:")
		fmt.Println("  export HF_TOKEN=your_huggingface_token")
		return nil
	}

	// Load a model with authentication
	// This is useful for private models or gated models like Llama
	tok, err := tokenizers.FromHuggingFace("distilbert-base-uncased",
		tokenizers.WithHFToken(token),
		tokenizers.WithHFRevision("main"),
	)
	if err != nil {
		return fmt.Errorf("failed to load tokenizer: %w", err)
	}
	defer tok.Close()

	text := "Authentication allows access to private and gated models."
	encoding, err := tok.Encode(text)
	if err != nil {
		return fmt.Errorf("failed to encode: %w", err)
	}

	fmt.Printf("Loaded with authentication\n")
	fmt.Printf("Text: %s\n", text)
	fmt.Printf("Number of tokens: %d\n", len(encoding.IDs))

	return nil
}

func loadOffline() error {
	// First, ensure we have a cached model
	// This downloads if not cached
	modelID := "bert-base-uncased"
	tok1, err := tokenizers.FromHuggingFace(modelID)
	if err != nil {
		return fmt.Errorf("failed to download model: %w", err)
	}
	tok1.Close()

	// Now load in offline mode (no network access)
	tok2, err := tokenizers.FromHuggingFace(modelID,
		tokenizers.WithHFOfflineMode(true),
	)
	if err != nil {
		return fmt.Errorf("failed to load in offline mode: %w", err)
	}
	defer tok2.Close()

	text := "Offline mode uses only cached models."
	encoding, err := tok2.Encode(text)
	if err != nil {
		return fmt.Errorf("failed to encode: %w", err)
	}

	fmt.Printf("Loaded from cache in offline mode\n")
	fmt.Printf("Model: %s\n", modelID)
	fmt.Printf("Text: %s\n", text)
	fmt.Printf("Number of tokens: %d\n", len(encoding.IDs))

	return nil
}

func manageCaches() error {
	modelID := "bert-base-uncased"

	// Get cache information
	info, err := tokenizers.GetHFCacheInfo(modelID)
	if err != nil {
		return fmt.Errorf("failed to get cache info: %w", err)
	}

	fmt.Printf("Cache info for %s:\n", modelID)
	for key, value := range info {
		fmt.Printf("  %s: %v\n", key, value)
	}

	// Clear cache for a specific model
	fmt.Printf("\nClearing cache for %s...\n", modelID)
	if err := tokenizers.ClearHFModelCache(modelID); err != nil {
		// It's OK if clearing fails (might not have permissions)
		fmt.Printf("Note: Could not clear cache: %v\n", err)
	} else {
		fmt.Println("Cache cleared successfully")
	}

	// To clear all HuggingFace caches (use with caution)
	// err = tokenizers.ClearHFCache()

	return nil
}