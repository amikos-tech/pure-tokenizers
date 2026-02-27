package tokenizers

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/ebitengine/purego"
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
		if err := verifyLibraryABICompatibility(libPath); err != nil {
			t.Skipf("Skipping test because %s is ABI/symbol incompatible: %v", libPath, err)
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
	if err := verifyLibraryABICompatibility(libPath); err != nil {
		t.Skipf("Skipping test because %s is ABI/symbol incompatible: %v", libPath, err)
		return ""
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

func TestNormalizeReleaseVersion(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "empty", input: "", expected: "latest"},
		{name: "latest", input: "latest", expected: "latest"},
		{name: "already normalized", input: "rust-v0.1.3", expected: "rust-v0.1.3"},
		{name: "v prefix", input: "v0.1.3", expected: "rust-v0.1.3"},
		{name: "raw semver", input: "0.1.3", expected: "rust-v0.1.3"},
		{name: "legacy rust tag", input: "rust-0.1.3", expected: "rust-v0.1.3"},
		{name: "non semver rust prefix", input: "rust-nightly", expected: "rust-nightly"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeReleaseVersion(tc.input)
			require.Equal(t, tc.expected, got)
		})
	}
}

func TestBuildReleaseURL(t *testing.T) {
	require.Equal(
		t,
		"https://releases.amikos.tech/pure-tokenizers/rust-v0.1.0/SHA256SUMS",
		buildReleaseURL("/pure-tokenizers/", "/rust-v0.1.0/", "SHA256SUMS"),
	)
	require.Equal(t, "https://releases.amikos.tech", buildReleaseURL("", "/"))
}

func TestSetRequestHeaders(t *testing.T) {
	t.Run("github host prefers GITHUB_TOKEN and includes api version", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "test-gh-token")
		t.Setenv("GH_TOKEN", "test-gh-cli-token")
		req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/amikos-tech/pure-tokenizers/releases", nil)
		require.NoError(t, err)

		setRequestHeaders(req, "application/json")

		require.Equal(t, "application/json", req.Header.Get("Accept"))
		require.Equal(t, "pure-tokenizers-downloader", req.Header.Get("User-Agent"))
		require.Equal(t, apiVer, req.Header.Get("X-GitHub-Api-Version"))
		require.Equal(t, "Bearer test-gh-token", req.Header.Get("Authorization"))
	})

	t.Run("github host uses GH_TOKEN when GITHUB_TOKEN is absent", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("GH_TOKEN", "test-gh-cli-token")
		req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/amikos-tech/pure-tokenizers/releases", nil)
		require.NoError(t, err)

		setRequestHeaders(req, "application/json")

		require.Equal(t, "application/json", req.Header.Get("Accept"))
		require.Equal(t, "pure-tokenizers-downloader", req.Header.Get("User-Agent"))
		require.Equal(t, apiVer, req.Header.Get("X-GitHub-Api-Version"))
		require.Equal(t, "Bearer test-gh-cli-token", req.Header.Get("Authorization"))
	})

	t.Run("non github host never includes github auth headers", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "test-gh-token")
		t.Setenv("GH_TOKEN", "test-gh-cli-token")
		req, err := http.NewRequest(http.MethodGet, "https://releases.amikos.tech/pure-tokenizers/latest.json", nil)
		require.NoError(t, err)

		setRequestHeaders(req, "application/json")

		require.Equal(t, "application/json", req.Header.Get("Accept"))
		require.Equal(t, "pure-tokenizers-downloader", req.Header.Get("User-Agent"))
		require.Empty(t, req.Header.Get("X-GitHub-Api-Version"))
		require.Empty(t, req.Header.Get("Authorization"))
	})
}

