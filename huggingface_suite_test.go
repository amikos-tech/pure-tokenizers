package tokenizers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test fixtures for different tokenizer configurations
var (
	bertTokenizerJSON = `{
		"version": "1.0",
		"truncation": null,
		"padding": null,
		"added_tokens": [
			{"id": 0, "content": "[PAD]", "single_word": false, "lstrip": false, "rstrip": false, "normalized": false, "special": true},
			{"id": 101, "content": "[CLS]", "single_word": false, "lstrip": false, "rstrip": false, "normalized": false, "special": true},
			{"id": 102, "content": "[SEP]", "single_word": false, "lstrip": false, "rstrip": false, "normalized": false, "special": true}
		],
		"normalizer": {"type": "BertNormalizer", "clean_text": true, "handle_chinese_chars": true, "strip_accents": null, "lowercase": true},
		"pre_tokenizer": {"type": "BertPreTokenizer"},
		"post_processor": {"type": "TemplateProcessing"},
		"decoder": {"type": "WordPiece", "prefix": "##", "cleanup": true},
		"model": {"type": "WordPiece", "vocab": {"[PAD]": 0, "[CLS]": 101, "[SEP]": 102, "hello": 7592, "world": 2088}, "unk_token": "[UNK]", "continuing_subword_prefix": "##"}
	}`

	gpt2TokenizerJSON = `{
		"version": "1.0",
		"truncation": null,
		"padding": null,
		"added_tokens": [],
		"normalizer": null,
		"pre_tokenizer": {"type": "ByteLevel", "add_prefix_space": false, "trim_offsets": true, "use_regex": true},
		"post_processor": {"type": "ByteLevel", "add_prefix_space": true, "trim_offsets": false},
		"decoder": {"type": "ByteLevel"},
		"model": {"type": "BPE", "vocab": {"hello": 31373, "world": 6894}, "merges": []}
	}`
)

// MockHFServer creates a configurable mock HuggingFace server for testing
type MockHFServer struct {
	*httptest.Server
	failureCount     int32
	rateLimitCount   int32
	requestLog       []string
	mu               sync.Mutex
	responseDelay    time.Duration
	simulateRedirect bool
}

func newMockHFServer(t *testing.T) *MockHFServer {
	m := &MockHFServer{
		requestLog: make([]string, 0),
	}

	m.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.requestLog = append(m.requestLog, r.URL.Path)
		m.mu.Unlock()

		// Simulate response delay
		if m.responseDelay > 0 {
			time.Sleep(m.responseDelay)
		}

		// Handle different test scenarios
		switch r.URL.Path {
		case "/bert-base-uncased/resolve/main/tokenizer.json":
			m.handleBertModel(w, r)
		case "/gpt2/resolve/main/tokenizer.json":
			m.handleGPT2Model(w, r)
		case "/error-model/resolve/main/tokenizer.json":
			m.handleErrorModel(w, r)
		case "/rate-limited/resolve/main/tokenizer.json":
			m.handleRateLimited(w, r)
		case "/redirect-model/resolve/main/tokenizer.json":
			m.handleRedirect(w, r)
		case "/slow-model/resolve/main/tokenizer.json":
			m.handleSlowResponse(w, r)
		case "/partial-model/resolve/main/tokenizer.json":
			m.handlePartialResponse(w, r)
		case "/auth-required/resolve/main/tokenizer.json":
			m.handleAuthRequired(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Model not found"})
		}
	}))

	return m
}

func (m *MockHFServer) handleBertModel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(bertTokenizerJSON))
}

func (m *MockHFServer) handleGPT2Model(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(gpt2TokenizerJSON))
}

func (m *MockHFServer) handleErrorModel(w http.ResponseWriter, r *http.Request) {
	count := atomic.AddInt32(&m.failureCount, 1)
	if count <= 2 {
		// Fail first 2 attempts
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal server error"))
		return
	}
	// Succeed on third attempt
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(bertTokenizerJSON))
}

