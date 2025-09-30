package tokenizers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const (
	concurrentAccessSameModel   = 10
	concurrentAccessDiffModels  = 3
	concurrentReaders           = 15
	concurrentWriters           = 5
	concurrentValidations       = 10
	concurrentErrorBufferMargin = 5
)

func TestCheckHFHubCache(t *testing.T) {
	// Create a temporary HF hub cache structure
	tmpDir := t.TempDir()
	_ = os.Setenv("HF_HUB_CACHE", tmpDir)
	defer func() { _ = os.Unsetenv("HF_HUB_CACHE") }()

	// Create mock cache structure
	modelID := "test-org/test-model"
	sanitizedID := "models--test-org--test-model"
	snapshotHash := "abc123def456"

	// Create directories
	snapshotDir := filepath.Join(tmpDir, sanitizedID, "snapshots", snapshotHash)
	err := os.MkdirAll(snapshotDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create snapshot dir: %v", err)
	}

	refsDir := filepath.Join(tmpDir, sanitizedID, "refs")
	err = os.MkdirAll(refsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create refs dir: %v", err)
	}

	// Create a mock tokenizer.json
	mockTokenizer := map[string]interface{}{
		"version": "1.0",
		"model":   map[string]interface{}{"type": "BPE"},
	}
	tokenizerData, _ := json.Marshal(mockTokenizer)
	tokenizerPath := filepath.Join(snapshotDir, "tokenizer.json")
	err = os.WriteFile(tokenizerPath, tokenizerData, 0644)
	if err != nil {
		t.Fatalf("Failed to write tokenizer.json: %v", err)
	}

	// Create ref for main branch
	refPath := filepath.Join(refsDir, "main")
	err = os.WriteFile(refPath, []byte(snapshotHash), 0644)
	if err != nil {
		t.Fatalf("Failed to write ref: %v", err)
	}

	// Test successful cache lookup
	data, err := checkHFHubCache(modelID, "main")
	if err != nil {
		t.Errorf("Expected successful cache lookup, got error: %v", err)
	}
	if data == nil {
		t.Error("Expected non-nil data from cache")
	}

	// Test with non-existent model
	_, err = checkHFHubCache("non-existent/model", "main")
	if err == nil {
		t.Error("Expected error for non-existent model, got nil")
	}
}

func TestLoadFromCacheWithValidation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid tokenizer file
	mockTokenizer := map[string]interface{}{
		"version": "1.0",
		"model":   map[string]interface{}{"type": "BPE"},
	}
	tokenizerData, _ := json.Marshal(mockTokenizer)
	cachePath := filepath.Join(tmpDir, "tokenizer.json")
	err := os.WriteFile(cachePath, tokenizerData, 0644)
	if err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	// Test loading without TTL
	data, err := loadFromCacheWithValidation(cachePath, 0)
	if err != nil {
		t.Errorf("Expected successful load without TTL, got error: %v", err)
	}
	if data == nil {
		t.Error("Expected non-nil data from cache")
	}

	// Test with valid TTL (file is fresh)
	data, err = loadFromCacheWithValidation(cachePath, 1*time.Hour)
	if err != nil {
		t.Errorf("Expected successful load with valid TTL, got error: %v", err)
	}
	if data == nil {
		t.Error("Expected non-nil data from cache")
	}

	// Test with expired TTL
	// Modify the file's modtime to be in the past
	oldTime := time.Now().Add(-2 * time.Hour)
	_ = os.Chtimes(cachePath, oldTime, oldTime)

	_, err = loadFromCacheWithValidation(cachePath, 1*time.Hour)
	if err == nil {
		t.Error("Expected error for expired cache, got nil")
	}

	// Test with non-existent file
	_, err = loadFromCacheWithValidation(filepath.Join(tmpDir, "non-existent.json"), 0)
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}

	// Test with invalid JSON
	invalidPath := filepath.Join(tmpDir, "invalid.json")
	_ = os.WriteFile(invalidPath, []byte("not json"), 0644)
	_, err = loadFromCacheWithValidation(invalidPath, 0)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestGetHFHubCacheDir(t *testing.T) {
	// Save original env vars
	originalHFCache := os.Getenv("HF_HUB_CACHE")
	originalHFHome := os.Getenv("HF_HOME")
	defer func() {
		if originalHFCache != "" {
			_ = os.Setenv("HF_HUB_CACHE", originalHFCache)
		} else {
			_ = os.Unsetenv("HF_HUB_CACHE")
		}
		if originalHFHome != "" {
			_ = os.Setenv("HF_HOME", originalHFHome)
		} else {
			_ = os.Unsetenv("HF_HOME")
		}
	}()

	// Test with HF_HUB_CACHE set
	_ = os.Setenv("HF_HUB_CACHE", "/custom/hub/cache")
	_ = os.Unsetenv("HF_HOME")
	dir := getHFHubCacheDir()
	if dir != "/custom/hub/cache" {
		t.Errorf("Expected /custom/hub/cache, got %s", dir)
	}

	// Test with HF_HOME set
	_ = os.Unsetenv("HF_HUB_CACHE")
	_ = os.Setenv("HF_HOME", "/custom/hf/home")
	dir = getHFHubCacheDir()
	expectedDir := filepath.Join("/custom/hf/home", "hub")
	if dir != expectedDir {
		t.Errorf("Expected %s, got %s", expectedDir, dir)
	}

	// Test with neither set (should use default)
	_ = os.Unsetenv("HF_HUB_CACHE")
	_ = os.Unsetenv("HF_HOME")
	dir = getHFHubCacheDir()
	if dir == "" {
		t.Error("Expected non-empty default cache dir")
	}
	// Should contain .cache/huggingface/hub
	if !filepath.IsAbs(dir) {
		t.Errorf("Expected absolute path, got %s", dir)
	}
}