func TestResolveChecksumsURL(t *testing.T) {
	t.Run("default path", func(t *testing.T) {
		url, err := resolveChecksumsURL("rust-v0.1.0", nil)
		require.NoError(t, err)
		require.Equal(t, "https://releases.amikos.tech/pure-tokenizers/rust-v0.1.0/SHA256SUMS", url)
	})

	t.Run("relative checksums url", func(t *testing.T) {
		url, err := resolveChecksumsURL("rust-v0.1.0", &releaseIndex{ChecksumsURL: "pure-tokenizers/rust-v0.1.0/SHA256SUMS"})
		require.NoError(t, err)
		require.Equal(t, "https://releases.amikos.tech/pure-tokenizers/rust-v0.1.0/SHA256SUMS", url)
	})

	t.Run("absolute releases host checksums url", func(t *testing.T) {
		url, err := resolveChecksumsURL("rust-v0.1.0", &releaseIndex{ChecksumsURL: "https://releases.amikos.tech/pure-tokenizers/rust-v0.1.0/SHA256SUMS"})
		require.NoError(t, err)
		require.Equal(t, "https://releases.amikos.tech/pure-tokenizers/rust-v0.1.0/SHA256SUMS", url)
	})

	t.Run("absolute github objects host checksums url", func(t *testing.T) {
		url, err := resolveChecksumsURL("rust-v0.1.0", &releaseIndex{ChecksumsURL: "https://objects.githubusercontent.com/github-production-release-asset-2e65be/SHA256SUMS"})
		require.NoError(t, err)
		require.Equal(t, "https://objects.githubusercontent.com/github-production-release-asset-2e65be/SHA256SUMS", url)
	})

	t.Run("absolute http checksums url rejected", func(t *testing.T) {
		_, err := resolveChecksumsURL("rust-v0.1.0", &releaseIndex{ChecksumsURL: "http://cdn.example.com/SHA256SUMS"})
		require.Error(t, err)
	})

	t.Run("absolute https checksums url rejected for unknown host", func(t *testing.T) {
		_, err := resolveChecksumsURL("rust-v0.1.0", &releaseIndex{ChecksumsURL: "https://cdn.example.com/SHA256SUMS"})
		require.Error(t, err)
	})
}

func TestChecksumForAsset(t *testing.T) {
	sumA := strings.Repeat("a", 64)
	sumB := strings.Repeat("b", 64)
	assetName := "libtokenizers-x86_64-apple-darwin.tar.gz"

	t.Run("extract checksum from manifest", func(t *testing.T) {
		manifest := sumA + "  " + assetName + "\n" + sumB + "  other.tar.gz\n"
		sum, err := checksumForAsset(manifest, assetName)
		require.NoError(t, err)
		require.Equal(t, sumA, sum)
	})

	t.Run("extract checksum from star-prefixed entry", func(t *testing.T) {
		manifest := sumA + " *" + assetName + "\n"
		sum, err := checksumForAsset(manifest, assetName)
		require.NoError(t, err)
		require.Equal(t, sumA, sum)
	})

	t.Run("missing asset entry", func(t *testing.T) {
		manifest := sumA + "  other.tar.gz\n"
		_, err := checksumForAsset(manifest, assetName)
		require.ErrorIs(t, err, errChecksumAssetNotFound)
	})

	t.Run("invalid manifest entry", func(t *testing.T) {
		manifest := "not-a-valid-line\n"
		_, err := checksumForAsset(manifest, assetName)
		require.ErrorIs(t, err, errChecksumManifestInvalid)
	})

	t.Run("empty manifest", func(t *testing.T) {
		_, err := checksumForAsset("", assetName)
		require.ErrorIs(t, err, errChecksumManifestInvalid)
	})

	t.Run("comment-only manifest", func(t *testing.T) {
		_, err := checksumForAsset("# comment\n   # another\n", assetName)
		require.ErrorIs(t, err, errChecksumManifestInvalid)
	})
}

