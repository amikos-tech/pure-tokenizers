package tokenizers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

const (
	HFDefaultRevision = "main"
	HFDefaultTimeout  = 30 * time.Second
	HFMaxRetries      = 3
	HFRetryDelay      = time.Second
	// HFMaxRetryAfterDelay caps the maximum delay from Retry-After headers
	// to prevent excessive waits from misconfigured or malicious servers
	HFMaxRetryAfterDelay = 5 * time.Minute

	// HTTP connection pooling defaults
	defaultMaxIdleConns        = 100
	defaultMaxIdleConnsPerHost = 10
	defaultIdleTimeout         = 90 * time.Second

	// HTTP connection pooling maximum bounds
	// These limits prevent resource exhaustion from misconfiguration
	maxAllowedIdleConns        = 1000 // Maximum total idle connections across all hosts
	maxAllowedIdleConnsPerHost = 100  // Maximum idle connections per individual host
)

var (
	HFHubBaseURL   = "https://huggingface.co" // Variable to allow testing with mock server
	libraryVersion = "0.1.0"                  // Default version, will be set from library if available

	// Shared HTTP client for HuggingFace downloads with connection pooling
	hfHTTPClient *http.Client
	hfClientOnce sync.Once
)

// GetLibraryVersion returns the current library version used in User-Agent
func GetLibraryVersion() string {
	return libraryVersion
}

// SetLibraryVersion sets the library version for User-Agent headers
func SetLibraryVersion(version string) {
	if version != "" {
		libraryVersion = version
	}
}

// getEnvInt retrieves an integer value from environment variable
func getEnvInt(key string, defaultValue int) int {
	if envVal := os.Getenv(key); envVal != "" {
		if val, err := strconv.Atoi(envVal); err == nil && val > 0 {
			return val
		} else if err != nil {
			// Always log warning for invalid configuration to help users debug
			log.Printf("[WARNING] Invalid integer value for %s: '%s' (error: %v), using default: %d\n",
				key, envVal, err, defaultValue)
		} else if val <= 0 {
			// Log warning for non-positive values
			log.Printf("[WARNING] Non-positive value for %s: %d, using default: %d\n",
				key, val, defaultValue)
		}
	}
	return defaultValue
}

// getEnvDuration retrieves a duration value from environment variable
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if envVal := os.Getenv(key); envVal != "" {
		if val, err := time.ParseDuration(envVal); err == nil && val > 0 {
			return val
		} else if err != nil {
			// Always log warning for invalid configuration to help users debug
			log.Printf("[WARNING] Invalid duration value for %s: '%s' (error: %v), using default: %v\n",
				key, envVal, err, defaultValue)
		} else if val <= 0 {
			// Log warning for non-positive durations
			log.Printf("[WARNING] Non-positive duration for %s: %v, using default: %v\n",
				key, val, defaultValue)
		}
	}
	return defaultValue
}

// validateHTTPPoolingConfig validates and adjusts HTTP pooling configuration for logical consistency
func validateHTTPPoolingConfig(maxIdleConns, maxIdleConnsPerHost int) (int, int) {
	originalMaxIdleConns := maxIdleConns
	originalMaxIdleConnsPerHost := maxIdleConnsPerHost

	// Ensure maxIdleConns is at least as large as maxIdleConnsPerHost
	// This is logical since total idle connections should be >= per-host idle connections
	if maxIdleConns < maxIdleConnsPerHost {
		maxIdleConns = maxIdleConnsPerHost
		// Always log this important logical adjustment
		log.Printf("[WARNING] HTTPMaxIdleConns (%d) was less than HTTPMaxIdleConnsPerHost (%d), adjusted to %d for consistency",
			originalMaxIdleConns, maxIdleConnsPerHost, maxIdleConns)
	}

	// Ensure reasonable bounds to prevent resource exhaustion
	if maxIdleConns > maxAllowedIdleConns {
		maxIdleConns = maxAllowedIdleConns
		log.Printf("[WARNING] HTTPMaxIdleConns (%d) exceeds maximum allowed (%d), capped to prevent resource exhaustion",
			originalMaxIdleConns, maxAllowedIdleConns)
	}
	if maxIdleConnsPerHost > maxAllowedIdleConnsPerHost {
		maxIdleConnsPerHost = maxAllowedIdleConnsPerHost
		log.Printf("[WARNING] HTTPMaxIdleConnsPerHost (%d) exceeds maximum allowed (%d), capped to prevent resource exhaustion",
			originalMaxIdleConnsPerHost, maxAllowedIdleConnsPerHost)
	}

	return maxIdleConns, maxIdleConnsPerHost
}

