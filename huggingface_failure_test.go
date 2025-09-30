package tokenizers

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FailureMode defines the type of failure to inject
type FailureMode string

const (
	FailureModeNone                FailureMode = "none"
	FailureModeTimeout             FailureMode = "timeout"
	FailureModeConnectionReset     FailureMode = "connection_reset"
	FailureModeSlowResponse        FailureMode = "slow_response"
	FailureModePartialResponse     FailureMode = "partial_response"
	FailureModeInvalidJSON         FailureMode = "invalid_json"
	FailureModeTruncatedJSON       FailureMode = "truncated_json"
	FailureModeServerError         FailureMode = "server_error"
	FailureModeRateLimit           FailureMode = "rate_limit"
	FailureModeAuthFailure         FailureMode = "auth_failure"
	FailureModeContentLengthMismatch FailureMode = "content_length_mismatch"
	FailureModeExcessiveSize       FailureMode = "excessive_size"
)

// FailureInjectionServer provides a configurable mock server for testing failure scenarios
type FailureInjectionServer struct {
	*httptest.Server
	failureMode      FailureMode
	failureCount     int32
	requestCount     int32
	retryAfterValue  string
	responseDelay    time.Duration
	statusCode       int
	mu               sync.RWMutex
	tokenizer        []byte
}

// NewFailureInjectionServer creates a new server with configurable failure modes
func NewFailureInjectionServer(t *testing.T) *FailureInjectionServer {
	t.Helper()

	fis := &FailureInjectionServer{
		failureMode: FailureModeNone,
		statusCode:  http.StatusOK,
		tokenizer:   []byte(bertTokenizerJSON),
	}

	fis.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fis.requestCount, 1)
		fis.handleRequest(w, r)
	}))

	return fis
}

// SetFailureMode configures the type of failure to inject
func (fis *FailureInjectionServer) SetFailureMode(mode FailureMode) {
	fis.mu.Lock()
	defer fis.mu.Unlock()
	fis.failureMode = mode
}

// SetFailureCount sets how many times to inject failures before succeeding
func (fis *FailureInjectionServer) SetFailureCount(count int32) {
	atomic.StoreInt32(&fis.failureCount, count)
}

// SetRetryAfter sets the Retry-After header value for rate limiting tests
func (fis *FailureInjectionServer) SetRetryAfter(value string) {
	fis.mu.Lock()
	defer fis.mu.Unlock()
	fis.retryAfterValue = value
}

// SetResponseDelay sets artificial delay for slow response simulation
func (fis *FailureInjectionServer) SetResponseDelay(delay time.Duration) {
	fis.mu.Lock()
	defer fis.mu.Unlock()
	fis.responseDelay = delay
}

// SetStatusCode sets the HTTP status code to return
func (fis *FailureInjectionServer) SetStatusCode(code int) {
	fis.mu.Lock()
	defer fis.mu.Unlock()
	fis.statusCode = code
}

// GetRequestCount returns the number of requests received
func (fis *FailureInjectionServer) GetRequestCount() int32 {
	return atomic.LoadInt32(&fis.requestCount)
}

// ResetCounters resets all counters
func (fis *FailureInjectionServer) ResetCounters() {
	atomic.StoreInt32(&fis.requestCount, 0)
	atomic.StoreInt32(&fis.failureCount, 0)
}

func (fis *FailureInjectionServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	fis.mu.RLock()
	mode := fis.failureMode
	delay := fis.responseDelay
	statusCode := fis.statusCode
	retryAfter := fis.retryAfterValue
	fis.mu.RUnlock()

	// Apply artificial delay
	if delay > 0 {
		time.Sleep(delay)
	}

	// Check if we should inject a failure
	failureCount := atomic.LoadInt32(&fis.failureCount)
	if failureCount > 0 {
		atomic.AddInt32(&fis.failureCount, -1)
		fis.injectFailure(w, r, mode, statusCode, retryAfter)
		return
	}

	// Normal successful response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fis.tokenizer)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(fis.tokenizer)
}

