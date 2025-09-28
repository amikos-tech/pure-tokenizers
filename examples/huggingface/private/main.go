package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	tokenizers "github.com/amikos-tech/pure-tokenizers"
)

func main() {
	fmt.Println("=== Private and Gated Model Examples ===\n")

	// Example 1: Authentication setup
	showAuthenticationSetup()

	// Example 2: Load with token from environment
	if err := loadWithEnvironmentToken(); err != nil {
		log.Printf("Environment token example failed: %v\n", err)
	}

	// Example 3: Load with explicit token
	if err := loadWithExplicitToken(); err != nil {
		log.Printf("Explicit token example failed: %v\n", err)
	}

	// Example 4: Load gated model (Llama example)
	if err := loadGatedModel(); err != nil {
		log.Printf("Gated model example failed: %v\n", err)
	}

	// Example 5: Error handling for authentication
	if err := demonstrateErrorHandling(); err != nil {
		// This is expected to show various error scenarios
		log.Printf("Error handling demonstration completed\n")
	}
}

func showAuthenticationSetup() {
	fmt.Println("1. Authentication Setup")
	fmt.Println("=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=")

	fmt.Println("To use private or gated models, you need a HuggingFace token:")
	fmt.Println()
	fmt.Println("  1. Get your token from: https://huggingface.co/settings/tokens")
	fmt.Println("  2. Create a token with 'read' permissions")
	fmt.Println("  3. Set it as an environment variable:")
	fmt.Println("     export HF_TOKEN=hf_xxxxxxxxxxxxxxxxxxxxxxxxx")
	fmt.Println()
	fmt.Println("For gated models (like Llama 2):")
	fmt.Println("  1. Visit the model page (e.g., https://huggingface.co/meta-llama/Llama-2-7b-hf)")
	fmt.Println("  2. Accept the license agreement")
	fmt.Println("  3. Wait for approval (usually instant)")
	fmt.Println()

	// Check if token is set
	token := os.Getenv("HF_TOKEN")
	if token != "" {
		masked := maskToken(token)
		fmt.Printf("✓ HF_TOKEN is set: %s\n", masked)
	} else {
		fmt.Println("✗ HF_TOKEN is not set")
		fmt.Println("  Set it with: export HF_TOKEN=your_token_here")
	}
	fmt.Println()
}

func loadWithEnvironmentToken() error {
	fmt.Println("2. Loading with Environment Token")
	fmt.Println("=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=")

	token := os.Getenv("HF_TOKEN")
	if token == "" {
		fmt.Println("HF_TOKEN not found in environment.")
		fmt.Println("Skipping this example. Set HF_TOKEN to run:")
		fmt.Println("  export HF_TOKEN=your_huggingface_token")
		fmt.Println()
		return nil
	}

	// The library automatically uses HF_TOKEN from environment
	// but we can also explicitly pass it
	fmt.Println("Loading model using HF_TOKEN from environment...")

	// Try loading a model (using public model for demonstration)
	tokenizer, err := tokenizers.FromHuggingFace("distilbert-base-uncased")
	if err != nil {
		return fmt.Errorf("failed to load model: %w", err)
	}
	defer tokenizer.Close()

	fmt.Println("✓ Successfully loaded model with environment token")
	fmt.Println("  Note: Token is automatically read from HF_TOKEN environment variable")
	fmt.Println()

	return nil
}