// initHFHTTPClient initializes the shared HTTP client with connection pooling.
// NOTE: Due to thread-safety via sync.Once, configuration changes after the first
// client initialization will not take effect. The HTTP client is initialized once
// per process lifetime.
func initHFHTTPClient(config *HFConfig) {
	hfClientOnce.Do(func() {
		// Apply configuration with priority: config fields > env vars > defaults
		maxIdleConns := config.HTTPMaxIdleConns
		if maxIdleConns == 0 {
			maxIdleConns = getEnvInt("HF_HTTP_MAX_IDLE_CONNS", defaultMaxIdleConns)
		}

		maxIdleConnsPerHost := config.HTTPMaxIdleConnsPerHost
		if maxIdleConnsPerHost == 0 {
			maxIdleConnsPerHost = getEnvInt("HF_HTTP_MAX_IDLE_CONNS_PER_HOST", defaultMaxIdleConnsPerHost)
		}

		// Store original values for logging
		originalMaxIdleConns := maxIdleConns
		originalMaxIdleConnsPerHost := maxIdleConnsPerHost

		// Validate and adjust configuration for logical consistency
		maxIdleConns, maxIdleConnsPerHost = validateHTTPPoolingConfig(maxIdleConns, maxIdleConnsPerHost)

		idleTimeout := config.HTTPIdleTimeout
		if idleTimeout == 0 {
			idleTimeout = getEnvDuration("HF_HTTP_IDLE_TIMEOUT", defaultIdleTimeout)
		}

		// Log final configuration in debug mode
		if os.Getenv("DEBUG") != "" {
			log.Printf("[DEBUG] HTTP Client Configuration:\n")
			log.Printf("  MaxIdleConns: %d", maxIdleConns)
			if originalMaxIdleConns != maxIdleConns {
				log.Printf("    (adjusted from %d for consistency)", originalMaxIdleConns)
			}
			log.Printf("  MaxIdleConnsPerHost: %d", maxIdleConnsPerHost)
			if originalMaxIdleConnsPerHost != maxIdleConnsPerHost {
				log.Printf("    (adjusted from %d due to bounds)", originalMaxIdleConnsPerHost)
			}
			log.Printf("  IdleTimeout: %v", idleTimeout)
		}

		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:   true,
			MaxIdleConns:        maxIdleConns,
			MaxIdleConnsPerHost: maxIdleConnsPerHost,
			// IdleConnTimeout is suitable for long-running processes that may
			// have periods of inactivity between downloads. For short scripts that
			// exit quickly, connections will be closed automatically on program exit.
			IdleConnTimeout:       idleTimeout,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}

		hfHTTPClient = &http.Client{
			Transport: transport,
			// Default timeout - will be overridden per-request using context
			Timeout: 0,
		}
	})
}

// getHFHTTPClient returns the shared HTTP client for HuggingFace downloads
func getHFHTTPClient(config *HFConfig) *http.Client {
	initHFHTTPClient(config)
	return hfHTTPClient
}

// HFConfig holds HuggingFace-specific configuration
type HFConfig struct {
	Token       string
	Revision    string
	CacheDir    string
	Timeout     time.Duration
	MaxRetries  int
	OfflineMode bool

	// HTTP client pooling configuration
	// These settings control connection reuse for improved performance.
	// Config fields take priority over environment variables.
	//
	// IMPORTANT: The HTTP client is initialized once per process using sync.Once.
	// Changes to these configuration values after the first HuggingFace download
	// will NOT take effect. Set these values before any HuggingFace operations.
	//
	// Performance trade-offs:
	// - Higher values: Better connection reuse, reduced latency for subsequent requests, but increased memory usage
	// - Lower values: Reduced memory footprint, but more connection establishment overhead
	//
	// Recommended configurations:
	// - High-throughput services: Increase HTTPMaxIdleConnsPerHost (e.g., 20-50) for parallel downloads
	// - Resource-constrained environments: Reduce both values (e.g., 50/5) to minimize memory usage
	// - Short-lived scripts: Reduce HTTPIdleTimeout (e.g., 10s) to release resources quickly
	//
	// Note: HTTPMaxIdleConns will be automatically adjusted to be >= HTTPMaxIdleConnsPerHost for logical consistency
	//
	// Debug mode: Set DEBUG=1 environment variable to see actual configuration values being used
	HTTPMaxIdleConns        int           // Maximum idle connections across all hosts (env: HF_HTTP_MAX_IDLE_CONNS, default: 100, max: 1000)
	HTTPMaxIdleConnsPerHost int           // Maximum idle connections per host (env: HF_HTTP_MAX_IDLE_CONNS_PER_HOST, default: 10, max: 100)
	HTTPIdleTimeout         time.Duration // How long to keep idle connections open (env: HF_HTTP_IDLE_TIMEOUT, default: 90s)
}