func TestWithHFUseLocalCache(t *testing.T) {
	tokenizer := &Tokenizer{}

	// Test enabling cache
	opt := WithHFUseLocalCache(true)
	err := opt(tokenizer)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if tokenizer.hfConfig == nil || !tokenizer.hfConfig.UseLocalCache {
		t.Error("Expected UseLocalCache to be true")
	}

	// Test disabling cache
	opt = WithHFUseLocalCache(false)
	err = opt(tokenizer)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if tokenizer.hfConfig.UseLocalCache {
		t.Error("Expected UseLocalCache to be false")
	}
}

func TestWithHFCacheTTL(t *testing.T) {
	tokenizer := &Tokenizer{}

	// Test with valid TTL
	ttl := 24 * time.Hour
	opt := WithHFCacheTTL(ttl)
	err := opt(tokenizer)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if tokenizer.hfConfig == nil || tokenizer.hfConfig.CacheTTL != ttl {
		t.Errorf("Expected CacheTTL to be %v, got %v", ttl, tokenizer.hfConfig.CacheTTL)
	}

	// Test with zero TTL (cache forever)
	opt = WithHFCacheTTL(0)
	err = opt(tokenizer)
	if err != nil {
		t.Errorf("Expected no error for zero TTL, got %v", err)
	}

	// Test with negative TTL (should error)
	opt = WithHFCacheTTL(-1 * time.Hour)
	err = opt(tokenizer)
	if err == nil {
		t.Error("Expected error for negative TTL, got nil")
	}
}

func TestDualCacheIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test simulates the complete dual cache flow
	tmpDir := t.TempDir()

	// Set up our cache directory
	ourCacheDir := filepath.Join(tmpDir, "our-cache")
	_ = os.MkdirAll(ourCacheDir, 0755)

	// Set up HF hub cache directory
	hfCacheDir := filepath.Join(tmpDir, "hf-cache")
	_ = os.Setenv("HF_HUB_CACHE", hfCacheDir)
	defer func() { _ = os.Unsetenv("HF_HUB_CACHE") }()

	// Create a tokenizer in HF hub cache
	modelID := "test/model"
	sanitizedID := "models--test--model"
	snapshotHash := "snapshot123"

	snapshotDir := filepath.Join(hfCacheDir, sanitizedID, "snapshots", snapshotHash)
	_ = os.MkdirAll(snapshotDir, 0755)

	// Create mock tokenizer
	mockTokenizer := map[string]interface{}{
		"version": "1.0",
		"model":   map[string]interface{}{"type": "BPE"},
		"from":    "hf-hub-cache",
	}
	tokenizerData, _ := json.Marshal(mockTokenizer)
	_ = os.WriteFile(filepath.Join(snapshotDir, "tokenizer.json"), tokenizerData, 0644)

	// Create ref
	refsDir := filepath.Join(hfCacheDir, sanitizedID, "refs")
	_ = os.MkdirAll(refsDir, 0755)
	_ = os.WriteFile(filepath.Join(refsDir, "main"), []byte(snapshotHash), 0644)

	// Test that checkHFHubCache finds the tokenizer
	data, err := checkHFHubCache(modelID, "main")
	if err != nil {
		t.Fatalf("Failed to find tokenizer in HF hub cache: %v", err)
	}

	// Verify the data contains our marker
	var loaded map[string]interface{}
	_ = json.Unmarshal(data, &loaded)
	if loaded["from"] != "hf-hub-cache" {
		t.Error("Expected tokenizer from HF hub cache")
	}
}

// TestConcurrentCacheAccessSameModel verifies that multiple goroutines can
// safely read from the cache when accessing the same model simultaneously.
// This tests the idempotent nature of cache reads and validates that no
// race conditions occur during concurrent file system reads.
func TestConcurrentCacheAccessSameModel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	hfCacheDir := filepath.Join(tmpDir, "hf-cache")
	_ = os.Setenv("HF_HUB_CACHE", hfCacheDir)
	t.Cleanup(func() { _ = os.Unsetenv("HF_HUB_CACHE") })

	// Create a mock HF hub cache with tokenizer
	modelID := "test/concurrent-model"
	sanitizedID := "models--test--concurrent-model"
	snapshotHash := "snapshot456"

	snapshotDir := filepath.Join(hfCacheDir, sanitizedID, "snapshots", snapshotHash)
	_ = os.MkdirAll(snapshotDir, 0755)

	mockTokenizer := map[string]interface{}{
		"version": "1.0",
		"model":   map[string]interface{}{"type": "BPE"},
	}
	tokenizerData, _ := json.Marshal(mockTokenizer)
	_ = os.WriteFile(filepath.Join(snapshotDir, "tokenizer.json"), tokenizerData, 0644)

	refsDir := filepath.Join(hfCacheDir, sanitizedID, "refs")
	_ = os.MkdirAll(refsDir, 0755)
	_ = os.WriteFile(filepath.Join(refsDir, "main"), []byte(snapshotHash), 0644)

	var wg sync.WaitGroup
	errorsChan := make(chan error, concurrentAccessSameModel+concurrentErrorBufferMargin)
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < concurrentAccessSameModel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := checkHFHubCache(modelID, "main")
			if err != nil {
				errorsChan <- err
				return
			}
			if data == nil {
				errorsChan <- nil
				return
			}
			mu.Lock()
			successCount++
			mu.Unlock()
		}()
	}

	wg.Wait()
	close(errorsChan)

	var errors []error
	for err := range errorsChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		t.Errorf("Encountered %d errors during concurrent access:", len(errors))
		for i, err := range errors {
			if err != nil {
				t.Errorf("  [%d] %v", i+1, err)
			} else {
				t.Errorf("  [%d] returned nil data", i+1)
			}
		}
	}

	if successCount != concurrentAccessSameModel {
		t.Errorf("Expected %d successful accesses, got %d", concurrentAccessSameModel, successCount)
	}
}