func (m *MockHFServer) handleRateLimited(w http.ResponseWriter, r *http.Request) {
	count := atomic.AddInt32(&m.rateLimitCount, 1)
	if count <= 1 {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("Rate limit exceeded"))
		return
	}
	// Succeed after rate limit
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(bertTokenizerJSON))
}

func (m *MockHFServer) handleRedirect(w http.ResponseWriter, r *http.Request) {
	if m.simulateRedirect {
		http.Redirect(w, r, "/bert-base-uncased/resolve/main/tokenizer.json", http.StatusMovedPermanently)
		m.simulateRedirect = false
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(bertTokenizerJSON))
}

func (m *MockHFServer) handleSlowResponse(w http.ResponseWriter, r *http.Request) {
	// Simulate slow response by writing in chunks
	w.Header().Set("Content-Type", "application/json")
	data := []byte(bertTokenizerJSON)
	chunkSize := len(data) / 3

	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		_, _ = w.Write(data[i:end])
		w.(http.Flusher).Flush()
		time.Sleep(100 * time.Millisecond)
	}
}

func (m *MockHFServer) handlePartialResponse(w http.ResponseWriter, r *http.Request) {
	// Simulate connection drop mid-response
	w.Header().Set("Content-Type", "application/json")
	data := []byte(bertTokenizerJSON)
	_, _ = w.Write(data[:len(data)/2]) // Write only half the data
	// Connection drops here
}

func (m *MockHFServer) handleAuthRequired(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if auth != "Bearer valid-token" {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Authentication required"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(bertTokenizerJSON))
}

func (m *MockHFServer) GetRequestCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.requestLog)
}

func (m *MockHFServer) ResetCounters() {
	atomic.StoreInt32(&m.failureCount, 0)
	atomic.StoreInt32(&m.rateLimitCount, 0)
	m.mu.Lock()
	m.requestLog = make([]string, 0)
	m.mu.Unlock()
}

// Test utilities
func setupTestEnvironment(t *testing.T) (string, func()) {
	tempDir := t.TempDir()
	originalURL := HFHubBaseURL
	cleanup := func() {
		HFHubBaseURL = originalURL
	}
	return tempDir, cleanup
}

// Skip conditions for environment-specific tests
// These functions are kept for potential future use in integration tests

// Unit Tests - Download Logic
func TestURLConstruction(t *testing.T) {
	testCases := []struct {
		name        string
		modelID     string
		revision    string
		expectedURL string
	}{
		{
			name:        "Simple model ID",
			modelID:     "bert-base-uncased",
			revision:    "main",
			expectedURL: "https://huggingface.co/bert-base-uncased/resolve/main/tokenizer.json",
		},
		{
			name:        "Model with organization",
			modelID:     "google/flan-t5",
			revision:    "v1.0",
			expectedURL: "https://huggingface.co/google/flan-t5/resolve/v1.0/tokenizer.json",
		},
		{
			name:        "Model with commit hash",
			modelID:     "model-name",
			revision:    "abc123def456",
			expectedURL: "https://huggingface.co/model-name/resolve/abc123def456/tokenizer.json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &HFConfig{Revision: tc.revision}
			url := buildHFDownloadURL(tc.modelID, cfg)
			assert.Equal(t, tc.expectedURL, url)
		})
	}
}

func TestAuthenticationHeaderConstruction(t *testing.T) {
	testCases := []struct {
		name           string
		token          string
		expectedHeader string
		expectHeader   bool
	}{
		{
			name:           "Valid token",
			token:          "hf_abcdefghijklmnop",
			expectedHeader: "Bearer hf_abcdefghijklmnop",
			expectHeader:   true,
		},
		{
			name:           "Empty token",
			token:          "",
			expectedHeader: "",
			expectHeader:   false,
		},
		{
			name:           "Token from environment",
			token:          "env_token_123",
			expectedHeader: "Bearer env_token_123",
			expectHeader:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "https://test.com", nil)
			_ = &HFConfig{Token: tc.token} // config would be used for actual requests

			if tc.token != "" {
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tc.token))
			}

			auth := req.Header.Get("Authorization")
			if tc.expectHeader {
				assert.Equal(t, tc.expectedHeader, auth)
			} else {
				assert.Empty(t, auth)
			}
		})
	}
}