// FromHuggingFace loads a tokenizer from HuggingFace Hub using the model identifier.
//
// The model identifier can be in the format "organization/model" or just "model".
// For example: "bert-base-uncased", "google/flan-t5-base", "meta-llama/Llama-2-7b-hf".
//
// By default, it loads from the "main" branch/revision. Use WithHFRevision to specify
// a different revision (branch, tag, or commit hash).
//
// For private or gated models, authentication is required. Set the HF_TOKEN environment
// variable or use WithHFToken option.
//
// The tokenizer is cached locally for faster subsequent loads. The cache location is
// platform-specific and can be overridden with WithHFCacheDir.
//
// Example:
//
//	tokenizer, err := FromHuggingFace("bert-base-uncased")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer tokenizer.Close()
func FromHuggingFace(modelID string, opts ...TokenizerOption) (*Tokenizer, error) {
	if modelID == "" {
		return nil, errors.New("model ID cannot be empty")
	}

	// Validate model ID format
	if err := validateModelID(modelID); err != nil {
		return nil, errors.Wrapf(err, "invalid model ID: %s", modelID)
	}

	// Create tokenizer with HF config
	tokenizer := &Tokenizer{
		defaultEncodingOpts: EncodeOptions{
			ReturnTokens: true,
		},
		hfConfig: &HFConfig{
			Revision:   HFDefaultRevision,
			Timeout:    HFDefaultTimeout,
			MaxRetries: HFMaxRetries,
		},
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(tokenizer); err != nil {
			return nil, errors.Wrapf(err, "failed to apply tokenizer option")
		}
	}

	// Get token from environment if not provided
	if tokenizer.hfConfig.Token == "" {
		tokenizer.hfConfig.Token = os.Getenv("HF_TOKEN")
	}

	// Try to load from cache first
	cachedPath := getHFCachePath(tokenizer.hfConfig.CacheDir, modelID, tokenizer.hfConfig.Revision)
	if tokenizer.hfConfig.OfflineMode || fileExists(cachedPath) {
		data, err := os.ReadFile(cachedPath)
		if err == nil {
			return FromBytes(data, opts...)
		}
		if tokenizer.hfConfig.OfflineMode {
			return nil, errors.Wrap(err, "offline mode enabled but tokenizer not found in cache")
		}
		// Continue to download if cache read failed
	}

	// Download tokenizer.json from HuggingFace
	data, err := downloadTokenizerFromHF(modelID, tokenizer.hfConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to download tokenizer from HuggingFace")
	}

	// Save to cache
	if err := saveToHFCache(cachedPath, data); err != nil {
		// Log warning but don't fail
		// In production, you might want to use a proper logger
		_ = err
	}

	// Create tokenizer from downloaded data
	return FromBytes(data, opts...)
}

// WithHFToken sets the HuggingFace API token for authentication
func WithHFToken(token string) TokenizerOption {
	return func(t *Tokenizer) error {
		if t.hfConfig == nil {
			t.hfConfig = &HFConfig{}
		}
		t.hfConfig.Token = token
		return nil
	}
}

// WithHFRevision sets the model revision (branch, tag, or commit hash)
func WithHFRevision(revision string) TokenizerOption {
	return func(t *Tokenizer) error {
		if t.hfConfig == nil {
			t.hfConfig = &HFConfig{}
		}
		t.hfConfig.Revision = revision
		return nil
	}
}

// WithHFCacheDir sets a custom cache directory for HuggingFace tokenizers
func WithHFCacheDir(dir string) TokenizerOption {
	return func(t *Tokenizer) error {
		if t.hfConfig == nil {
			t.hfConfig = &HFConfig{}
		}
		t.hfConfig.CacheDir = dir
		return nil
	}
}

// WithHFTimeout sets the download timeout for HuggingFace requests
func WithHFTimeout(timeout time.Duration) TokenizerOption {
	return func(t *Tokenizer) error {
		if timeout <= 0 {
			return errors.New("timeout must be positive")
		}
		if t.hfConfig == nil {
			t.hfConfig = &HFConfig{}
		}
		t.hfConfig.Timeout = timeout
		return nil
	}
}

