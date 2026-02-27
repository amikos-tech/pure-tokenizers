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
// 4. Automatic download from releases.amikos.tech (with GitHub Releases fallback)
func LoadTokenizerLibrary(userPath string) (uintptr, error) {
	// Priority 1: User-provided path
	if userPath != "" {
		if _, err := os.Stat(userPath); err == nil {
			if err := verifyLibraryABICompatibility(userPath); err != nil {
				return 0, errors.Wrapf(err, "library at user-provided path is ABI/symbol incompatible: %s", userPath)
			}
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
			if err := verifyLibraryABICompatibility(envPath); err != nil {
				return 0, errors.Wrapf(err, "library at TOKENIZERS_LIB_PATH is ABI/symbol incompatible: %s", envPath)
			}
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
	var cachedLoadErr error
	if isLibraryValid(cachedPath) {
		shouldClearCache := false

		if err := verifyLibraryABICompatibility(cachedPath); err != nil {
			cachedLoadErr = errors.Wrapf(err, "cached library failed ABI/symbol compatibility check: %s", cachedPath)
			shouldClearCache = true
		}

		if cachedLoadErr == nil {
			libh, err := loadLibrary(cachedPath)
			if err == nil {
				return libh, nil
			}
			cachedLoadErr = errors.Wrapf(err, "failed to load cached library from %s", cachedPath)
			shouldClearCache = true
		}

		// If cached library fails compatibility or load, clear cache once and re-download.
		if shouldClearCache {
			if clearErr := ClearLibraryCache(); clearErr != nil {
				_, _ = fmt.Fprintf(
					os.Stderr,
					"warning: failed to clear cached library %s (%v); continuing with re-download attempt\n",
					cachedPath,
					clearErr,
				)
			}
		}
	}

	// Priority 4: Download from releases endpoint (with GitHub fallback)
	if err := DownloadAndCacheLibrary(); err != nil {
		if cachedLoadErr != nil {
			return 0, errors.Wrapf(err, "failed to download library after cached load error: %v", cachedLoadErr)
		}
		return 0, errors.Wrap(err, "failed to download library from release endpoint")
	}

	// Try loading the newly downloaded library
	libh, err := loadLibrary(cachedPath)
	if err != nil {
		if cachedLoadErr != nil {
			return 0, errors.Wrapf(err, "failed to load downloaded library from %s (previous cached load error: %v)", cachedPath, cachedLoadErr)
		}
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
