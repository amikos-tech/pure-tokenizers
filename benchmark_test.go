package tokenizers

import (
	"os"
	"path/filepath"
	"testing"
)

var (
	shortText  = "Hello, world!"
	mediumText = "The quick brown fox jumps over the lazy dog. This is a medium length text for benchmarking tokenization performance."
	longText   = `Natural language processing (NLP) is a subfield of linguistics, computer science, and artificial intelligence
concerned with the interactions between computers and human language, in particular how to program computers to process
and analyze large amounts of natural language data. The goal is a computer capable of understanding the contents of documents,
including the contextual nuances of the language within them. The technology can then accurately extract information and insights
contained in the documents as well as categorize and organize the documents themselves. Challenges in natural language processing
frequently involve speech recognition, natural language understanding, and natural language generation. Natural language processing
has its roots in the 1950s. Already in 1950, Alan Turing published an article titled "Computing Machinery and Intelligence" which
proposed what is now called the Turing test as a criterion of intelligence, though at the time that was not articulated as a problem
separate from artificial intelligence.`
)

func setupBenchmark(b *testing.B) *Tokenizer {
	b.Helper()

	testFilePath := "test-data/tokenizer.json"
	if _, err := os.Stat(testFilePath); err == nil {
		tokenizer, err := FromFile(testFilePath)
		if err != nil {
			b.Fatalf("Failed to load tokenizer from test file: %v", err)
		}
		return tokenizer
	}

	cachedPath := GetCachedLibraryPath()
	if _, err := os.Stat(cachedPath); err != nil {
		if err := DownloadLibraryFromGitHub(cachedPath); err != nil {
			b.Fatalf("Failed to download library: %v", err)
		}
	}

	modelID := "bert-base-uncased"
	tokenizer, err := FromHuggingFace(modelID)
	if err != nil {
		b.Fatalf("Failed to create tokenizer from HuggingFace: %v", err)
	}

	return tokenizer
}

func BenchmarkEncode(b *testing.B) {
	tokenizer := setupBenchmark(b)
	defer func() { _ = tokenizer.Close() }()

	testCases := []struct {
		name string
		text string
	}{
		{"Short", shortText},
		{"Medium", mediumText},
		{"Long", longText},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for b.Loop() {
				_, err := tokenizer.Encode(tc.text)
				if err != nil {
					b.Fatalf("Failed to encode: %v", err)
				}
			}
		})
	}
}

func BenchmarkEncodeWithOptions(b *testing.B) {
	tokenizer := setupBenchmark(b)
	defer func() { _ = tokenizer.Close() }()

	testCases := []struct {
		name string
		opts EncodeOption
	}{
		{"Default", nil},
		{"WithTypeIDs", func(eo *EncodeOptions) error { eo.ReturnTypeIDs = true; return nil }},
		{"WithTokens", func(eo *EncodeOptions) error { eo.ReturnTokens = true; return nil }},
		{"WithOffsets", func(eo *EncodeOptions) error { eo.ReturnOffsets = true; return nil }},
		{"AllOptions", WithReturnAllAttributes()},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for b.Loop() {
				if tc.opts != nil {
					_, err := tokenizer.Encode(mediumText, tc.opts)
					if err != nil {
						b.Fatalf("Failed to encode: %v", err)
					}
				} else {
					_, err := tokenizer.Encode(mediumText)
					if err != nil {
						b.Fatalf("Failed to encode: %v", err)
					}
				}
			}
		})
	}
}

func BenchmarkDecode(b *testing.B) {
	tokenizer := setupBenchmark(b)
	defer func() { _ = tokenizer.Close() }()

	encoded, err := tokenizer.Encode(mediumText)
	if err != nil {
		b.Fatalf("Failed to encode test text: %v", err)
	}

	testCases := []struct {
		name              string
		skipSpecialTokens bool
	}{
		{"WithSpecialTokens", false},
		{"SkipSpecialTokens", true},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for b.Loop() {
				_, err := tokenizer.Decode(encoded.IDs, tc.skipSpecialTokens)
				if err != nil {
					b.Fatalf("Failed to decode: %v", err)
				}
			}
		})
	}
}

func BenchmarkBatchEncode(b *testing.B) {
	tokenizer := setupBenchmark(b)
	defer func() { _ = tokenizer.Close() }()

	texts := []string{
		shortText,
		mediumText,
		longText,
		"Another sample text for batch processing.",
		"Tokenization is an important step in NLP pipelines.",
	}

	b.ResetTimer()
	for b.Loop() {
		for _, text := range texts {
			_, err := tokenizer.Encode(text)
			if err != nil {
				b.Fatalf("Failed to encode: %v", err)
			}
		}
	}
}