func loadWithExplicitToken() error {
	fmt.Println("3. Loading with Explicit Token")
	fmt.Println("=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=")

	// You can also explicitly provide the token
	token := os.Getenv("HF_TOKEN")
	if token == "" {
		fmt.Println("No token available for explicit example.")
		fmt.Println("This example shows how to pass token programmatically:")
		fmt.Println()
		fmt.Println(`  tokenizer, err := tokenizers.FromHuggingFace("private-model",`)
		fmt.Println(`      tokenizers.WithHFToken("hf_xxxxxxxxx"),`)
		fmt.Println(`  )`)
		fmt.Println()
		return nil
	}

	fmt.Println("Loading model with explicit token...")

	// Load with explicit token
	tokenizer, err := tokenizers.FromHuggingFace("bert-base-uncased",
		tokenizers.WithHFToken(token),
		tokenizers.WithHFRevision("main"),
		tokenizers.WithHFTimeout(30*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to load with explicit token: %w", err)
	}
	defer tokenizer.Close()

	masked := maskToken(token)
	fmt.Printf("✓ Successfully loaded with explicit token: %s\n", masked)

	// Test the tokenizer
	text := "Authentication successful!"
	encoding, err := tokenizer.Encode(text)
	if err != nil {
		return fmt.Errorf("failed to encode: %w", err)
	}

	fmt.Printf("  Tokenized text: %d tokens\n", len(encoding.IDs))
	fmt.Println()

	return nil
}

func loadGatedModel() error {
	fmt.Println("4. Loading Gated Models (Llama 2 Example)")
	fmt.Println("=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=")

	token := os.Getenv("HF_TOKEN")
	if token == "" {
		fmt.Println("Cannot load gated models without authentication.")
		fmt.Println("Gated models like Llama 2 require:")
		fmt.Println("  1. A HuggingFace account")
		fmt.Println("  2. Accepting the model's license on HuggingFace")
		fmt.Println("  3. An authentication token")
		fmt.Println()
		fmt.Println("Example code for loading Llama 2:")
		fmt.Println()
		fmt.Println(`  // After accepting license at https://huggingface.co/meta-llama/Llama-2-7b-hf`)
		fmt.Println(`  tokenizer, err := tokenizers.FromHuggingFace("meta-llama/Llama-2-7b-hf",`)
		fmt.Println(`      tokenizers.WithHFToken(os.Getenv("HF_TOKEN")),`)
		fmt.Println(`  )`)
		fmt.Println()
		return nil
	}

	// Try to load a gated model
	// Note: This will only work if you've accepted the license
	fmt.Println("Attempting to load Llama 2 (requires accepted license)...")

	tokenizer, err := tokenizers.FromHuggingFace("meta-llama/Llama-2-7b-hf",
		tokenizers.WithHFToken(token),
		tokenizers.WithHFTimeout(60*time.Second), // Larger models may take longer
	)

	if err != nil {
		if strings.Contains(err.Error(), "403") {
			fmt.Println("✗ Access denied (403 Forbidden)")
			fmt.Println("  This means you need to:")
			fmt.Println("  1. Visit https://huggingface.co/meta-llama/Llama-2-7b-hf")
			fmt.Println("  2. Accept the license agreement")
			fmt.Println("  3. Wait for approval")
			fmt.Println()
			return nil
		}
		return fmt.Errorf("failed to load Llama 2: %w", err)
	}
	defer tokenizer.Close()

	fmt.Println("✓ Successfully loaded Llama 2!")
	fmt.Println("  You have accepted the license and have access.")

	// Test with Llama tokenizer
	text := "The Llama 2 model is now accessible!"
	encoding, err := tokenizer.Encode(text)
	if err != nil {
		return fmt.Errorf("failed to encode: %w", err)
	}

	fmt.Printf("  Tokenized with Llama 2: %d tokens\n", len(encoding.IDs))
	fmt.Println()

	return nil
}

func demonstrateErrorHandling() error {
	fmt.Println("5. Error Handling for Authentication")
	fmt.Println("=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=")

	fmt.Println("Common authentication errors and their meanings:\n")

	// Simulate various error scenarios
	scenarios := []struct {
		name        string
		modelID     string
		token       string
		expectedErr string
	}{
		{
			name:        "Invalid token format",
			modelID:     "bert-base-uncased",
			token:       "invalid_token",
			expectedErr: "401",
		},
		{
			name:        "Empty token for private model",
			modelID:     "private-org/private-model",
			token:       "",
			expectedErr: "401",
		},
		{
			name:        "Valid token but no access",
			modelID:     "meta-llama/Llama-2-7b-hf",
			token:       "hf_fake_token_xxxxx",
			expectedErr: "403",
		},
	}

	for _, scenario := range scenarios {
		fmt.Printf("Scenario: %s\n", scenario.name)
		fmt.Printf("  Model: %s\n", scenario.modelID)

		var opts []tokenizers.TokenizerOption
		if scenario.token != "" {
			opts = append(opts, tokenizers.WithHFToken(scenario.token))
		}
		opts = append(opts, tokenizers.WithHFTimeout(5*time.Second))

		_, err := tokenizers.FromHuggingFace(scenario.modelID, opts...)

		if err != nil {
			if strings.Contains(err.Error(), "401") {
				fmt.Println("  Error: 401 Unauthorized - Invalid or missing token")
			} else if strings.Contains(err.Error(), "403") {
				fmt.Println("  Error: 403 Forbidden - Valid token but no access to model")
			} else if strings.Contains(err.Error(), "404") {
				fmt.Println("  Error: 404 Not Found - Model doesn't exist or is private")
			} else {
				fmt.Printf("  Error: %v\n", err)
			}
		}
		fmt.Println()
	}

	fmt.Println("Tips for troubleshooting:")
	fmt.Println("  • 401 errors: Check your token is valid and correctly set")
	fmt.Println("  • 403 errors: Accept model license on HuggingFace website")
	fmt.Println("  • 404 errors: Verify model ID and that it's accessible")
	fmt.Println("  • Network errors: Check internet connection and proxy settings")
	fmt.Println()

	return nil
}

func maskToken(token string) string {
	if len(token) < 10 {
		return "***"
	}
	return token[:7] + "..." + token[len(token)-4:]
}