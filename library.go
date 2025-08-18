//go:build !windows

package tokenizers

import (
	"fmt"
	"os"

	"github.com/ebitengine/purego"
)

func loadLibrary(path string) (uintptr, error) {
	libh, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return 0, fmt.Errorf("failed to load shared library: %w", err)
	}
	if libh == 0 {
		return 0, fmt.Errorf("shared library handle is nil after loading: %s", path)
	}
	return libh, nil
}

// isLibraryValid checks if the library file exists and is valid
func isLibraryValid(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}

	// Try to load the library to verify it's valid
	if libh, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL); err == nil {
		_ = purego.Dlclose(libh)
		return true
	}
	return false
}

func closeLibrary(handle uintptr) error {
	if err := purego.Dlclose(handle); err != nil {
		return fmt.Errorf("failed to close library: %w", err)
	}
	return nil
}