func BenchmarkFromFile(b *testing.B) {
	testFilePath := "test-data/tokenizer.json"
	if _, err := os.Stat(testFilePath); err != nil {
		tmpDir := b.TempDir()
		testFilePath = filepath.Join(tmpDir, "tokenizer.json")

		modelID := "bert-base-uncased"
		tokenizer, err := FromHuggingFace(modelID)
		if err != nil {
			b.Skipf("Failed to download tokenizer for benchmark setup: %v", err)
		}
		defer func() { _ = tokenizer.Close() }()

		b.Skipf("Test tokenizer file not available at %s", testFilePath)
	}

	b.ResetTimer()
	for b.Loop() {
		tokenizer, err := FromFile(testFilePath)
		if err != nil {
			b.Fatalf("Failed to load tokenizer: %v", err)
		}
		_ = tokenizer.Close()
	}
}

func BenchmarkFromHuggingFace(b *testing.B) {
	originalCache := os.Getenv("HF_HUB_CACHE")
	tmpDir := b.TempDir()
	_ = os.Setenv("HF_HUB_CACHE", tmpDir)
	defer func() {
		if originalCache != "" {
			_ = os.Setenv("HF_HUB_CACHE", originalCache)
		} else {
			_ = os.Unsetenv("HF_HUB_CACHE")
		}
	}()

	modelID := "bert-base-uncased"

	tokenizer, err := FromHuggingFace(modelID)
	if err != nil {
		b.Skipf("Initial download failed: %v", err)
	}
	_ = tokenizer.Close()

	b.Run("CreationOnly", func(b *testing.B) {
		b.ResetTimer()
		for b.Loop() {
			tokenizer, err := FromHuggingFace(modelID)
			if err != nil {
				b.Fatalf("Failed to load tokenizer: %v", err)
			}
			b.StopTimer()
			_ = tokenizer.Close()
			b.StartTimer()
		}
	})

	b.Run("FullLifecycle", func(b *testing.B) {
		b.ResetTimer()
		for b.Loop() {
			tokenizer, err := FromHuggingFace(modelID)
			if err != nil {
				b.Fatalf("Failed to load tokenizer: %v", err)
			}
			_ = tokenizer.Close()
		}
	})
}

func BenchmarkVocabSize(b *testing.B) {
	tokenizer := setupBenchmark(b)
	defer func() { _ = tokenizer.Close() }()

	b.ResetTimer()
	for b.Loop() {
		_, err := tokenizer.VocabSize()
		if err != nil {
			b.Fatalf("Failed to get vocab size: %v", err)
		}
	}
}

func BenchmarkEncodeDecode(b *testing.B) {
	tokenizer := setupBenchmark(b)
	defer func() { _ = tokenizer.Close() }()

	testCases := []struct {
		name string
		text string
	}{
		{"Short", shortText},
		{"Medium", mediumText},
		{"Long", longText},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for b.Loop() {
				encoded, err := tokenizer.Encode(tc.text)
				if err != nil {
					b.Fatalf("Failed to encode: %v", err)
				}

				_, err = tokenizer.Decode(encoded.IDs, false)
				if err != nil {
					b.Fatalf("Failed to decode: %v", err)
				}
			}
		})
	}
}

func BenchmarkTruncation(b *testing.B) {
	tokenizer := setupBenchmark(b)
	defer func() { _ = tokenizer.Close() }()

	truncatedTokenizer, err := FromHuggingFace("bert-base-uncased",
		WithTruncation(128, TruncationDirectionRight, TruncationStrategyLongestFirst))
	if err != nil {
		b.Skipf("Failed to create tokenizer with truncation: %v", err)
	}
	defer func() { _ = truncatedTokenizer.Close() }()

	b.ResetTimer()
	for b.Loop() {
		_, err := truncatedTokenizer.Encode(longText)
		if err != nil {
			b.Fatalf("Failed to encode with truncation: %v", err)
		}
	}
}

func BenchmarkPadding(b *testing.B) {
	tokenizer := setupBenchmark(b)
	defer func() { _ = tokenizer.Close() }()

	paddedTokenizer, err := FromHuggingFace("bert-base-uncased",
		WithPadding(true, PaddingStrategy{Tag: PaddingStrategyFixed, FixedSize: 256}))
	if err != nil {
		b.Skipf("Failed to create tokenizer with padding: %v", err)
	}
	defer func() { _ = paddedTokenizer.Close() }()

	texts := []string{shortText, mediumText}

	b.ResetTimer()
	for b.Loop() {
		for _, text := range texts {
			_, err := paddedTokenizer.Encode(text)
			if err != nil {
				b.Fatalf("Failed to encode with padding: %v", err)
			}
		}
	}
}