// TestConcurrentCacheAccessDifferentModels verifies that multiple goroutines
// can safely access different models from the cache simultaneously without
// interfering with each other. This validates that the cache lookup mechanism
// correctly isolates different model accesses and prevents cross-contamination.
func TestConcurrentCacheAccessDifferentModels(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	hfCacheDir := filepath.Join(tmpDir, "hf-cache")
	_ = os.Setenv("HF_HUB_CACHE", hfCacheDir)
	t.Cleanup(func() { _ = os.Unsetenv("HF_HUB_CACHE") })

	// Create multiple model caches
	models := []string{"test/model-1", "test/model-2", "test/model-3"}
	for _, modelID := range models {
		sanitizedID := "models--test--" + modelID[5:]
		snapshotHash := "snapshot-" + modelID[5:]

		snapshotDir := filepath.Join(hfCacheDir, sanitizedID, "snapshots", snapshotHash)
		_ = os.MkdirAll(snapshotDir, 0755)

		mockTokenizer := map[string]interface{}{
			"version": "1.0",
			"model":   map[string]interface{}{"type": "BPE"},
			"id":      modelID,
		}
		tokenizerData, _ := json.Marshal(mockTokenizer)
		_ = os.WriteFile(filepath.Join(snapshotDir, "tokenizer.json"), tokenizerData, 0644)

		refsDir := filepath.Join(hfCacheDir, sanitizedID, "refs")
		_ = os.MkdirAll(refsDir, 0755)
		_ = os.WriteFile(filepath.Join(refsDir, "main"), []byte(snapshotHash), 0644)
	}

	var wg sync.WaitGroup
	totalOps := len(models) * concurrentAccessDiffModels
	errorsChan := make(chan error, totalOps+concurrentErrorBufferMargin)
	results := make(map[string]int)
	var mu sync.Mutex

	for _, modelID := range models {
		for j := 0; j < concurrentAccessDiffModels; j++ {
			wg.Add(1)
			model := modelID
			go func() {
				defer wg.Done()
				data, err := checkHFHubCache(model, "main")
				if err != nil {
					errorsChan <- err
					return
				}
				if data == nil {
					errorsChan <- nil
					return
				}

				var loaded map[string]interface{}
				_ = json.Unmarshal(data, &loaded)
				mu.Lock()
				results[model]++
				mu.Unlock()
			}()
		}
	}

	wg.Wait()
	close(errorsChan)

	var errors []error
	for err := range errorsChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		t.Errorf("Encountered %d errors during concurrent access:", len(errors))
		for i, err := range errors {
			if err != nil {
				t.Errorf("  [%d] %v", i+1, err)
			} else {
				t.Errorf("  [%d] returned nil data", i+1)
			}
		}
	}

	for _, modelID := range models {
		if count := results[modelID]; count != concurrentAccessDiffModels {
			t.Errorf("Model %s: expected %d successful accesses, got %d", modelID, concurrentAccessDiffModels, count)
		}
	}
}

// TestConcurrentCacheReadWrite verifies that concurrent read and write
// operations on cache files don't cause race conditions or data corruption.
// This simulates real-world scenarios where cache metadata (modification times)
// may be updated while other processes are reading the cache.
func TestConcurrentCacheReadWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	tmpDir := t.TempDir()

	mockTokenizer := map[string]interface{}{
		"version": "1.0",
		"model":   map[string]interface{}{"type": "BPE"},
	}
	tokenizerData, _ := json.Marshal(mockTokenizer)
	cachePath := filepath.Join(tmpDir, "tokenizer.json")
	_ = os.WriteFile(cachePath, tokenizerData, 0644)

	var wg sync.WaitGroup
	totalOps := concurrentReaders + concurrentWriters
	errorsChan := make(chan error, totalOps+concurrentErrorBufferMargin)
	readCount := 0
	var mu sync.Mutex

	// Concurrent reads
	for i := 0; i < concurrentReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := loadFromCacheWithValidation(cachePath, 0)
			if err != nil {
				errorsChan <- err
				return
			}
			if data == nil {
				errorsChan <- nil
				return
			}
			mu.Lock()
			readCount++
			mu.Unlock()
		}()
	}

	// Concurrent writes (updating modtime)
	for i := 0; i < concurrentWriters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			newTime := time.Now()
			err := os.Chtimes(cachePath, newTime, newTime)
			if err != nil {
				errorsChan <- err
			}
		}()
	}

	wg.Wait()
	close(errorsChan)

	var errors []error
	for err := range errorsChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		t.Errorf("Encountered %d errors during concurrent read/write:", len(errors))
		for i, err := range errors {
			if err != nil {
				t.Errorf("  [%d] %v", i+1, err)
			} else {
				t.Errorf("  [%d] read returned nil data", i+1)
			}
		}
	}

	if readCount != concurrentReaders {
		t.Errorf("Expected %d successful reads, got %d", concurrentReaders, readCount)
	}
}

