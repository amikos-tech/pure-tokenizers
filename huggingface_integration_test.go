//go:build integration

package tokenizers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipIfNoToken(t *testing.T) string {
	token := os.Getenv("HF_TOKEN")
	if token == "" {
		t.Skip("HF_TOKEN not set, skipping HuggingFace integration test")
	}
	return token
}

func skipIfNoLibrary(t *testing.T) {
	if os.Getenv("TOKENIZERS_LIB_PATH") == "" {
		libPath := getTestLibraryPath()
		if libPath == "" {
			t.Skip("No tokenizer library available for integration testing")
		}
		_ = os.Setenv("TOKENIZERS_LIB_PATH", libPath)
	}
}

func TestHFIntegrationPublicModel(t *testing.T) {
	skipIfNoLibrary(t)

	tempDir := t.TempDir()

	testCases := []struct {
		name     string
		modelID  string
		text     string
		minTokens int
	}{
		{
			name:     "BERT base uncased",
			modelID:  "bert-base-uncased",
			text:     "Hello, how are you doing today?",
			minTokens: 5,
		},
		{
			name:     "GPT2",
			modelID:  "gpt2",
			text:     "The quick brown fox jumps over the lazy dog",
			minTokens: 8,
		},
		{
			name:     "DistilBERT",
			modelID:  "distilbert-base-uncased",
			text:     "Machine learning is fascinating",
			minTokens: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tok, err := FromHuggingFace(tc.modelID,
				WithHFCacheDir(tempDir),
				WithHFTimeout(30*time.Second),
			)
			require.NoError(t, err, "Failed to load %s from HuggingFace", tc.modelID)
			defer func() { _ = tok.Close() }()

			encoding, err := tok.Encode(tc.text)
			require.NoError(t, err, "Failed to encode text with %s", tc.modelID)
			assert.NotNil(t, encoding)
			assert.GreaterOrEqual(t, len(encoding.IDs), tc.minTokens,
				"%s should produce at least %d tokens", tc.modelID, tc.minTokens)

			// Test decode functionality
			decoded, err := tok.Decode(encoding.IDs, false)
			require.NoError(t, err, "Failed to decode tokens with %s", tc.modelID)
			assert.NotEmpty(t, decoded)

			cachePath := getHFCachePath(tempDir, tc.modelID, "main")
			assert.True(t, fileExists(cachePath), "Model should be cached after download")
		})
	}
}

func TestHFIntegrationPrivateModel(t *testing.T) {
	token := skipIfNoToken(t)
	skipIfNoLibrary(t)

	privateModel := os.Getenv("HF_PRIVATE_MODEL")
	if privateModel == "" {
		t.Skip("HF_PRIVATE_MODEL not set, skipping private model test")
	}

	tempDir := t.TempDir()

	tok, err := FromHuggingFace(privateModel,
		WithHFToken(token),
		WithHFCacheDir(tempDir),
		WithHFTimeout(30*time.Second),
	)

	if err != nil {
		if isAuthenticationError(err) {
			t.Skip("Token doesn't have access to private model, skipping")
		}
		require.NoError(t, err, "Failed to load private model")
	}
	defer func() { _ = tok.Close() }()

	testText := "This is a test of private model access"
	encoding, err := tok.Encode(testText)
	require.NoError(t, err)
	assert.NotNil(t, encoding)
	assert.Greater(t, len(encoding.IDs), 0)
}