func (fis *FailureInjectionServer) injectFailure(w http.ResponseWriter, r *http.Request, mode FailureMode, statusCode int, retryAfter string) {
	switch mode {
	case FailureModeTimeout:
		// Simulate timeout by sleeping longer than client timeout
		time.Sleep(10 * time.Second)

	case FailureModeConnectionReset:
		// Simulate connection reset by hijacking and closing
		if hijacker, ok := w.(http.Hijacker); ok {
			conn, _, err := hijacker.Hijack()
			if err == nil {
				_ = conn.Close()
				return
			}
		}
		w.WriteHeader(http.StatusInternalServerError)

	case FailureModeSlowResponse:
		// Write response in very small chunks with delays
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		data := fis.tokenizer
		chunkSize := 10 // Very small chunks
		for i := 0; i < len(data); i += chunkSize {
			end := i + chunkSize
			if end > len(data) {
				end = len(data)
			}
			_, _ = w.Write(data[i:end])
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			time.Sleep(50 * time.Millisecond)
		}

	case FailureModePartialResponse:
		// Send only half the data then close connection
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fis.tokenizer)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fis.tokenizer[:len(fis.tokenizer)/2])
		if hijacker, ok := w.(http.Hijacker); ok {
			conn, _, err := hijacker.Hijack()
			if err == nil {
				_ = conn.Close()
				return
			}
		}

	case FailureModeInvalidJSON:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{not valid json}"))

	case FailureModeTruncatedJSON:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		truncated := fis.tokenizer[:len(fis.tokenizer)/2]
		_, _ = w.Write(truncated)

	case FailureModeServerError:
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte("Internal Server Error"))

	case FailureModeRateLimit:
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("Rate limit exceeded"))

	case FailureModeAuthFailure:
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Authentication required"))

	case FailureModeContentLengthMismatch:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "999999") // Wrong size
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fis.tokenizer)

	case FailureModeExcessiveSize:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "2147483648") // 2GB
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))

	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// TestNetworkTimeouts tests various timeout scenarios
func TestNetworkTimeouts(t *testing.T) {
	server := NewFailureInjectionServer(t)
	defer server.Close()

	originalURL := HFHubBaseURL
	HFHubBaseURL = server.URL
	defer func() { HFHubBaseURL = originalURL }()

	tempDir := t.TempDir()

	t.Run("Connection timeout", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeTimeout)
		server.SetFailureCount(1)

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    100 * time.Millisecond,
			MaxRetries: 1,
		}

		_, err := downloadTokenizerFromHF("test-model", config)
		require.Error(t, err)
		errMsg := strings.ToLower(err.Error())
		assert.True(t, strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "deadline exceeded") ||
			strings.Contains(errMsg, "context deadline"),
			"Expected timeout error, got: %v", err)
	})

	t.Run("Read timeout during download", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeSlowResponse)
		server.SetFailureCount(1)

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    200 * time.Millisecond,
			MaxRetries: 1,
		}

		_, err := downloadTokenizerFromHF("test-model", config)
		require.Error(t, err)
	})

	t.Run("Context cancellation", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeNone)
		server.SetResponseDelay(2 * time.Second)

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    100 * time.Millisecond,
			MaxRetries: 1,
		}

		_, err := downloadTokenizerFromHF("test-model", config)
		require.Error(t, err)
	})
}

