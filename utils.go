package tokenizers

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"github.com/pkg/errors"
)

func MasksFromBuf(buf Buffer) (special, attention []uint32) {
	n := int(buf.Len)
	if n == 0 {
		return nil, nil // Return empty slices if length is zero
	}

	if buf.SpecialTokensMask != nil {
		special = unsafe.Slice(buf.SpecialTokensMask, n)

	}
	if buf.AttentionMask != nil {
		attention = unsafe.Slice(buf.AttentionMask, n)
	}

	return
}

func TokensFromBuf(buf Buffer) []string {
	if buf.Tokens == nil || buf.Len == 0 {
		return nil
	}
	ptrs := unsafe.Slice(buf.Tokens, buf.Len) // []*byte
	out := make([]string, 0, len(ptrs))

	for _, p := range ptrs {
		if p == nil {
			continue
		}
		q := unsafe.Pointer(p)
		var n uintptr
		for *(*byte)(unsafe.Add(q, n)) != 0 {
			n++
		}
		b := unsafe.Slice((*byte)(q), n)
		out = append(out, string(b))
	}
	return out
}

func downloadLibrary(dest string) error {
	return DownloadLibraryFromGitHub(dest)
}

func getCacheDir() string {
	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "tokenizers", "lib")
		}
		return filepath.Join(os.TempDir(), "tokenizers", "lib")
	case "darwin":
		if home := os.Getenv("HOME"); home != "" {
			return filepath.Join(home, "Library", "Caches", "tokenizers", "lib")
		}
	default: // linux
		if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
			return filepath.Join(xdgCache, "tokenizers", "lib")
		}
		if home := os.Getenv("HOME"); home != "" {
			return filepath.Join(home, ".cache", "tokenizers", "lib")
		}
	}
	return filepath.Join(os.TempDir(), "tokenizers", "lib")
}

func isMusl() bool {
	matches, err := filepath.Glob("/lib/ld-musl-*.so*")
	return err == nil && len(matches) > 0
}

func getLibraryName() string {
	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf("%s.dll", LibName)
	case "darwin":
		return fmt.Sprintf("lib%s.dylib", LibName)
	default: // gnu/linux, freebsd, etc.
		// For musl libc, we use .a (static library) instead of .so (shared library)
		// This is a workaround for musl-based systems where shared libraries
		if isMusl() {
			return fmt.Sprintf("lib%s.a", LibName)
		}
		return fmt.Sprintf("lib%s.so", LibName)
	}
}

func LoadTokenizerLibrary(userProvidedPath string) (uintptr, error) {
	// 1. Check explicit user path
	if userProvidedPath != "" {
		if _, err := os.Stat(userProvidedPath); os.IsNotExist(err) {
			return 0, errors.Errorf("shared library does not exist at user-provided path: %s", userProvidedPath)
		} else if err != nil {
			return 0, errors.Wrapf(err, "error checking user-provided path: %s", userProvidedPath)
		}

		if lib, err := loadLibrary(userProvidedPath); err == nil {
			return lib, nil
		} else {
			return 0, errors.Wrapf(err, "failed to load library from user-provided path: %s", userProvidedPath)
		}
	}

	// 2. Check environment variable
	if envPath := os.Getenv("TOKENIZERS_LIB_PATH"); envPath != "" {
		if lib, err := loadLibrary(envPath); err == nil {
			return lib, nil
		} else {
			return 0, errors.Wrapf(err, "failed to load library from environment variable TOKENIZERS_LIB_PATH: %s", envPath)
		}
	}

	// 3. Try cache location
	cacheDir := getCacheDir()
	cachedPath := filepath.Join(cacheDir, getLibraryName())

	if isLibraryValid(cachedPath) {
		if lib, err := loadLibrary(cachedPath); err == nil {
			return lib, nil
		}
	}

	// 4. Download to cache and load
	if err := downloadLibrary(cachedPath); err != nil {
		return 0, errors.Wrap(err, "failed to download library")
	}

	return loadLibrary(cachedPath)
}
