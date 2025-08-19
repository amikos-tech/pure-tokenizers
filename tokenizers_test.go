package tokenizers

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func checkLibraryExists(t *testing.T) string {
	var libPath string

	if os.Getenv("TOKENIZERS_LIB_PATH") != "" {
		libPath = os.Getenv("TOKENIZERS_LIB_PATH")
		t.Logf("Using library path from environment: %s", libPath)
		if _, err := os.Stat(libPath); os.IsNotExist(err) {
			t.Skipf("Skipping test because %s does not exist", libPath)
			return ""
		}
		return libPath
	}
	switch runtime.GOOS {
	case "windows":
		libPath = "target/debug/tokenizers.dll"
	case "darwin":
		libPath = "target/debug/libtokenizers.dylib"
	case "linux":
		libPath = "target/debug/libtokenizers.so"
	default:
		t.Skipf("Unsupported platform: %s", runtime.GOOS)
		return ""
	}

	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		t.Skipf("Skipping test because %s does not exist", libPath)
	}
	return libPath
}

func TestDownloadLibraryFromGitHub(t *testing.T) {
	versions, err := GetAvailableVersions()
	require.NoError(t, err, "Failed to get available versions")
	if len(versions) == 0 {
		t.Skip("No versions available to download")
	}
	testPath := filepath.Join(os.TempDir(), "tokenizers.dylib")
	err = DownloadLibraryFromGitHub(testPath)
	require.NoError(t, err, "Failed to download library from GitHub")
	require.FileExists(t, testPath)
	t.Cleanup(func() {
		err := os.Remove(testPath)
		require.NoError(t, err, "Failed to remove downloaded library")
	})
}

func TestGetPlatformAssetName(t *testing.T) {
	assetName := getPlatformAssetName()
	t.Logf("Platform asset name: %s", assetName)

	// Verify it contains expected platform components
	if runtime.GOOS == "darwin" && !contains(assetName, "apple-darwin") {
		t.Errorf("Expected asset name to contain 'apple-darwin' for macOS, got: %s", assetName)
	}
	if runtime.GOOS == "linux" && !contains(assetName, "unknown-linux-gnu") {
		t.Errorf("Expected asset name to contain 'unknown-linux-gnu' for Linux, got: %s", assetName)
	}
	if runtime.GOOS == "windows" && !contains(assetName, "pc-windows-msvc") {
		t.Errorf("Expected asset name to contain 'pc-windows-msvc' for Windows, got: %s", assetName)
	}
}

func TestGetCachedLibraryPath(t *testing.T) {
	path := GetCachedLibraryPath()
	t.Logf("Cached library path: %s", path)

	// Verify path contains expected library name
	expectedName := getLibraryName()
	if filepath.Base(path) != expectedName {
		t.Errorf("Expected cached path to end with %s, got: %s", expectedName, path)
	}
}

func TestCacheDirectory(t *testing.T) {
	cacheDir := getCacheDir()
	t.Logf("Cache directory: %s", cacheDir)

	// Verify cache directory is platform appropriate
	switch runtime.GOOS {
	case "darwin":
		if !contains(cacheDir, "Library/Caches") {
			t.Errorf("Expected macOS cache dir to contain 'Library/Caches', got: %s", cacheDir)
		}
	case "linux":
		if !contains(cacheDir, ".cache") && !contains(cacheDir, "XDG_CACHE_HOME") {
			t.Logf("Linux cache dir: %s (using temp dir is acceptable)", cacheDir)
		}
	case "windows":
		if !contains(cacheDir, "APPDATA") && !contains(cacheDir, "tokenizers") {
			t.Logf("Windows cache dir: %s (using temp dir is acceptable)", cacheDir)
		}
	}
}