// TestHTTPErrorCodes tests handling of various HTTP error codes
func TestHTTPErrorCodes(t *testing.T) {
	server := NewFailureInjectionServer(t)
	defer server.Close()

	originalURL := HFHubBaseURL
	HFHubBaseURL = server.URL
	defer func() { HFHubBaseURL = originalURL }()

	tempDir := t.TempDir()

	testCases := []struct {
		name       string
		statusCode int
		shouldRetry bool
	}{
		{"500 Internal Server Error", http.StatusInternalServerError, true},
		{"502 Bad Gateway", http.StatusBadGateway, true},
		{"503 Service Unavailable", http.StatusServiceUnavailable, true},
		{"504 Gateway Timeout", http.StatusGatewayTimeout, true},
		// Note: The current implementation retries all errors except those matching
		// specific strings (authentication, forbidden, not found, invalid).
		// Status 400 doesn't match these patterns, so it will be retried.
		{"401 Unauthorized", http.StatusUnauthorized, false},
		{"403 Forbidden", http.StatusForbidden, false},
		{"404 Not Found", http.StatusNotFound, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server.ResetCounters()
			server.SetFailureMode(FailureModeServerError)
			server.SetStatusCode(tc.statusCode)

			if tc.shouldRetry {
				// Set failure count to 2, then succeed
				server.SetFailureCount(2)
			} else {
				// Always fail for non-retryable errors
				server.SetFailureCount(10)
			}

			config := &HFConfig{
				Revision:   "main",
				CacheDir:   tempDir,
				Timeout:    2 * time.Second,
				MaxRetries: 3,
			}

			data, err := downloadTokenizerFromHF("test-model", config)

			if tc.shouldRetry {
				// Should succeed after retries
				require.NoError(t, err)
				assert.NotNil(t, data)
				assert.GreaterOrEqual(t, server.GetRequestCount(), int32(3), "Should have retried")
			} else {
				// Should fail immediately without retrying
				require.Error(t, err)
				assert.LessOrEqual(t, server.GetRequestCount(), int32(1), "Should not retry non-retryable errors")
			}
		})
	}
}

// TestRateLimiting tests rate limiting scenarios with Retry-After header
func TestRateLimiting(t *testing.T) {
	server := NewFailureInjectionServer(t)
	defer server.Close()

	originalURL := HFHubBaseURL
	HFHubBaseURL = server.URL
	defer func() { HFHubBaseURL = originalURL }()

	tempDir := t.TempDir()

	t.Run("Rate limit with integer Retry-After", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeRateLimit)
		server.SetRetryAfter("1")
		server.SetFailureCount(2)

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    10 * time.Second,
			MaxRetries: 3,
		}

		start := time.Now()
		data, err := downloadTokenizerFromHF("test-model", config)
		duration := time.Since(start)

		require.NoError(t, err)
		assert.NotNil(t, data)
		assert.GreaterOrEqual(t, duration.Seconds(), 1.5, "Should respect Retry-After delay")
	})

	t.Run("Rate limit with HTTP date Retry-After", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeRateLimit)
		retryTime := time.Now().Add(500 * time.Millisecond)
		server.SetRetryAfter(retryTime.UTC().Format(http.TimeFormat))
		server.SetFailureCount(1)

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    10 * time.Second,
			MaxRetries: 2,
		}

		start := time.Now()
		data, err := downloadTokenizerFromHF("test-model", config)
		duration := time.Since(start)

		require.NoError(t, err)
		assert.NotNil(t, data)
		// Be lenient with timing - just verify it took some time (allow for execution overhead)
		assert.GreaterOrEqual(t, duration.Milliseconds(), int64(200), "Should respect Retry-After delay")
	})

	t.Run("Rate limit exhaustion", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeRateLimit)
		server.SetRetryAfter("1")
		server.SetFailureCount(10) // More than max retries

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    10 * time.Second,
			MaxRetries: 3,
		}

		_, err := downloadTokenizerFromHF("test-model", config)
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "rate limit")
	})

	t.Run("Invalid Retry-After falls back to exponential backoff", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeRateLimit)
		server.SetRetryAfter("invalid-value")
		server.SetFailureCount(2)

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    10 * time.Second,
			MaxRetries: 3,
		}

		data, err := downloadTokenizerFromHF("test-model", config)
		require.NoError(t, err)
		assert.NotNil(t, data)
	})

	t.Run("Excessive Retry-After capped at max", func(t *testing.T) {
		// Test that parseRetryAfter caps excessive values
		testCases := []struct {
			value    string
			expected time.Duration
		}{
			{"3600", HFMaxRetryAfterDelay},    // 1 hour -> capped
			{"10000", HFMaxRetryAfterDelay},   // Very large -> capped
			{"300", HFMaxRetryAfterDelay},     // 5 minutes -> exactly at cap
			{"299", 299 * time.Second},        // Just under cap -> not capped
			{"60", 60 * time.Second},          // 1 minute -> not capped
		}

		for _, tc := range testCases {
			duration := parseRetryAfter(tc.value)
			assert.Equal(t, tc.expected, duration,
				"parseRetryAfter(%s) should return %v", tc.value, tc.expected)
		}
	})
}

