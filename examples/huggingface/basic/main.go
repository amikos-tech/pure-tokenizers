package main

import (
	"fmt"
	"log"
	"os"

	tokenizers "github.com/amikos-tech/pure-tokenizers"
)

func main() {
	fmt.Println("=== Basic HuggingFace Tokenizer Examples ===\n")

	// Track failures for exit code
	var failures int

	// Example 1: Load BERT tokenizer
	if err := loadBERT(); err != nil {
		log.Printf("BERT example failed: %v\n", err)
		failures++
		// Continue with other examples even if one fails
	}

	// Example 2: Load GPT-2 tokenizer
	if err := loadGPT2(); err != nil {
		log.Printf("GPT-2 example failed: %v\n", err)
		failures++
	}

	// Example 3: Load DistilBERT tokenizer
	if err := loadDistilBERT(); err != nil {
		log.Printf("DistilBERT example failed: %v\n", err)
		failures++
	}

	// Example 4: Load Sentence Transformer
	if err := loadSentenceTransformer(); err != nil {
		log.Printf("Sentence Transformer example failed: %v\n", err)
		failures++
	}

	// Exit with appropriate code
	if failures > 0 {
		fmt.Printf("\n⚠️  %d example(s) failed. Check network connection and HF_TOKEN if needed.\n", failures)
		os.Exit(1)
	} else {
		fmt.Println("\n✅ All examples completed successfully!")
	}
}

func loadBERT() error {
	fmt.Println("Loading BERT tokenizer...")

	tokenizer, err := tokenizers.FromHuggingFace("bert-base-uncased")
	if err != nil {
		return fmt.Errorf("failed to load BERT: %w", err)
	}
	defer tokenizer.Close()

	// Tokenize sample text
	text := "Hello, how are you doing today? The weather is great."
	encoding, err := tokenizer.Encode(text, tokenizers.WithAddSpecialTokens())
	if err != nil {
		return fmt.Errorf("failed to encode: %w", err)
	}

	fmt.Printf("Model: BERT (bert-base-uncased)\n")
	fmt.Printf("Text: %s\n", text)
	fmt.Printf("Tokens: %v\n", encoding.Tokens)
	fmt.Printf("Token IDs: %v\n", encoding.IDs)
	fmt.Printf("Number of tokens: %d\n\n", len(encoding.IDs))

	// Decode back to text
	decoded, err := tokenizer.Decode(encoding.IDs, true)
	if err != nil {
		return fmt.Errorf("failed to decode: %w", err)
	}
	fmt.Printf("Decoded text: %s\n\n", decoded)

	return nil
}

func loadGPT2() error {
	fmt.Println("Loading GPT-2 tokenizer...")

	tokenizer, err := tokenizers.FromHuggingFace("gpt2")
	if err != nil {
		return fmt.Errorf("failed to load GPT-2: %w", err)
	}
	defer tokenizer.Close()

	// GPT-2 uses different tokenization
	text := "The quick brown fox jumps over the lazy dog."
	encoding, err := tokenizer.Encode(text)
	if err != nil {
		return fmt.Errorf("failed to encode: %w", err)
	}

	fmt.Printf("Model: GPT-2\n")
	fmt.Printf("Text: %s\n", text)
	fmt.Printf("Tokens: %v\n", encoding.Tokens)
	fmt.Printf("Token IDs: %v\n", encoding.IDs)
	fmt.Printf("Number of tokens: %d\n\n", len(encoding.IDs))

	return nil
}

func loadDistilBERT() error {
	fmt.Println("Loading DistilBERT tokenizer...")

	tokenizer, err := tokenizers.FromHuggingFace("distilbert-base-uncased")
	if err != nil {
		return fmt.Errorf("failed to load DistilBERT: %w", err)
	}
	defer tokenizer.Close()

	// DistilBERT example with attention mask
	text := "Machine learning is fascinating!"
	encoding, err := tokenizer.Encode(text,
		tokenizers.WithAddSpecialTokens(),
		tokenizers.WithReturnAttentionMask(),
	)
	if err != nil {
		return fmt.Errorf("failed to encode: %w", err)
	}

	fmt.Printf("Model: DistilBERT (distilbert-base-uncased)\n")
	fmt.Printf("Text: %s\n", text)
	fmt.Printf("Tokens: %v\n", encoding.Tokens)
	fmt.Printf("Token IDs: %v\n", encoding.IDs)
	if encoding.AttentionMask != nil {
		fmt.Printf("Attention Mask: %v\n", encoding.AttentionMask)
	}
	fmt.Printf("Number of tokens: %d\n\n", len(encoding.IDs))

	return nil
}

func loadSentenceTransformer() error {
	fmt.Println("Loading Sentence Transformer tokenizer...")

	tokenizer, err := tokenizers.FromHuggingFace("sentence-transformers/all-MiniLM-L6-v2")
	if err != nil {
		return fmt.Errorf("failed to load sentence transformer: %w", err)
	}
	defer tokenizer.Close()

	// Sentence transformers are often used for embeddings
	sentences := []string{
		"This is the first sentence.",
		"Here's another sentence for comparison.",
		"Sentence transformers are great for semantic similarity.",
	}

	fmt.Printf("Model: Sentence Transformer (all-MiniLM-L6-v2)\n")

	for i, sentence := range sentences {
		encoding, err := tokenizer.Encode(sentence, tokenizers.WithAddSpecialTokens())
		if err != nil {
			return fmt.Errorf("failed to encode sentence %d: %w", i, err)
		}

		fmt.Printf("Sentence %d: %s\n", i+1, sentence)
		fmt.Printf("  Token count: %d\n", len(encoding.IDs))
		fmt.Printf("  First 5 tokens: %v\n", encoding.Tokens[:min(5, len(encoding.Tokens))])
	}
	fmt.Println()

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}