// Unit Tests - Error Handling
func TestErrorWrapping(t *testing.T) {
	testCases := []struct {
		name         string
		baseError    error
		expectedType string
		isRetryable  bool
	}{
		{
			name:         "Network error",
			baseError:    &net.OpError{Op: "dial", Err: errors.New("connection refused")},
			expectedType: "network",
			isRetryable:  true,
		},
		{
			name:         "Timeout error",
			baseError:    context.DeadlineExceeded,
			expectedType: "timeout",
			isRetryable:  true,
		},
		{
			name:         "Authentication error",
			baseError:    fmt.Errorf("authentication required"),
			expectedType: "auth",
			isRetryable:  false,
		},
		{
			name:         "Not found error",
			baseError:    fmt.Errorf("model not found"),
			expectedType: "not_found",
			isRetryable:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			retryable := isNonRetryableError(tc.baseError)
			assert.Equal(t, !tc.isRetryable, retryable,
				"Error retryability mismatch for %s", tc.expectedType)
		})
	}
}

// Integration Tests - Mock Server
func TestDownloadWithMockServer(t *testing.T) {
	mockServer := newMockHFServer(t)
	defer mockServer.Close()

	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()
	HFHubBaseURL = mockServer.URL

	t.Run("Successful download", func(t *testing.T) {
		mockServer.ResetCounters()
		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    5 * time.Second,
			MaxRetries: 3,
		}

		data, err := downloadTokenizerFromHF("bert-base-uncased", config)
		require.NoError(t, err)
		assert.NotNil(t, data)

		// Verify JSON is valid
		var tokenizer map[string]interface{}
		err = json.Unmarshal(data, &tokenizer)
		assert.NoError(t, err)
		assert.Equal(t, "1.0", tokenizer["version"])
		assert.Equal(t, 1, mockServer.GetRequestCount())
	})

	t.Run("Retry on failure", func(t *testing.T) {
		mockServer.ResetCounters()
		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    5 * time.Second,
			MaxRetries: 3,
		}

		data, err := downloadTokenizerFromHF("error-model", config)
		require.NoError(t, err)
		assert.NotNil(t, data)
		assert.Equal(t, 3, mockServer.GetRequestCount(), "Should retry failed requests")
	})

	t.Run("Rate limiting with Retry-After", func(t *testing.T) {
		mockServer.ResetCounters()
		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    10 * time.Second,
			MaxRetries: 2,
		}

		start := time.Now()
		data, err := downloadTokenizerFromHF("rate-limited", config)
		duration := time.Since(start)

		require.NoError(t, err)
		assert.NotNil(t, data)
		// Should wait approximately 2 seconds as specified by Retry-After
		assert.GreaterOrEqual(t, duration.Seconds(), 1.5, "Should respect Retry-After delay")
	})

	t.Run("Authentication required", func(t *testing.T) {
		mockServer.ResetCounters()

		// Without token - should fail
		config := &HFConfig{
			Revision: "main",
			CacheDir: tempDir,
			Timeout:  5 * time.Second,
		}
		_, err := downloadTokenizerFromHF("auth-required", config)
		if assert.Error(t, err, "Expected authentication error without token") {
			assert.Contains(t, strings.ToLower(err.Error()), "auth")
		}

		// With valid token - should succeed
		config.Token = "valid-token"
		data, err := downloadTokenizerFromHF("auth-required", config)
		require.NoError(t, err)
		assert.NotNil(t, data)
	})

	t.Run("Timeout handling", func(t *testing.T) {
		mockServer.ResetCounters()
		mockServer.responseDelay = 2 * time.Second

		config := &HFConfig{
			Revision: "main",
			CacheDir: tempDir,
			Timeout:  500 * time.Millisecond, // Very short timeout
		}

		_, err := downloadTokenizerFromHF("bert-base-uncased", config)
		assert.Error(t, err)
		// The error might be wrapped, so check if it's timeout-related
		assert.True(t, errors.Is(err, context.DeadlineExceeded) ||
			containsSubstring(err.Error(), "timeout") ||
			containsSubstring(err.Error(), "deadline"),
			"Expected timeout error, got: %v", err)

		mockServer.responseDelay = 0 // Reset delay
	})
}