// TestPartialAndCorruptedResponses tests handling of incomplete or malformed data
func TestPartialAndCorruptedResponses(t *testing.T) {
	server := NewFailureInjectionServer(t)
	defer server.Close()

	originalURL := HFHubBaseURL
	HFHubBaseURL = server.URL
	defer func() { HFHubBaseURL = originalURL }()

	tempDir := t.TempDir()

	t.Run("Partial response - connection drop", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModePartialResponse)
		server.SetFailureCount(1)

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    5 * time.Second,
			MaxRetries: 1,
		}

		_, err := downloadTokenizerFromHF("test-model", config)
		require.Error(t, err)
	})

	t.Run("Invalid JSON response", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeInvalidJSON)
		server.SetFailureCount(1)

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    5 * time.Second,
			MaxRetries: 1,
		}

		_, err := downloadTokenizerFromHF("test-model", config)
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "invalid")
	})

	t.Run("Truncated JSON response", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeTruncatedJSON)
		server.SetFailureCount(1)

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    5 * time.Second,
			MaxRetries: 1,
		}

		_, err := downloadTokenizerFromHF("test-model", config)
		require.Error(t, err)
	})

	t.Run("Retry succeeds after partial response", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModePartialResponse)
		server.SetFailureCount(2)

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    5 * time.Second,
			MaxRetries: 3,
		}

		data, err := downloadTokenizerFromHF("test-model", config)
		require.NoError(t, err)
		assert.NotNil(t, data)
		assert.Equal(t, int32(3), server.GetRequestCount())
	})
}

// TestContentLengthValidation tests Content-Length header handling
func TestContentLengthValidation(t *testing.T) {
	server := NewFailureInjectionServer(t)
	defer server.Close()

	originalURL := HFHubBaseURL
	HFHubBaseURL = server.URL
	defer func() { HFHubBaseURL = originalURL }()

	tempDir := t.TempDir()

	t.Run("Excessive file size rejected", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeExcessiveSize)
		server.SetFailureCount(1)

		config := &HFConfig{
			Revision:         "main",
			CacheDir:         tempDir,
			Timeout:          5 * time.Second,
			MaxRetries:       1,
			MaxTokenizerSize: DefaultMaxTokenizerSize,
		}

		_, err := downloadTokenizerFromHF("test-model", config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too large")
	})

	t.Run("Custom size limit enforced", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeExcessiveSize)
		server.SetFailureCount(1)

		config := &HFConfig{
			Revision:         "main",
			CacheDir:         tempDir,
			Timeout:          5 * time.Second,
			MaxRetries:       1,
			MaxTokenizerSize: 1024, // Very small limit
		}

		_, err := downloadTokenizerFromHF("test-model", config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too large")
	})
}

// TestAuthenticationFailures tests various authentication scenarios
func TestAuthenticationFailures(t *testing.T) {
	server := NewFailureInjectionServer(t)
	defer server.Close()

	originalURL := HFHubBaseURL
	HFHubBaseURL = server.URL
	defer func() { HFHubBaseURL = originalURL }()

	tempDir := t.TempDir()

	t.Run("Missing authentication", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeAuthFailure)
		server.SetFailureCount(10)

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    5 * time.Second,
			MaxRetries: 3,
			Token:      "", // No token
		}

		_, err := downloadTokenizerFromHF("test-model", config)
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "authentication")
		// Should not retry auth errors
		assert.Equal(t, int32(1), server.GetRequestCount())
	})

	t.Run("Invalid token format", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeAuthFailure)
		server.SetFailureCount(10)

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    5 * time.Second,
			MaxRetries: 3,
			Token:      "invalid_token",
		}

		_, err := downloadTokenizerFromHF("test-model", config)
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "authentication")
	})
}

