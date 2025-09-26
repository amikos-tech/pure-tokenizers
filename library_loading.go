package tokenizers

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pkg/errors"
)

// LoadTokenizerLibrary loads the tokenizer shared library from the specified path
// or attempts to find it through various fallback mechanisms:
// 1. User-provided path
// 2. TOKENIZERS_LIB_PATH environment variable
// 3. Cached library in platform-specific directory
// 4. Automatic download from GitHub releases
func LoadTokenizerLibrary(userPath string) (uintptr, error) {
	// Priority 1: User-provided path
	if userPath != "" {
		if _, err := os.Stat(userPath); err == nil {
			libh, err := loadLibrary(userPath)
			if err == nil {
				return libh, nil
			}
			return 0, errors.Wrapf(err, "failed to load library from user-provided path: %s", userPath)
		}
		return 0, errors.Errorf("library file not found at user-provided path: %s", userPath)
	}

	// Priority 2: Environment variable
	if envPath := os.Getenv("TOKENIZERS_LIB_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			libh, err := loadLibrary(envPath)
			if err == nil {
				return libh, nil
			}
			return 0, errors.Wrapf(err, "failed to load library from TOKENIZERS_LIB_PATH: %s", envPath)
		}
		return 0, errors.Errorf("library file not found at TOKENIZERS_LIB_PATH: %s", envPath)
	}

	// Priority 3: Cached library
	cachedPath := GetCachedLibraryPath()
	if isLibraryValid(cachedPath) {
		libh, err := loadLibrary(cachedPath)
		if err == nil {
			return libh, nil
		}
		// If cached library fails to load, try to re-download
		_ = ClearLibraryCache()
	}

	// Priority 4: Download from GitHub
	if err := DownloadAndCacheLibrary(); err != nil {
		return 0, errors.Wrap(err, "failed to download library from GitHub releases")
	}

	// Try loading the newly downloaded library
	libh, err := loadLibrary(cachedPath)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to load downloaded library from: %s", cachedPath)
	}

	return libh, nil
}

// getLibraryName returns the platform-specific library name
func getLibraryName() string {
	switch runtime.GOOS {
	case "darwin":
		return "libtokenizers.dylib"
	case "linux":
		return "libtokenizers.so"
	case "windows":
		return "tokenizers.dll"
	default:
		return fmt.Sprintf("libtokenizers_%s", runtime.GOOS)
	}
}

// getCacheDir returns the platform-specific cache directory
func getCacheDir() string {
	var cacheDir string

	switch runtime.GOOS {
	case "darwin":
		// macOS: ~/Library/Caches/tokenizers/lib
		if home, err := os.UserHomeDir(); err == nil {
			cacheDir = filepath.Join(home, "Library", "Caches", "tokenizers", "lib")
		}
	case "linux":
		// Linux: Use XDG_CACHE_HOME or ~/.cache
		if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
			cacheDir = filepath.Join(xdgCache, "tokenizers", "lib")
		} else if home, err := os.UserHomeDir(); err == nil {
			cacheDir = filepath.Join(home, ".cache", "tokenizers", "lib")
		}
	case "windows":
		// Windows: %APPDATA%/tokenizers/lib
		if appData := os.Getenv("APPDATA"); appData != "" {
			cacheDir = filepath.Join(appData, "tokenizers", "lib")
		}
	}

	// Fallback to temp directory
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "tokenizers", "lib")
	}

	return cacheDir
}

// isMusl checks if the current Linux system uses musl libc
func isMusl() bool {
	// Check if ldd exists and mentions musl
	// This is a simplified check; a more robust implementation might
	// check the actual libc being linked
	if _, err := os.Stat("/lib/ld-musl-x86_64.so.1"); err == nil {
		return true
	}
	if _, err := os.Stat("/lib/ld-musl-aarch64.so.1"); err == nil {
		return true
	}
	return false
}