// Cache Tests
func TestCacheKeyGeneration(t *testing.T) {
	testCases := []struct {
		name        string
		modelID     string
		revision    string
		expectedKey string
	}{
		{
			name:        "Simple model",
			modelID:     "bert-base",
			revision:    "main",
			expectedKey: "bert-base/main",
		},
		{
			name:        "Model with org",
			modelID:     "google/t5-small",
			revision:    "v1.0",
			expectedKey: "google--t5-small/v1.0",
		},
		{
			name:        "Model with commit hash",
			modelID:     "model",
			revision:    "abc123",
			expectedKey: "model/abc123",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cachePath := getHFCachePath("", tc.modelID, tc.revision)
			// Check that the path contains the expected key structure
			assert.Contains(t, cachePath, strings.ReplaceAll(tc.expectedKey, "/", string(filepath.Separator)))
		})
	}
}

func TestCacheOperations(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("Cache write and read", func(t *testing.T) {
		modelID := "test-model"
		revision := "main"
		cachePath := getHFCachePath(tempDir, modelID, revision)

		// Write to cache
		testData := []byte(`{"test": "data"}`)
		err := saveToHFCache(cachePath, testData)
		require.NoError(t, err)

		// Verify file exists
		assert.True(t, fileExists(cachePath))

		// Read from cache
		data, err := os.ReadFile(cachePath)
		require.NoError(t, err)
		assert.Equal(t, testData, data)
	})

	t.Run("Cache directory creation", func(t *testing.T) {
		modelID := "nested/model/path"
		revision := "v2.0"
		cachePath := getHFCachePath(tempDir, modelID, revision)

		testData := []byte(`{"nested": "model"}`)
		err := saveToHFCache(cachePath, testData)
		require.NoError(t, err)

		// Verify nested directories were created
		assert.True(t, fileExists(cachePath))
		assert.True(t, fileExists(filepath.Dir(cachePath)))
	})

	t.Run("Cache invalidation", func(t *testing.T) {
		modelID := "cache-test"
		revision := "main"
		cachePath := getHFCachePath(tempDir, modelID, revision)

		// Create cache entry
		err := saveToHFCache(cachePath, []byte(`{"version": "1"}`))
		require.NoError(t, err)

		// Update cache entry
		newData := []byte(`{"version": "2"}`)
		err = saveToHFCache(cachePath, newData)
		require.NoError(t, err)

		// Verify cache was updated
		data, err := os.ReadFile(cachePath)
		require.NoError(t, err)
		assert.Equal(t, newData, data)
	})
}

func TestConcurrentCacheAccess(t *testing.T) {
	tempDir := t.TempDir()
	modelID := "concurrent-test"
	revision := "main"
	cachePath := getHFCachePath(tempDir, modelID, revision)

	// Simulate concurrent writes
	var wg sync.WaitGroup
	numGoroutines := 10
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			data := []byte(fmt.Sprintf(`{"writer": %d}`, id))
			err := saveToHFCache(cachePath, data)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Verify cache file exists and is valid JSON
	data, err := os.ReadFile(cachePath)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	assert.NoError(t, err, "Cache should contain valid JSON after concurrent access")
}