// TestConcurrentDownloadFailures tests failure handling with concurrent operations
func TestConcurrentDownloadFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	server := NewFailureInjectionServer(t)
	defer server.Close()

	originalURL := HFHubBaseURL
	HFHubBaseURL = server.URL
	defer func() { HFHubBaseURL = originalURL }()

	tempDir := t.TempDir()

	t.Run("Concurrent downloads with intermittent failures", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeServerError)
		server.SetStatusCode(http.StatusInternalServerError)
		server.SetFailureCount(5) // Some failures

		var wg sync.WaitGroup
		numGoroutines := 10
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				config := &HFConfig{
					Revision:   "main",
					CacheDir:   filepath.Join(tempDir, fmt.Sprintf("cache-%d", id)),
					Timeout:    5 * time.Second,
					MaxRetries: 3,
				}

				modelID := fmt.Sprintf("test-model-%d", id)
				data, err := downloadTokenizerFromHF(modelID, config)
				if err != nil {
					errors <- err
				} else if data == nil {
					errors <- fmt.Errorf("nil data for model %s", modelID)
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		var errorList []error
		for err := range errors {
			errorList = append(errorList, err)
		}

		// Some should succeed, some may fail depending on timing
		t.Logf("Concurrent downloads: %d errors out of %d attempts", len(errorList), numGoroutines)
	})
}

// TestSlowNetworkConditions tests behavior under degraded network
func TestSlowNetworkConditions(t *testing.T) {
	server := NewFailureInjectionServer(t)
	defer server.Close()

	originalURL := HFHubBaseURL
	HFHubBaseURL = server.URL
	defer func() { HFHubBaseURL = originalURL }()

	tempDir := t.TempDir()

	t.Run("Slow but successful download", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeNone)
		server.SetResponseDelay(500 * time.Millisecond)

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    2 * time.Second,
			MaxRetries: 1,
		}

		start := time.Now()
		data, err := downloadTokenizerFromHF("test-model", config)
		duration := time.Since(start)

		require.NoError(t, err)
		assert.NotNil(t, data)
		assert.GreaterOrEqual(t, duration.Milliseconds(), int64(500))
	})

	t.Run("Slow download exceeds timeout", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeNone)
		server.SetResponseDelay(2 * time.Second)

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    500 * time.Millisecond,
			MaxRetries: 1,
		}

		_, err := downloadTokenizerFromHF("test-model", config)
		require.Error(t, err)
	})
}

// TestConnectionResetScenarios tests connection reset handling
func TestConnectionResetScenarios(t *testing.T) {
	server := NewFailureInjectionServer(t)
	defer server.Close()

	originalURL := HFHubBaseURL
	HFHubBaseURL = server.URL
	defer func() { HFHubBaseURL = originalURL }()

	tempDir := t.TempDir()

	t.Run("Connection reset with retry", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeConnectionReset)
		server.SetFailureCount(2)

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    5 * time.Second,
			MaxRetries: 3,
		}

		data, err := downloadTokenizerFromHF("test-model", config)
		require.NoError(t, err)
		assert.NotNil(t, data)
		assert.GreaterOrEqual(t, server.GetRequestCount(), int32(3))
	})
}