// WithHFOfflineMode forces the tokenizer to only use cached versions
func WithHFOfflineMode(offline bool) TokenizerOption {
	return func(t *Tokenizer) error {
		if t.hfConfig == nil {
			t.hfConfig = &HFConfig{}
		}
		t.hfConfig.OfflineMode = offline
		return nil
	}
}

// downloadTokenizerFromHF downloads the tokenizer.json file from HuggingFace Hub
func downloadTokenizerFromHF(modelID string, config *HFConfig) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/resolve/%s/tokenizer.json", HFHubBaseURL, modelID, config.Revision)

	var lastErr error
	var retryAfterDuration time.Duration
	for attempt := 0; attempt < config.MaxRetries; attempt++ {
		if attempt > 0 {
			var delay time.Duration

			// Use server-suggested delay if available
			if retryAfterDuration > 0 {
				delay = retryAfterDuration
				// Reset for next iteration
				retryAfterDuration = 0
			} else {
				// Exponential backoff with jitter
				baseDelay := HFRetryDelay * time.Duration(1<<uint(attempt-1))
				// Add 0-25% jitter to prevent thundering herd
				jitter := time.Duration(rand.Float64() * 0.25 * float64(baseDelay))
				delay = baseDelay + jitter
			}

			time.Sleep(delay)
		}

		data, resp, err := downloadWithRetryAndResponse(url, config)
		if err == nil {
			return data, nil
		}

		lastErr = err

		// Parse Retry-After header if present
		if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				retryAfterDuration = parseRetryAfter(retryAfter)
			}
		}

		// Don't retry on certain errors
		if isNonRetryableError(err) {
			break
		}
	}

	return nil, lastErr
}

// downloadWithRetryAndResponse performs a single download attempt and returns the response.
// Unlike a simple download function, this returns the HTTP response alongside the data
// to allow the caller to inspect response headers (e.g., Retry-After header for rate limiting).
func downloadWithRetryAndResponse(url string, config *HFConfig) ([]byte, *http.Response, error) {
	// Create a context with timeout for this specific request
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create request")
	}

	// Set headers
	req.Header.Set("User-Agent", fmt.Sprintf("pure-tokenizers/%s", GetLibraryVersion()))
	if config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+config.Token)
	}

	// Use the shared HTTP client with connection pooling
	client := getHFHTTPClient(config)
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, errors.Wrap(err, "request failed")
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check status code
	switch resp.StatusCode {
	case http.StatusOK:
		// Success
	case http.StatusUnauthorized:
		return nil, resp, errors.New("authentication required: please set HF_TOKEN environment variable or use WithHFToken()")
	case http.StatusForbidden:
		return nil, resp, errors.New("access forbidden: token may be invalid or model may be gated")
	case http.StatusNotFound:
		return nil, resp, errors.New("model or tokenizer.json not found")
	case http.StatusTooManyRequests:
		// Return with response so caller can parse Retry-After
		return nil, resp, errors.New("rate limited: too many requests")
	default:
		return nil, resp, errors.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp, errors.Wrap(err, "failed to read response")
	}

	// Validate it's valid JSON
	var validateJSON map[string]interface{}
	if err := json.Unmarshal(data, &validateJSON); err != nil {
		return nil, resp, errors.Wrap(err, "invalid tokenizer.json format")
	}

	return data, resp, nil
}

// validateModelID checks if the model ID is valid
func validateModelID(modelID string) error {
	// Empty model ID is handled separately in FromHuggingFace
	if modelID == "" {
		return nil
	}

	// Validate format: must be either "repo_name" or "owner/repo_name"
	parts := strings.Split(modelID, "/")
	if len(parts) > 2 {
		return errors.New("model ID must be in format 'owner/repo_name' or just 'repo_name'")
	}

	// If format is owner/repo_name, validate owner
	if len(parts) == 2 {
		owner := parts[0]
		if owner == "" {
			return errors.New("owner cannot be empty")
		}
		// Owner follows same rules as repo_name
		if len(owner) > 96 {
			return errors.New("owner cannot exceed 96 characters")
		}
		if !isValidRepoName(owner) {
			return errors.New("owner contains invalid characters (must match [\\w\\-.]{1,96})")
		}
	}

	// Validate repo_name (last part)
	repoName := parts[len(parts)-1]
	if repoName == "" {
		return errors.New("repo_name cannot be empty")
	}
	if len(repoName) > 96 {
		return errors.Errorf("repo_name cannot exceed 96 characters (got %d)", len(repoName))
	}
	if !isValidRepoName(repoName) {
		return errors.New("repo_name contains invalid characters (must match [\\w\\-.]{1,96})")
	}

	return nil
}

