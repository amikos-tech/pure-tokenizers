package tokenizers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadLibraryFailures(t *testing.T) {
	t.Run("Missing library file", func(t *testing.T) {
		_, err := loadLibrary("nonexistent_library.so")
		require.Error(t, err)
		require.Contains(t, err.Error(), "no such file")
	})

	t.Run("Invalid library file", func(t *testing.T) {
		fakeLibPath := filepath.Join(t.TempDir(), "fake_library.so")
		// Create a fake file that is not a valid shared library
		err := os.WriteFile(fakeLibPath, []byte("not a valid library"), 0644)
		require.NoError(t, err)
		_, err = loadLibrary(fakeLibPath)
		require.Error(t, err)
		require.Contains(t, err.Error(), "is not valid")
	})
}

func TestCloseLibraryFailures(t *testing.T) {
	t.Run("Invalid handle", func(t *testing.T) {
		err := closeLibrary(0)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to close library")
	})
}

func TestIsLibraryValidFailures(t *testing.T) {
	t.Run("Nonexistent library file", func(t *testing.T) {
		valid := isLibraryValid("nonexistent_library.so")
		require.False(t, valid)
	})

	t.Run("Invalid library file", func(t *testing.T) {
		fakeLibPath := filepath.Join(t.TempDir(), "fake_library.so")
		err := os.WriteFile(fakeLibPath, []byte("not a valid library"), 0644)
		require.NoError(t, err)
		valid := isLibraryValid(fakeLibPath)
		require.False(t, valid)
	})
}