func TestLibraryValidation(t *testing.T) {
	// Test with non-existent file
	if isLibraryValid("/non/existent/path") {
		t.Error("Expected non-existent file to be invalid")
	}

	// Test with regular file (should be invalid as library)
	tempFile := filepath.Join(os.TempDir(), "test-not-a-library.txt")
	if err := os.WriteFile(tempFile, []byte("not a library"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(tempFile); err != nil {
			t.Errorf("Failed to remove temp file: %v", err)
		}
	}()

	if isLibraryValid(tempFile) {
		t.Error("Expected regular text file to be invalid as library")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsAt(s, substr)))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
func TestGetLibraryInfo(t *testing.T) {
	info := GetLibraryInfo()

	// Check required fields
	requiredFields := []string{
		"platform_asset_name", "library_name", "cache_path",
		"cache_dir", "is_cached", "github_repo", "version",
	}

	for _, field := range requiredFields {
		if _, exists := info[field]; !exists {
			t.Errorf("Expected field %s to exist in library info", field)
		}
	}

	// Verify types
	if _, ok := info["is_cached"].(bool); !ok {
		t.Error("Expected is_cached to be a boolean")
	}

	if env, ok := info["environment"].(map[string]string); ok {
		t.Logf("Environment variables: %+v", env)
	}

	t.Logf("Library info: %+v", info)
}

func TestDownloadFunctionality(t *testing.T) {
	// Test that download functions exist and can be called
	// Note: These may fail if no releases exist, but should not panic

	t.Run("GetAvailableVersions", func(t *testing.T) {
		versions, err := GetAvailableVersions()
		if err != nil {
			t.Logf("GetAvailableVersions failed (expected if no releases): %v", err)
		} else {
			t.Logf("Available versions: %v", versions)
		}
	})

	t.Run("IsLibraryCached", func(t *testing.T) {
		cached := IsLibraryCached()
		t.Logf("Library is cached: %v", cached)
	})

	t.Run("ClearLibraryCache", func(t *testing.T) {
		err := ClearLibraryCache()
		if err != nil {
			t.Logf("Clear cache failed: %v", err)
		} else {
			t.Log("Cache cleared successfully")
		}
	})
}

func TestFromFile(t *testing.T) {
	libpath := checkLibraryExists(t)
	tok, err := FromFile("./tokenizer.json", WithLibraryPath(libpath))
	require.NoError(t, err, "Failed to load tokenizer from file")
	t.Cleanup(func() {
		_ = tok.Close()
	})
	res, err := tok.Encode("Hello, world!", WithReturnAllAttributes())
	require.NoError(t, err, "Failed to encode text")
	require.Contains(t, res.IDs, uint32(7592))
	require.Contains(t, res.IDs, uint32(1010))
	require.Contains(t, res.IDs, uint32(2088))
	require.Contains(t, res.IDs, uint32(999))
	require.Contains(t, res.IDs, uint32(102))
	require.Contains(t, res.Tokens, "hello")
	require.Contains(t, res.Tokens, ",")
	require.Contains(t, res.Tokens, "world")
	require.Contains(t, res.Tokens, "!")
	require.Len(t, res.Tokens, 128)
}

func TestFromBytes(t *testing.T) {
	libpath := checkLibraryExists(t)
	data, err := os.ReadFile("./tokenizer.json")
	require.NoError(t, err, "Failed to read tokenizer file")
	tok, err := FromBytes(data, WithLibraryPath(libpath))
	require.NoError(t, err, "Failed to load tokenizer from file")
	t.Cleanup(func() {
		_ = tok.Close()
	})
	res, err := tok.Encode("Hello, world!", WithReturnAllAttributes())
	require.NoError(t, err, "Failed to encode text")
	require.Contains(t, res.IDs, uint32(7592))
	require.Contains(t, res.IDs, uint32(1010))
	require.Contains(t, res.IDs, uint32(2088))
	require.Contains(t, res.IDs, uint32(999))
	require.Contains(t, res.IDs, uint32(102))
	require.Contains(t, res.Tokens, "hello")
	require.Contains(t, res.Tokens, ",")
	require.Contains(t, res.Tokens, "world")
	require.Contains(t, res.Tokens, "!")
	require.Len(t, res.Tokens, 128)
}

func TestFromFileWithTruncation(t *testing.T) {
	libpath := checkLibraryExists(t)
	tok, err := FromFile("./tokenizer.json",
		WithLibraryPath(libpath),
		WithTruncation(256, TruncationDirectionDefault, TruncationStrategyDefault),
	)
	require.NoError(t, err, "Failed to load tokenizer from file")
	t.Cleanup(func() {
		_ = tok.Close()
	})
	res, err := tok.Encode("Hello, world!", WithReturnAllAttributes())
	require.NoError(t, err, "Failed to encode text")
	require.Contains(t, res.IDs, uint32(7592))
	require.Contains(t, res.IDs, uint32(1010))
	require.Contains(t, res.IDs, uint32(2088))
	require.Contains(t, res.IDs, uint32(999))
	require.Contains(t, res.IDs, uint32(102))
	require.Contains(t, res.Tokens, "hello")
	require.Contains(t, res.Tokens, ",")
	require.Contains(t, res.Tokens, "world")
	require.Contains(t, res.Tokens, "!")
	require.Len(t, res.Tokens, 128)
}

func TestFromFileWithPadding(t *testing.T) {
	libpath := checkLibraryExists(t)
	tok, err := FromFile("./tokenizer.json",
		WithLibraryPath(libpath),
		WithPadding(true, PaddingStrategy{Tag: PaddingStrategyFixed, FixedSize: 256}),
	)
	require.NoError(t, err, "Failed to load tokenizer from file")
	t.Cleanup(func() {
		_ = tok.Close()
	})
	res, err := tok.Encode("Hello, world!", WithReturnAllAttributes())
	require.NoError(t, err, "Failed to encode text")
	require.Contains(t, res.IDs, uint32(7592))
	require.Contains(t, res.IDs, uint32(1010))
	require.Contains(t, res.IDs, uint32(2088))
	require.Contains(t, res.IDs, uint32(999))
	require.Contains(t, res.IDs, uint32(102))
	require.Contains(t, res.Tokens, "hello")
	require.Contains(t, res.Tokens, ",")
	require.Contains(t, res.Tokens, "world")
	require.Contains(t, res.Tokens, "!")
	require.Len(t, res.Tokens, 256)
}

func TestErrors(t *testing.T) {
	// Test invalid library path
	_, err := FromFile("./tokenizer.json", WithLibraryPath("/invalid/path/to/library"))
	require.Error(t, err, "Expected error for invalid library path")

	// Test invalid tokenizer file
	_, err = FromFile("/invalid/path/to/tokenizer.json")
	require.Error(t, err, "Expected error for invalid tokenizer file")

	// Test unsupported tokenizer type
	_, err = FromFile("./unsupported_tokenizer_type.json")
	require.Error(t, err, "Expected error for unsupported tokenizer type")
}