// TestCacheCorruptionWithDownload tests cache corruption scenarios
func TestCacheCorruptionWithDownload(t *testing.T) {
	server := NewFailureInjectionServer(t)
	defer server.Close()

	originalURL := HFHubBaseURL
	HFHubBaseURL = server.URL
	defer func() { HFHubBaseURL = originalURL }()

	tempDir := t.TempDir()

	t.Run("Corrupted cache triggers redownload", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeNone)

		modelID := "test-model"
		cachePath := getHFCachePath(tempDir, modelID, "main")

		// Create corrupted cache file
		err := os.MkdirAll(filepath.Dir(cachePath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(cachePath, []byte("corrupted json"), 0644)
		require.NoError(t, err)

		// Cache load should fail, triggering download
		data, err := loadFromCacheWithValidation(cachePath, 0)
		assert.Error(t, err)
		assert.Nil(t, data)

		// Download should succeed
		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    5 * time.Second,
			MaxRetries: 1,
		}

		data, err = downloadTokenizerFromHF(modelID, config)
		require.NoError(t, err)
		assert.NotNil(t, data)
	})

	t.Run("Read-only cache directory", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Skipping test when running as root")
		}

		server.ResetCounters()
		server.SetFailureMode(FailureModeNone)

		readOnlyDir := filepath.Join(tempDir, "readonly")
		err := os.MkdirAll(readOnlyDir, 0555)
		require.NoError(t, err)
		defer func() { _ = os.Chmod(readOnlyDir, 0755) }()

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   readOnlyDir,
			Timeout:    5 * time.Second,
			MaxRetries: 1,
		}

		// Download should succeed but cache write may fail
		data, err := downloadTokenizerFromHF("test-model", config)
		require.NoError(t, err)
		assert.NotNil(t, data)
	})
}

// BenchmarkDownloadWithFailureRecovery benchmarks performance with intermittent failures
func BenchmarkDownloadWithFailureRecovery(b *testing.B) {
	server := NewFailureInjectionServer(&testing.T{})
	defer server.Close()

	originalURL := HFHubBaseURL
	HFHubBaseURL = server.URL
	defer func() { HFHubBaseURL = originalURL }()

	tempDir := b.TempDir()

	// Set up intermittent failures (fail every 3rd request)
	server.SetFailureMode(FailureModeServerError)
	server.SetStatusCode(http.StatusInternalServerError)

	config := &HFConfig{
		Revision:   "main",
		CacheDir:   tempDir,
		Timeout:    5 * time.Second,
		MaxRetries: 3,
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Set failure for this iteration
		if i%3 == 0 {
			server.SetFailureCount(1)
		}

		modelID := fmt.Sprintf("bench-model-%d", i)
		_, _ = downloadTokenizerFromHF(modelID, config)
	}
}

// BenchmarkConcurrentDownloadsWithFailures benchmarks concurrent downloads with failures
func BenchmarkConcurrentDownloadsWithFailures(b *testing.B) {
	server := NewFailureInjectionServer(&testing.T{})
	defer server.Close()

	originalURL := HFHubBaseURL
	HFHubBaseURL = server.URL
	defer func() { HFHubBaseURL = originalURL }()

	tempDir := b.TempDir()

	server.SetFailureMode(FailureModeServerError)
	server.SetStatusCode(http.StatusInternalServerError)
	server.SetFailureCount(int32(b.N / 4)) // 25% failure rate

	config := &HFConfig{
		Revision:   "main",
		CacheDir:   tempDir,
		Timeout:    5 * time.Second,
		MaxRetries: 2,
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			modelID := fmt.Sprintf("bench-model-%d", i)
			_, _ = downloadTokenizerFromHF(modelID, config)
			i++
		}
	})
}

// TestDialerFailures tests low-level connection failures
func TestDialerFailures(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("Connection refused", func(t *testing.T) {
		// Use a closed port
		originalURL := HFHubBaseURL
		HFHubBaseURL = "http://localhost:9999"
		defer func() { HFHubBaseURL = originalURL }()

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    1 * time.Second,
			MaxRetries: 2,
		}

		_, err := downloadTokenizerFromHF("test-model", config)
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "connection")
	})

	t.Run("Invalid hostname", func(t *testing.T) {
		originalURL := HFHubBaseURL
		HFHubBaseURL = "http://this-domain-does-not-exist-12345.invalid"
		defer func() { HFHubBaseURL = originalURL }()

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    2 * time.Second,
			MaxRetries: 1,
		}

		_, err := downloadTokenizerFromHF("test-model", config)
		require.Error(t, err)
	})
}