func TestLooksLikeSHA256(t *testing.T) {
	t.Run("valid lowercase hash", func(t *testing.T) {
		require.True(t, looksLikeSHA256(strings.Repeat("a", 64)))
	})

	t.Run("valid uppercase hash", func(t *testing.T) {
		require.True(t, looksLikeSHA256(strings.Repeat("A", 64)))
	})

	t.Run("too short", func(t *testing.T) {
		require.False(t, looksLikeSHA256(strings.Repeat("a", 63)))
	})

	t.Run("too long", func(t *testing.T) {
		require.False(t, looksLikeSHA256(strings.Repeat("a", 65)))
	})

	t.Run("contains non-hex", func(t *testing.T) {
		require.False(t, looksLikeSHA256(strings.Repeat("a", 63)+"g"))
	})
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
		"releases_base_url", "releases_project",
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
		strict := os.Getenv("TOKENIZERS_REQUIRE_ONLINE_TESTS") == "1"
		versions, err := GetAvailableVersions()
		if err != nil {
			if strict {
				require.NoError(t, err, "expected releases endpoint or GitHub fallback to resolve versions")
			}
			t.Skipf("GetAvailableVersions unavailable in this environment: %v", err)
		}
		require.NotEmpty(t, versions)
		t.Logf("Available versions: %v", versions)
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

func TestEncodeOptions(t *testing.T) {

	libpath := checkLibraryExists(t)
	tok, err := FromFile("./tokenizer.json", WithLibraryPath(libpath))
	require.NoError(t, err, "Failed to load tokenizer from file")
	t.Cleanup(func() {
		_ = tok.Close()
	})
	t.Run("No Options", func(t *testing.T) {
		res, err := tok.Encode("Hello, world!")
		require.Len(t, res.IDs, 128)
		require.Len(t, res.Tokens, 128)
		require.Len(t, res.TypeIDs, 0)
		require.Len(t, res.SpecialTokensMask, 0)
		require.Len(t, res.AttentionMask, 0)
		require.Len(t, res.Offsets, 0)
		require.NoError(t, err, "Failed to encode text without options")
		require.Contains(t, res.Tokens, "hello")
		require.Contains(t, res.Tokens, ",")
		require.Contains(t, res.Tokens, "world")
		require.Contains(t, res.Tokens, "!")
	})

	t.Run("WithReturnTypeIDs", func(t *testing.T) {
		res, err := tok.Encode("Hello, world!", WithReturnTypeIDs())
		require.Len(t, res.IDs, 128)
		require.Len(t, res.Tokens, 128)
		require.Len(t, res.TypeIDs, 128)
		require.Len(t, res.SpecialTokensMask, 0)
		require.Len(t, res.AttentionMask, 0)
		require.Len(t, res.Offsets, 0)
		require.NoError(t, err, "Failed to encode text without options")
		require.Contains(t, res.Tokens, "hello")
		require.Contains(t, res.Tokens, ",")
		require.Contains(t, res.Tokens, "world")
		require.Contains(t, res.Tokens, "!")
	})

	t.Run("WithReturnSpecialTokensMask", func(t *testing.T) {
		res, err := tok.Encode("Hello, world!", WithReturnSpecialTokensMask())
		require.Len(t, res.IDs, 128)
		require.Len(t, res.Tokens, 128)
		require.Len(t, res.TypeIDs, 0)
		require.Len(t, res.SpecialTokensMask, 128)
		require.Len(t, res.AttentionMask, 0)
		require.Len(t, res.Offsets, 0)
		require.NoError(t, err, "Failed to encode text without options")
		require.Contains(t, res.Tokens, "hello")
		require.Contains(t, res.Tokens, ",")
		require.Contains(t, res.Tokens, "world")
		require.Contains(t, res.Tokens, "!")
	})

	t.Run("WithReturnAttentionMask", func(t *testing.T) {
		res, err := tok.Encode("Hello, world!", WithReturnAttentionMask())
		require.Len(t, res.IDs, 128)
		require.Len(t, res.Tokens, 128)
		require.Len(t, res.TypeIDs, 0)
		require.Len(t, res.SpecialTokensMask, 0)
		require.Len(t, res.AttentionMask, 128)
		require.Len(t, res.Offsets, 0)
		require.NoError(t, err, "Failed to encode text without options")
		require.Contains(t, res.Tokens, "hello")
		require.Contains(t, res.Tokens, ",")
		require.Contains(t, res.Tokens, "world")
		require.Contains(t, res.Tokens, "!")
	})

	t.Run("WithReturnOffsets", func(t *testing.T) {
		res, err := tok.Encode("Hello, world!", WithReturnOffsets())
		require.Len(t, res.IDs, 128)
		require.Len(t, res.Tokens, 128)
		require.Len(t, res.TypeIDs, 0)
		require.Len(t, res.SpecialTokensMask, 0)
		require.Len(t, res.AttentionMask, 0)
		require.Len(t, res.Offsets, 128*2)
		require.NoError(t, err, "Failed to encode text without options")
		require.Contains(t, res.Tokens, "hello")
		require.Contains(t, res.Tokens, ",")
		require.Contains(t, res.Tokens, "world")
		require.Contains(t, res.Tokens, "!")
	})

	t.Run("WithAddSpecialTokens", func(t *testing.T) {
		res, err := tok.Encode("Hello, world!", WithAddSpecialTokens())
		require.Len(t, res.IDs, 128)
		require.Len(t, res.Tokens, 128)
		require.Len(t, res.TypeIDs, 0)
		require.Len(t, res.SpecialTokensMask, 0)
		require.Len(t, res.AttentionMask, 0)
		require.Len(t, res.Offsets, 0)
		require.NoError(t, err, "Failed to encode text without options")
		require.Contains(t, res.Tokens, "[CLS]")
		require.Contains(t, res.Tokens, "[SEP]")
		require.Contains(t, res.Tokens, "hello")
		require.Contains(t, res.Tokens, ",")
		require.Contains(t, res.Tokens, "world")
		require.Contains(t, res.Tokens, "!")
	})

	t.Run("WithReturnTokens", func(t *testing.T) {
		res, err := tok.Encode("Hello, world!", WithReturnTokens())
		require.Len(t, res.IDs, 128)
		require.Len(t, res.Tokens, 128)
		require.Len(t, res.TypeIDs, 0)
		require.Len(t, res.SpecialTokensMask, 0)
		require.Len(t, res.AttentionMask, 0)
		require.Len(t, res.Offsets, 0)
		require.NoError(t, err, "Failed to encode text without options")
		require.Contains(t, res.Tokens, "hello")
		require.Contains(t, res.Tokens, ",")
		require.Contains(t, res.Tokens, "world")
		require.Contains(t, res.Tokens, "!")
	})
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

func TestEncodePair(t *testing.T) {
	libpath := checkLibraryExists(t)
	tok, err := FromFile("./tokenizer.json", WithLibraryPath(libpath))
	require.NoError(t, err, "Failed to load tokenizer from file")
	t.Cleanup(func() {
		_ = tok.Close()
	})

	t.Run("Single pair encoding", func(t *testing.T) {
		res, err := tok.EncodePair("Hello, world!", "How are you?", WithReturnAllAttributes())
		require.NoError(t, err, "Failed to encode pair")
		require.NotNil(t, res)
		require.Greater(t, len(res.IDs), 0)
		require.Contains(t, res.Tokens, "hello")
		require.Contains(t, res.Tokens, "how")
		// BERT adds [SEP] between sequences
		require.Contains(t, res.Tokens, "[SEP]")
	})

	t.Run("Empty pair", func(t *testing.T) {
		res, err := tok.EncodePair("Hello", "", WithReturnTokens())
		require.NoError(t, err, "Failed to encode pair with empty second sequence")
		require.NotNil(t, res)
		require.Greater(t, len(res.IDs), 0)
	})
}

func TestEncodePairs(t *testing.T) {
	libpath := checkLibraryExists(t)
	tok, err := FromFile("./tokenizer.json", WithLibraryPath(libpath))
	require.NoError(t, err, "Failed to load tokenizer from file")
	t.Cleanup(func() {
		_ = tok.Close()
	})

	t.Run("Multiple pairs encoding", func(t *testing.T) {
		sequences := []string{
			"What is the capital of France?",
			"Who wrote Romeo and Juliet?",
			"What is 2+2?",
		}
		pairs := []string{
			"Paris is the capital of France.",
			"William Shakespeare wrote Romeo and Juliet.",
			"The answer is 4.",
		}

		results, err := tok.EncodePairs(sequences, pairs, WithReturnAllAttributes())
		require.NoError(t, err, "Failed to encode pairs")
		require.Len(t, results, 3)

		for i, res := range results {
			require.Greater(t, len(res.IDs), 0, "Result %d should have IDs", i)
			require.Greater(t, len(res.Tokens), 0, "Result %d should have tokens", i)
			require.Contains(t, res.Tokens, "[SEP]", "Result %d should contain separator token", i)
		}
	})

	t.Run("Single pair batch", func(t *testing.T) {
		results, err := tok.EncodePairs(
			[]string{"Hello"},
			[]string{"World"},
			WithReturnTokens(),
		)
		require.NoError(t, err, "Failed to encode single pair batch")
		require.Len(t, results, 1)
		require.Greater(t, len(results[0].IDs), 0)
	})

	t.Run("Empty batch", func(t *testing.T) {
		results, err := tok.EncodePairs([]string{}, []string{}, WithReturnTokens())
		require.NoError(t, err, "Failed to encode empty batch")
		require.Len(t, results, 0)
	})

	t.Run("Mismatched lengths", func(t *testing.T) {
		_, err := tok.EncodePairs(
			[]string{"Hello", "World"},
			[]string{"Foo"},
			WithReturnTokens(),
		)
		require.Error(t, err, "Should fail with mismatched lengths")
		require.Contains(t, err.Error(), "same length")
	})

	t.Run("Batch with options", func(t *testing.T) {
		sequences := []string{"Query 1", "Query 2"}
		pairs := []string{"Document 1", "Document 2"}

		results, err := tok.EncodePairs(sequences, pairs,
			WithReturnTypeIDs(),
			WithReturnAttentionMask(),
			WithReturnOffsets(),
		)
		require.NoError(t, err)
		require.Len(t, results, 2)

		for i, res := range results {
			require.Greater(t, len(res.TypeIDs), 0, "Result %d should have type IDs", i)
			require.Greater(t, len(res.AttentionMask), 0, "Result %d should have attention mask", i)
			require.Greater(t, len(res.Offsets), 0, "Result %d should have offsets", i)
		}
	})

	t.Run("Null termination with long strings", func(t *testing.T) {
		// Test with longer strings to ensure null termination works correctly
		// Longer strings are less likely to have lucky null bytes in adjacent memory
		longSeq := strings.Repeat("A", 100)
		longPair := strings.Repeat("B", 100)

		results, err := tok.EncodePairs([]string{longSeq}, []string{longPair}, WithReturnTokens())
		require.NoError(t, err, "Failed to encode long strings")
		require.Len(t, results, 1)
		require.Greater(t, len(results[0].IDs), 0, "Should have token IDs")
		require.Greater(t, len(results[0].Tokens), 0, "Should have tokens")
	})
}

func TestAbi(t *testing.T) {
	t.Run("Compatible ABI", func(t *testing.T) {
		constraint, err := semver.NewConstraint("v0.1.x")
		require.NoError(t, err)
		mockt := &Tokenizer{
			getVersion: func() string {
				return "0.1.0"
			},
		}
		err = mockt.abiCheck(constraint)
		require.NoError(t, err)
	})

	t.Run("Compatible ABI - pre-release", func(t *testing.T) {
		constraint, err := semver.NewConstraint("^v0.2.0-0")
		require.NoError(t, err)
		mockt := &Tokenizer{
			getVersion: func() string {
				return "0.2.0-alpha.1"
			},
		}
		err = mockt.abiCheck(constraint)
		require.NoError(t, err)
	})
	t.Run("Incompatible ABI", func(t *testing.T) {
		constraint, err := semver.NewConstraint("v0.2.x")
		require.NoError(t, err)
		mockt := &Tokenizer{
			getVersion: func() string {
				return "0.1.0"
			},
		}
		err = mockt.abiCheck(constraint)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not compatible")
	})

	t.Run("Nil constraint", func(t *testing.T) {

		mockt := &Tokenizer{
			getVersion: func() string {
				return "0.1.0"
			},
		}
		err := mockt.abiCheck(nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "ABI version constraint cannot be nil")
	})

	t.Run("getVersion uninitialized", func(t *testing.T) {
		mockt := &Tokenizer{
			getVersion: nil,
		}
		constraint, _ := semver.NewConstraint(AbiCompatibilityConstraint)
		err := mockt.abiCheck(constraint)
		require.Error(t, err)
		require.Contains(t, err.Error(), "getVersion function is not initialized")
	})

	t.Run("Invalid version", func(t *testing.T) {
		mockt := &Tokenizer{
			getVersion: func() string {
				return "dqwe12321"
			},
		}
		constraint, _ := semver.NewConstraint(AbiCompatibilityConstraint)
		err := mockt.abiCheck(constraint)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to parse version string")
	})

	t.Run("GetVersion from Rust", func(t *testing.T) {
		libpath := checkLibraryExists(t)
		var libh uintptr
		var err error
		libh, err = loadLibrary(libpath)
		require.NoError(t, err)
		t.Cleanup(func() {
			if libh != 0 {
				_ = closeLibrary(libh)
			}
		})
		mockt := &Tokenizer{}
		purego.RegisterLibFunc(&mockt.getVersion, libh, "get_version")
		require.NotNil(t, mockt.getVersion, "getVersion function should not be nil")
		constraint, _ := semver.NewConstraint(AbiCompatibilityConstraint)
		err = mockt.abiCheck(constraint)
		require.NoError(t, err)

	})
}