// isValidRepoName checks if a repo/owner name matches HuggingFace's pattern [\w\-.]{1,96}
func isValidRepoName(name string) bool {
	if len(name) == 0 || len(name) > 96 {
		return false
	}
	for _, c := range name {
		// \w matches [a-zA-Z0-9_], plus we allow - and .
		if !((c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '_' || c == '-' || c == '.') {
			return false
		}
	}
	return true
}

// getHFCachePath returns the cache path for a HuggingFace tokenizer
func getHFCachePath(customCacheDir, modelID, revision string) string {
	var cacheDir string
	if customCacheDir != "" {
		cacheDir = customCacheDir
	} else {
		cacheDir = getHFCacheDir()
	}

	// Sanitize model ID for filesystem
	sanitizedModelID := strings.ReplaceAll(modelID, "/", "--")

	return filepath.Join(cacheDir, "models", sanitizedModelID, revision, "tokenizer.json")
}

// getHFCacheDir returns the default HuggingFace cache directory
func getHFCacheDir() string {
	// Check HF environment variables first
	if hfHome := os.Getenv("HF_HOME"); hfHome != "" {
		return filepath.Join(hfHome, "tokenizers")
	}
	if hfCache := os.Getenv("HF_HUB_CACHE"); hfCache != "" {
		return filepath.Join(hfCache, "..", "tokenizers")
	}

	// Use our standard cache directory with HF subdirectory
	baseCache := getCacheDir()
	return filepath.Join(baseCache, "hf")
}

// saveToHFCache saves the tokenizer data to the cache
func saveToHFCache(path string, data []byte) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.Wrap(err, "failed to create cache directory")
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return errors.Wrap(err, "failed to write cache file")
	}

	return nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// isNonRetryableError checks if an error should not be retried
func isNonRetryableError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "authentication required") ||
		strings.Contains(errStr, "access forbidden") ||
		strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "invalid")
}

// GetHFCacheInfo returns information about the HuggingFace cache for a model
func GetHFCacheInfo(modelID string) (map[string]interface{}, error) {
	info := make(map[string]interface{})
	info["model_id"] = modelID

	// Check default cache
	defaultPath := getHFCachePath("", modelID, HFDefaultRevision)
	info["default_cache_path"] = defaultPath
	info["is_cached"] = fileExists(defaultPath)

	if fileExists(defaultPath) {
		if stat, err := os.Stat(defaultPath); err == nil {
			info["cache_size"] = stat.Size()
			info["cache_modified"] = stat.ModTime()
		}
	}

	return info, nil
}

// ClearHFModelCache clears the cache for a specific model
func ClearHFModelCache(modelID string) error {
	cacheDir := getHFCacheDir()
	sanitizedModelID := strings.ReplaceAll(modelID, "/", "--")
	modelCacheDir := filepath.Join(cacheDir, "models", sanitizedModelID)

	if _, err := os.Stat(modelCacheDir); os.IsNotExist(err) {
		return nil // Already doesn't exist
	}

	return os.RemoveAll(modelCacheDir)
}

// ClearHFCache clears all HuggingFace tokenizer cache
func ClearHFCache() error {
	cacheDir := getHFCacheDir()
	modelsDir := filepath.Join(cacheDir, "models")

	if _, err := os.Stat(modelsDir); os.IsNotExist(err) {
		return nil // Already doesn't exist
	}

	return os.RemoveAll(modelsDir)
}

// parseRetryAfter parses the Retry-After header value.
// It can be either a delay in seconds or an HTTP date.
// The returned duration is capped at HFMaxRetryAfterDelay to prevent excessive waits.
func parseRetryAfter(value string) time.Duration {
	var duration time.Duration

	// First, try to parse as seconds
	if seconds, err := strconv.Atoi(value); err == nil {
		duration = time.Duration(seconds) * time.Second
	} else if t, err := http.ParseTime(value); err == nil {
		// Try to parse as HTTP date (RFC1123)
		// Calculate duration from now
		duration = time.Until(t)
		if duration < 0 {
			// If the time is in the past, don't wait
			return 0
		}
	} else {
		// If we can't parse it, return 0 (fallback to exponential backoff)
		return 0
	}

	// Cap the delay to prevent excessive waits
	if duration > HFMaxRetryAfterDelay {
		return HFMaxRetryAfterDelay
	}
	return duration
}
