package tokenizers

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
)

var (
	HFHubBaseURL      = "https://huggingface.co"
	HFDefaultRevision = "main"
	HFDefaultTimeout  = 30 * time.Second
	HFMaxRetries      = 3
	libraryVersion    = "0.1.0" // Default version, will be set from library if available
	HFRetryDelay      = time.Second
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

// HFConfig holds HuggingFace-specific configuration
type HFConfig struct {
	Token       string
	Revision    string
	CacheDir    string
	Timeout     time.Duration
	MaxRetries  int
	OfflineMode bool
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
	for attempt := 0; attempt < config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with jitter
			baseDelay := HFRetryDelay * time.Duration(1<<uint(attempt-1))
			// Add 0-25% jitter to prevent thundering herd
			jitter := time.Duration(rand.Float64() * 0.25 * float64(baseDelay))
			time.Sleep(baseDelay + jitter)
		}

		data, err := downloadWithRetry(url, config)
		if err == nil {
			return data, nil
		}

		lastErr = err

		// Don't retry on certain errors
		if isNonRetryableError(err) {
			break
		}
	}

	return nil, lastErr
}

// downloadWithRetry performs a single download attempt
func downloadWithRetry(url string, config *HFConfig) ([]byte, error) {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	// Set headers
	req.Header.Set("User-Agent", fmt.Sprintf("pure-tokenizers/%s", GetLibraryVersion()))
	if config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+config.Token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "request failed")
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check status code
	switch resp.StatusCode {
	case http.StatusOK:
		// Success
	case http.StatusUnauthorized:
		return nil, errors.New("authentication required: please set HF_TOKEN environment variable or use WithHFToken()")
	case http.StatusForbidden:
		return nil, errors.New("access forbidden: token may be invalid or model may be gated")
	case http.StatusNotFound:
		return nil, errors.New("model or tokenizer.json not found")
	case http.StatusTooManyRequests:
		// Parse retry-after header if available
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			return nil, errors.Errorf("rate limited: retry after %s seconds", retryAfter)
		}
		return nil, errors.New("rate limited: too many requests")
	default:
		return nil, errors.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response")
	}

	// Validate it's valid JSON
	var validateJSON map[string]interface{}
	if err := json.Unmarshal(data, &validateJSON); err != nil {
		return nil, errors.Wrap(err, "invalid tokenizer.json format")
	}

	return data, nil
}

// validateModelID checks if the model ID is valid
func validateModelID(modelID string) error {
	// Empty model ID is handled separately in FromHuggingFace
	if modelID == "" {
		return nil
	}

	// Check length limit
	if len(modelID) > 256 {
		return errors.New("model ID cannot exceed 256 characters")
	}

	// Basic validation - can be enhanced
	if strings.Contains(modelID, "..") {
		return errors.New("model ID cannot contain '..'")
	}
	if strings.HasPrefix(modelID, "/") || strings.HasSuffix(modelID, "/") {
		return errors.New("model ID cannot start or end with '/'")
	}

	// Validate organization/model format
	parts := strings.Split(modelID, "/")
	if len(parts) > 2 {
		return errors.New("model ID should be in format 'organization/model' or just 'model'")
	}
	for _, part := range parts {
		if part == "" {
			return errors.New("model ID parts cannot be empty")
		}
		if len(part) > 128 {
			return errors.New("each part of model ID cannot exceed 128 characters")
		}
	}

	// Check for valid characters (alphanumeric, dash, underscore, slash)
	for _, char := range modelID {
		if !isValidModelIDChar(char) {
			return errors.Errorf("invalid character in model ID: %c", char)
		}
	}
	return nil
}

// isValidModelIDChar checks if a character is valid in a model ID
func isValidModelIDChar(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '/' || c == '.'
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