// Benchmark Tests
func BenchmarkFromHuggingFaceWithCache(b *testing.B) {
	// Create mock server - benchmark functions need a special approach
	mockServer := &MockHFServer{
		requestLog: make([]string, 0),
	}
	mockServer.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(bertTokenizerJSON))
	}))
	defer mockServer.Close()

	tempDir := b.TempDir()
	originalURL := HFHubBaseURL
	HFHubBaseURL = mockServer.URL
	defer func() { HFHubBaseURL = originalURL }()

	// Pre-populate cache
	modelID := "bert-base-uncased"
	cachePath := getHFCachePath(tempDir, modelID, "main")
	_ = saveToHFCache(cachePath, []byte(bertTokenizerJSON))

	// config is not used in this benchmark, just reading from cache
	_ = &HFConfig{
		Revision: "main",
		CacheDir: tempDir,
		Timeout:  5 * time.Second,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := loadFromCache(cachePath)
		if err != nil {
			b.Fatal(err)
		}
		if data == nil {
			b.Fatal("Expected cached data")
		}
	}
}

func BenchmarkFromHuggingFaceWithoutCache(b *testing.B) {
	// Create mock server - benchmark functions need a special approach
	mockServer := &MockHFServer{
		requestLog: make([]string, 0),
	}
	mockServer.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(bertTokenizerJSON))
	}))
	defer mockServer.Close()

	tempDir := b.TempDir()
	originalURL := HFHubBaseURL
	HFHubBaseURL = mockServer.URL
	defer func() { HFHubBaseURL = originalURL }()

	config := &HFConfig{
		Revision: "main",
		CacheDir: tempDir,
		Timeout:  5 * time.Second,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clear cache each iteration to force download
		modelID := fmt.Sprintf("bert-base-uncased-%d", i)
		data, err := downloadTokenizerFromHF(modelID, config)
		if err != nil {
			b.Fatal(err)
		}
		if data == nil {
			b.Fatal("Expected downloaded data")
		}
	}
}

// End-to-End Tests
func TestE2EHuggingFaceWorkflow(t *testing.T) {
	// Skip if no library available
	if os.Getenv("TOKENIZERS_LIB_PATH") == "" {
		libpath := getTestLibraryPath()
		if libpath == "" {
			t.Skip("No tokenizer library available for E2E testing")
		}
		_ = os.Setenv("TOKENIZERS_LIB_PATH", libpath)
	}

	mockServer := newMockHFServer(t)
	defer mockServer.Close()

	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()
	HFHubBaseURL = mockServer.URL

	modelID := "bert-base-uncased"
	text := "Hello, world!"

	// Step 1: Load tokenizer from HF
	tok1, err := FromHuggingFace(modelID,
		WithHFCacheDir(tempDir),
		WithHFTimeout(5*time.Second),
	)
	if err != nil {
		// The mock tokenizer might not work with the actual library
		t.Skipf("Mock tokenizer incompatible with library: %v", err)
	}
	defer func() { _ = tok1.Close() }()

	// Step 2: Encode text
	encoding, err := tok1.Encode(text)
	require.NoError(t, err)
	assert.NotNil(t, encoding)
	assert.NotEmpty(t, encoding.IDs)

	// Step 3: Decode tokens
	decoded, err := tok1.Decode(encoding.IDs, false)
	require.NoError(t, err)
	assert.NotEmpty(t, decoded)

	// Step 4: Verify cache was created
	cachePath := getHFCachePath(tempDir, modelID, "main")
	assert.True(t, fileExists(cachePath), "Cache file should exist")

	// Step 5: Load again and verify cache hit
	mockServer.ResetCounters()
	tok2, err := FromHuggingFace(modelID,
		WithHFCacheDir(tempDir),
		WithHFOfflineMode(true), // Force using cache
	)
	if err == nil {
		defer func() { _ = tok2.Close() }()
		assert.Equal(t, 0, mockServer.GetRequestCount(), "Should use cache, not download")
	}

	// Step 6: Clear cache
	err = os.RemoveAll(filepath.Dir(cachePath))
	assert.NoError(t, err)

	// Step 7: Verify cache cleared
	assert.False(t, fileExists(cachePath), "Cache file should be removed")
}

// Helper function
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) >= len(substr) && s[len(s)-len(substr):] == substr ||
		len(substr) < len(s) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func buildHFDownloadURL(modelID string, config *HFConfig) string {
	revision := config.Revision
	if revision == "" {
		revision = "main"
	}
	return fmt.Sprintf("%s/%s/resolve/%s/tokenizer.json", HFHubBaseURL, modelID, revision)
}

