package tokenizers

import (
	"os"
	"path/filepath"
	"runtime"
	"unsafe"
)

func MasksFromBuf(buf Buffer) (special, attention []uint32) {
	n := int(buf.Len)
	if n == 0 {
		return nil, nil // Return empty slices if length is zero
	}

	if buf.SpecialTokensMask != nil {
		special = unsafe.Slice(buf.SpecialTokensMask, n)

	}
	if buf.AttentionMask == nil {
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
		out = append(out, unsafe.String((*byte)(q), int(n)))
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