func TestHFIntegrationCaching(t *testing.T) {
	skipIfNoLibrary(t)

	tempDir := t.TempDir()
	modelID := "bert-base-uncased"

	start := time.Now()
	tok1, err := FromHuggingFace(modelID,
		WithHFCacheDir(tempDir),
		WithHFTimeout(30*time.Second),
	)
	require.NoError(t, err)
	downloadTime := time.Since(start)
	_ = tok1.Close()

	cachePath := getHFCachePath(tempDir, modelID, "main")
	assert.True(t, fileExists(cachePath), "Model should be cached")

	start = time.Now()
	tok2, err := FromHuggingFace(modelID,
		WithHFCacheDir(tempDir),
		WithHFOfflineMode(true),
	)
	require.NoError(t, err, "Should load from cache in offline mode")
	cacheTime := time.Since(start)
	defer func() { _ = tok2.Close() }()

	assert.Less(t, cacheTime, downloadTime/2,
		"Loading from cache should be significantly faster than downloading")

	testText := "Testing cached tokenizer"
	encoding, err := tok2.Encode(testText)
	require.NoError(t, err)
	assert.Greater(t, len(encoding.IDs), 0)
}

func TestHFIntegrationRevisions(t *testing.T) {
	skipIfNoLibrary(t)

	tempDir := t.TempDir()
	modelID := "bert-base-uncased"

	revisions := []string{"main", "refs/pr/1"}

	for _, rev := range revisions {
		t.Run(rev, func(t *testing.T) {
			tok, err := FromHuggingFace(modelID,
				WithHFRevision(rev),
				WithHFCacheDir(tempDir),
				WithHFTimeout(30*time.Second),
			)

			if err != nil && rev != "main" {
				t.Skipf("Revision %s might not exist, skipping", rev)
			}
			require.NoError(t, err)
			defer func() { _ = tok.Close() }()

			cachePath := getHFCachePath(tempDir, modelID, rev)
			assert.True(t, fileExists(cachePath),
				"Model revision %s should be cached separately", rev)
		})
	}
}

func TestHFIntegrationRateLimiting(t *testing.T) {
	skipIfNoLibrary(t)

	tempDir := t.TempDir()
	modelID := "bert-base-uncased"

	concurrentRequests := 5
	errors := make(chan error, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		go func(idx int) {
			cacheDir := filepath.Join(tempDir, "concurrent", string(rune('0'+idx)))
			tok, err := FromHuggingFace(modelID,
				WithHFCacheDir(cacheDir),
				WithHFTimeout(30*time.Second),
			)
			if err == nil {
				_ = tok.Close()
			}
			errors <- err
		}(i)
	}

	successCount := 0
	rateLimitCount := 0

	for i := 0; i < concurrentRequests; i++ {
		err := <-errors
		if err == nil {
			successCount++
		} else if isRateLimitError(err) {
			rateLimitCount++
		}
	}

	assert.Greater(t, successCount, 0, "At least some requests should succeed")
	t.Logf("Success: %d, Rate limited: %d", successCount, rateLimitCount)
}

func TestHFIntegrationLargeModel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large model test in short mode")
	}

	skipIfNoLibrary(t)

	tempDir := t.TempDir()
	largeModel := os.Getenv("HF_LARGE_MODEL")
	if largeModel == "" {
		largeModel = "sentence-transformers/all-MiniLM-L6-v2"
	}

	tok, err := FromHuggingFace(largeModel,
		WithHFCacheDir(tempDir),
		WithHFTimeout(60*time.Second),
	)
	require.NoError(t, err, "Failed to load large model %s", largeModel)
	defer func() { _ = tok.Close() }()

	longText := "This is a test of encoding and decoding with a larger model. " +
		"The model should handle longer sequences and produce consistent results. " +
		"We're testing to ensure that the tokenizer can handle real-world text processing."

	encoding, err := tok.Encode(longText)
	require.NoError(t, err)
	assert.Greater(t, len(encoding.IDs), 10, "Large model should produce many tokens for long text")

	// Test decode functionality
	decoded, err := tok.Decode(encoding.IDs, false)
	require.NoError(t, err)
	assert.NotEmpty(t, decoded)
}