// TestConcurrentCacheValidation verifies that multiple goroutines can
// concurrently validate the same cache entry without conflicts. This ensures
// that data integrity checks (JSON parsing, schema validation) are safe to
// perform in parallel and don't introduce race conditions.
func TestConcurrentCacheValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	hfCacheDir := filepath.Join(tmpDir, "hf-cache")
	_ = os.Setenv("HF_HUB_CACHE", hfCacheDir)
	t.Cleanup(func() { _ = os.Unsetenv("HF_HUB_CACHE") })

	modelID := "test/validation-model"
	sanitizedID := "models--test--validation-model"
	snapshotHash := "snapshot789"

	snapshotDir := filepath.Join(hfCacheDir, sanitizedID, "snapshots", snapshotHash)
	_ = os.MkdirAll(snapshotDir, 0755)

	mockTokenizer := map[string]interface{}{
		"version": "1.0",
		"model":   map[string]interface{}{"type": "BPE"},
	}
	tokenizerData, _ := json.Marshal(mockTokenizer)
	tokenizerPath := filepath.Join(snapshotDir, "tokenizer.json")
	_ = os.WriteFile(tokenizerPath, tokenizerData, 0644)

	refsDir := filepath.Join(hfCacheDir, sanitizedID, "refs")
	_ = os.MkdirAll(refsDir, 0755)
	_ = os.WriteFile(filepath.Join(refsDir, "main"), []byte(snapshotHash), 0644)

	var wg sync.WaitGroup
	errorsChan := make(chan error, concurrentValidations+concurrentErrorBufferMargin)
	validCount := 0
	var mu sync.Mutex

	for i := 0; i < concurrentValidations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := checkHFHubCache(modelID, "main")
			if err != nil {
				errorsChan <- err
				return
			}
			if data == nil {
				errorsChan <- nil
				return
			}

			// Validate the data
			var loaded map[string]interface{}
			if err := json.Unmarshal(data, &loaded); err != nil {
				errorsChan <- err
				return
			}

			if loaded["version"] != "1.0" {
				errorsChan <- nil
				return
			}

			mu.Lock()
			validCount++
			mu.Unlock()
		}()
	}

	wg.Wait()
	close(errorsChan)

	var errors []error
	for err := range errorsChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		t.Errorf("Encountered %d errors during concurrent validation:", len(errors))
		for i, err := range errors {
			if err != nil {
				t.Errorf("  [%d] %v", i+1, err)
			} else {
				t.Errorf("  [%d] validation failed", i+1)
			}
		}
	}

	if validCount != concurrentValidations {
		t.Errorf("Expected %d successful validations, got %d", concurrentValidations, validCount)
	}
}

// TestConcurrentCacheCorruption verifies that the cache system handles
// corrupted data gracefully during concurrent access. This simulates scenarios
// where cache files may be partially written or corrupted, ensuring that
// errors are properly reported and don't cause panics or data races.
func TestConcurrentCacheCorruption(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a corrupted cache file (invalid JSON)
	cachePath := filepath.Join(tmpDir, "tokenizer.json")
	_ = os.WriteFile(cachePath, []byte("{ invalid json"), 0644)

	var wg sync.WaitGroup
	errorsChan := make(chan error, concurrentValidations+concurrentErrorBufferMargin)
	errorCount := 0
	var mu sync.Mutex

	for i := 0; i < concurrentValidations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := loadFromCacheWithValidation(cachePath, 0)
			if err != nil {
				mu.Lock()
				errorCount++
				mu.Unlock()
				errorsChan <- err
			}
		}()
	}

	wg.Wait()
	close(errorsChan)

	// All reads should fail with an error (not panic)
	if errorCount != concurrentValidations {
		t.Errorf("Expected %d errors from corrupted cache, got %d", concurrentValidations, errorCount)
	}

	// Verify errors were properly reported
	var errors []error
	for err := range errorsChan {
		errors = append(errors, err)
	}

	if len(errors) != concurrentValidations {
		t.Errorf("Expected %d errors in channel, got %d", concurrentValidations, len(errors))
	}
}