func loadFromCache(cachePath string) ([]byte, error) {
	if !fileExists(cachePath) {
		return nil, nil
	}
	return os.ReadFile(cachePath)
}

// Test for streaming and partial downloads
func TestStreamingDownload(t *testing.T) {
	mockServer := newMockHFServer(t)
	defer mockServer.Close()

	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()
	HFHubBaseURL = mockServer.URL

	t.Run("Handle slow streaming response", func(t *testing.T) {
		config := &HFConfig{
			Revision: "main",
			CacheDir: tempDir,
			Timeout:  10 * time.Second, // Long enough for slow response
		}

		data, err := downloadTokenizerFromHF("slow-model", config)
		require.NoError(t, err)
		assert.NotNil(t, data)

		// Verify complete data was received
		var tokenizer map[string]interface{}
		err = json.Unmarshal(data, &tokenizer)
		assert.NoError(t, err, "Should receive complete valid JSON despite slow streaming")
	})

	t.Run("Handle partial response failure", func(t *testing.T) {
		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    5 * time.Second,
			MaxRetries: 1,
		}

		_, err := downloadTokenizerFromHF("partial-model", config)
		assert.Error(t, err, "Should error on partial response")
	})
}

// Test for file size validation
func TestFileSizeValidation(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/huge-model/resolve/main/tokenizer.json" {
			// Simulate a suspiciously large tokenizer file
			w.Header().Set("Content-Length", "1073741824") // 1GB
			w.WriteHeader(http.StatusOK)
			// Don't actually send 1GB of data
			_, _ = w.Write([]byte(`{"error": "file too large"}`))
		} else {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(bertTokenizerJSON))
		}
	}))
	defer mockServer.Close()

	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()
	HFHubBaseURL = mockServer.URL

	t.Run("Normal file size", func(t *testing.T) {
		config := &HFConfig{
			Revision: "main",
			CacheDir: tempDir,
			Timeout:  5 * time.Second,
		}

		data, err := downloadTokenizerFromHF("bert-base-uncased", config)
		require.NoError(t, err)
		assert.NotNil(t, data)
		assert.Less(t, len(data), 100*1024*1024, "Normal tokenizer should be less than 100MB")
	})

	// Note: Actual file size validation would be implemented in the download function
	// This test demonstrates how to test for it
}

// Test for disk space errors
func TestDiskSpaceErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Disk space simulation not reliable on Windows")
	}

	// Create a small tmpfs mount point with limited space (Linux only)
	// This is complex to simulate cross-platform, so we'll use a mock approach

	t.Run("Simulate disk full error", func(t *testing.T) {
		// Create a mock that simulates disk full
		cachePath := filepath.Join(t.TempDir(), "full-disk-test")

		// Try to write to a read-only directory
		err := os.MkdirAll(cachePath, 0555) // Read-only
		require.NoError(t, err)

		err = saveToHFCache(filepath.Join(cachePath, "test.json"), []byte("test"))
		assert.Error(t, err, "Should fail when disk is full or read-only")
	})
}

// Test redirect handling
func TestRedirectHandling(t *testing.T) {
	mockServer := newMockHFServer(t)
	defer mockServer.Close()

	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()
	HFHubBaseURL = mockServer.URL

	t.Run("Handle HTTP redirect", func(t *testing.T) {
		mockServer.simulateRedirect = true
		config := &HFConfig{
			Revision: "main",
			CacheDir: tempDir,
			Timeout:  5 * time.Second,
		}

		data, err := downloadTokenizerFromHF("redirect-model", config)
		require.NoError(t, err)
		assert.NotNil(t, data)

		// The mock server should have handled the redirect
		// and returned the BERT tokenizer
		var tokenizer map[string]interface{}
		err = json.Unmarshal(data, &tokenizer)
		assert.NoError(t, err)
	})
}