// TestContextCancellation tests explicit context cancellation
func TestContextCancellation(t *testing.T) {
	server := NewFailureInjectionServer(t)
	defer server.Close()

	originalURL := HFHubBaseURL
	HFHubBaseURL = server.URL
	defer func() { HFHubBaseURL = originalURL }()

	t.Run("Cancel during download", func(t *testing.T) {
		server.ResetCounters()
		server.SetFailureMode(FailureModeNone)
		server.SetResponseDelay(2 * time.Second)

		// Create a custom HTTP client with cancellable context
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Cancel after 100ms
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/test-model/resolve/main/tokenizer.json", nil)
		require.NoError(t, err)

		client := getHFHTTPClient(&HFConfig{})
		_, err = client.Do(req)
		require.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled) ||
			strings.Contains(strings.ToLower(err.Error()), "cancel"),
			"Expected cancellation error")
	})
}

// TestErrorMessageQuality verifies that error messages are helpful for debugging
func TestErrorMessageQuality(t *testing.T) {
	server := NewFailureInjectionServer(t)
	defer server.Close()

	originalURL := HFHubBaseURL
	HFHubBaseURL = server.URL
	defer func() { HFHubBaseURL = originalURL }()

	tempDir := t.TempDir()

	testCases := []struct {
		name           string
		failureMode    FailureMode
		statusCode     int
		expectedSubstr []string
	}{
		{
			name:           "Server error message",
			failureMode:    FailureModeServerError,
			statusCode:     http.StatusInternalServerError,
			expectedSubstr: []string{"status", "500"},
		},
		{
			name:           "Auth error message",
			failureMode:    FailureModeAuthFailure,
			statusCode:     http.StatusUnauthorized,
			expectedSubstr: []string{"authentication"},
		},
		{
			name:           "Rate limit message",
			failureMode:    FailureModeRateLimit,
			statusCode:     http.StatusTooManyRequests,
			expectedSubstr: []string{"rate limit"},
		},
		{
			name:           "Invalid JSON message",
			failureMode:    FailureModeInvalidJSON,
			statusCode:     http.StatusOK,
			expectedSubstr: []string{"invalid", "json"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server.ResetCounters()
			server.SetFailureMode(tc.failureMode)
			server.SetStatusCode(tc.statusCode)
			server.SetFailureCount(10)

			config := &HFConfig{
				Revision:   "main",
				CacheDir:   tempDir,
				Timeout:    2 * time.Second,
				MaxRetries: 1,
			}

			_, err := downloadTokenizerFromHF("test-model", config)
			require.Error(t, err)

			errMsg := strings.ToLower(err.Error())
			for _, substr := range tc.expectedSubstr {
				assert.Contains(t, errMsg, strings.ToLower(substr),
					"Error message should contain '%s'", substr)
			}
		})
	}
}

// TestDNSFailures tests DNS resolution failures
func TestDNSFailures(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("DNS resolution failure", func(t *testing.T) {
		originalURL := HFHubBaseURL
		HFHubBaseURL = "http://invalid.example.invalid"
		defer func() { HFHubBaseURL = originalURL }()

		config := &HFConfig{
			Revision:   "main",
			CacheDir:   tempDir,
			Timeout:    2 * time.Second,
			MaxRetries: 1,
		}

		_, err := downloadTokenizerFromHF("test-model", config)
		require.Error(t, err)

		// Check for DNS or connection error
		var dnsErr *net.DNSError
		var opErr *net.OpError
		assert.True(t,
			errors.As(err, &dnsErr) || errors.As(err, &opErr) ||
			strings.Contains(strings.ToLower(err.Error()), "no such host") ||
			strings.Contains(strings.ToLower(err.Error()), "request failed"),
			"Expected DNS or connection error, got: %v", err)
	})
}
