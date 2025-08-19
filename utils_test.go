package tokenizers

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestMasksFromBuf(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		buf := Buffer{}
		s, a := MasksFromBuf(buf)
		require.Empty(t, s)
		require.Empty(t, a)
	})
	t.Run("SpecialTokenMask", func(t *testing.T) {
		specialTokens := []uint32{10, 20, 30}
		buf := Buffer{
			Len:               3,
			SpecialTokensMask: &specialTokens[0],
		}
		s, a := MasksFromBuf(buf)
		require.Len(t, s, len(specialTokens))
		require.Equal(t, specialTokens, s)
		require.Empty(t, a)
	})

	t.Run("AttentionMask", func(t *testing.T) {
		attentionMask := []uint32{10, 20, 30}
		buf := Buffer{
			Len:           3,
			AttentionMask: &attentionMask[0],
		}
		s, a := MasksFromBuf(buf)
		require.Len(t, a, len(attentionMask))
		require.Equal(t, attentionMask, a)
		require.Empty(t, s)
	})
}

type CStringArray struct {
	ptrs  []*byte  // slice of string pointers
	bufs  [][]byte // backing storage (NUL-terminated)
	count int
}

func StringsToPtrArray(strs []string) *CStringArray {
	a := &CStringArray{
		ptrs:  make([]*byte, len(strs)),
		bufs:  make([][]byte, len(strs)),
		count: len(strs),
	}

	for i, s := range strs {
		b := make([]byte, len(s)+1) // +1 for NUL
		copy(b, s)
		a.bufs[i] = b
		a.ptrs[i] = &b[0]
	}

	return a
}

func (a *CStringArray) Ptr() **byte {
	if len(a.ptrs) == 0 {
		return nil
	}
	return (**byte)(unsafe.Pointer(&a.ptrs[0]))
}

func (a *CStringArray) Len() int { return a.count }

func TestTokensFromBuf(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		buf := Buffer{}
		tokens := TokensFromBuf(buf)
		require.Empty(t, tokens)
	})

	t.Run("with tokens", func(t *testing.T) {
		buffTokens := []string{"hello", ",", "world"}
		ptrs := StringsToPtrArray(buffTokens)
		buf := Buffer{
			Len:    3,
			Tokens: ptrs.Ptr(),
		}
		tokens := TokensFromBuf(buf)
		require.NotEmpty(t, tokens)
		require.Len(t, tokens, len(buffTokens))
		for i, token := range tokens {
			require.Equal(t, buffTokens[i], token)
		}
	})
}

func TestLoadTokenizerLibrary(t *testing.T) {
	t.Run("Invalid path", func(t *testing.T) {
		_, err := LoadTokenizerLibrary("invalid/path/to/libtokenizers.so")
		require.Error(t, err)
		require.Contains(t, err.Error(), "shared library does not exist at user-provided path")
	})

	t.Run("Invalid library", func(t *testing.T) {
		fakeLibPath := filepath.Join(t.TempDir(), "fake_library.so")
		err := os.WriteFile(fakeLibPath, []byte("not a valid library"), 0644)
		require.NoError(t, err)
		_, err = LoadTokenizerLibrary(fakeLibPath)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to load library ")
	})

	t.Run("Invalid env var library path", func(t *testing.T) {
		//fakeLibPath := filepath.Join(t.TempDir(), "fake_library.so")
		//err := os.WriteFile(fakeLibPath, []byte("not a valid library"), 0644)
		//require.NoError(t, err)
		t.Setenv("TOKENIZERS_LIB_PATH", "invalid")
		_, err := LoadTokenizerLibrary("")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to load library ")
	})
}
