//go:build windows

package tokenizers

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
)

func loadLibrary(path string) (uintptr, error) {
	handle, err := windows.LoadLibrary(path)
	if err != nil {
		return 0, fmt.Errorf("failed to load shared library: %w", err)
	}
	if handle == 0 {
		return 0, fmt.Errorf("shared library handle is nil after loading: %s", path)
	}
	return uintptr(handle), nil
}

func isLibraryValid(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}

	// Try to load the library to verify it's valid
	if handle, err := windows.LoadLibrary(path); err == nil {
		_ = windows.FreeLibrary(handle)
		return true
	}
	return false
}

func closeLibrary(handle uintptr) error {
	if err := windows.FreeLibrary(windows.Handle(handle)); err != nil {
		return fmt.Errorf("failed to close library: %w", err)
	}
	return nil
}