func TestHFIntegrationCacheManagement(t *testing.T) {
	skipIfNoLibrary(t)

	tempDir := t.TempDir()
	modelID := "bert-base-uncased"

	tok, err := FromHuggingFace(modelID,
		WithHFCacheDir(tempDir),
		WithHFTimeout(30*time.Second),
	)
	require.NoError(t, err)
	_ = tok.Close()

	info, err := GetHFCacheInfo(modelID)
	require.NoError(t, err)
	assert.Equal(t, modelID, info["model_id"])

	cachePath := getHFCachePath(tempDir, modelID, "main")
	assert.True(t, fileExists(cachePath))

	err = ClearHFModelCache(modelID)
	assert.NoError(t, err)
}

func TestHFIntegrationNegativeCases(t *testing.T) {
	skipIfNoLibrary(t)

	tempDir := t.TempDir()

	testCases := []struct {
		name        string
		modelID     string
		expectError string
	}{
		{
			name:        "Non-existent model",
			modelID:     "this-model-definitely-does-not-exist-123456789",
			expectError: "", // Could be "not found" or "authentication" depending on HF API behavior
		},
		{
			name:        "Invalid model ID with spaces",
			modelID:     "invalid model name",
			expectError: "invalid character",
		},
		{
			name:        "Model ID too long",
			modelID:     strings.Repeat("a", 97),
			expectError: "cannot exceed 96 characters",
		},
		{
			name:        "Invalid characters in model ID",
			modelID:     "model@invalid#chars",
			expectError: "invalid character",
		},
		{
			name:        "Too many slashes",
			modelID:     "org/suborg/model",
			expectError: "must be in format",
		},
		{
			name:        "Empty organization",
			modelID:     "/model",
			expectError: "owner cannot be empty",
		},
		{
			name:        "Empty model name",
			modelID:     "org/",
			expectError: "repo_name cannot be empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tok, err := FromHuggingFace(tc.modelID,
				WithHFCacheDir(tempDir),
				WithHFTimeout(10*time.Second),
			)

			assert.Error(t, err, "Expected error for %s", tc.modelID)

			// Only check error message if we have a specific expectation
			if tc.expectError != "" {
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tc.expectError),
					"Error should contain expected message for %s", tc.modelID)
			} else if tc.name == "Non-existent model" {
				// For non-existent models, we expect either "not found" or authentication error
				errStr := strings.ToLower(err.Error())
				assert.True(t,
					strings.Contains(errStr, "not found") ||
					strings.Contains(errStr, "404") ||
					strings.Contains(errStr, "authentication") ||
					strings.Contains(errStr, "401"),
					"Error should indicate model not found or authentication required, got: %v", err)
			}

			if tok != nil {
				_ = tok.Close()
				t.Errorf("Expected nil tokenizer for invalid model %s", tc.modelID)
			}
		})
	}
}

func TestHFIntegrationInvalidToken(t *testing.T) {
	skipIfNoLibrary(t)

	privateModel := os.Getenv("HF_PRIVATE_MODEL")
	if privateModel == "" {
		// Use a known private/gated model for testing
		privateModel = "meta-llama/Llama-2-7b-hf"
	}

	tempDir := t.TempDir()

	// Test with invalid token
	tok, err := FromHuggingFace(privateModel,
		WithHFToken("invalid-token-12345"),
		WithHFCacheDir(tempDir),
		WithHFTimeout(10*time.Second),
	)

	assert.Error(t, err, "Should fail with invalid token")
	if tok != nil {
		_ = tok.Close()
	}

	// Error should indicate authentication failure
	errStr := strings.ToLower(err.Error())
	assert.True(t,
		strings.Contains(errStr, "authentication") ||
		strings.Contains(errStr, "forbidden") ||
		strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "401") ||
		strings.Contains(errStr, "403"),
		"Error should indicate authentication failure, got: %v", err)
}

func isAuthenticationError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return isNonRetryableError(err) &&
		(containsSubstring(errStr, "authentication") || containsSubstring(errStr, "forbidden"))
}

func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	return containsSubstring(err.Error(), "rate limit")
}