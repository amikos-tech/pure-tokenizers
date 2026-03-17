package tokenizers

import (
	stderrors "errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newLifecycleTestTokenizer(t *testing.T) *Tokenizer {
	t.Helper()

	libpath := checkLibraryExists(t)
	tok, err := FromFile("./tokenizer.json", WithLibraryPath(libpath))
	require.NoError(t, err, "Failed to load tokenizer from file")
	return tok
}

func TestCloseIsIdempotent(t *testing.T) {
	tok := newLifecycleTestTokenizer(t)

	require.NoError(t, tok.Close())
	require.NoError(t, tok.Close())
}

func TestTokenizerMethodsReturnErrTokenizerClosed(t *testing.T) {
	tok := newLifecycleTestTokenizer(t)

	encoding, err := tok.Encode("Hello, world!")
	require.NoError(t, err)

	require.NoError(t, tok.Close())

	_, err = tok.Encode("Hello again")
	require.ErrorIs(t, err, ErrTokenizerClosed)

	_, err = tok.EncodePairs([]string{"query"}, []string{"document"})
	require.ErrorIs(t, err, ErrTokenizerClosed)

	_, err = tok.Decode(encoding.IDs, false)
	require.ErrorIs(t, err, ErrTokenizerClosed)

	_, err = tok.VocabSize()
	require.ErrorIs(t, err, ErrTokenizerClosed)

	require.Equal(t, "unknown", tok.GetLibraryVersion())
}

func TestCloseWaitsForActiveOperations(t *testing.T) {
	tok := newLifecycleTestTokenizer(t)

	tok.lifecycleMu.RLock()
	closeDone := make(chan error, 1)
	go func() {
		closeDone <- tok.Close()
	}()

	select {
	case err := <-closeDone:
		t.Fatalf("Close returned before active operations finished: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	tok.lifecycleMu.RUnlock()
	require.NoError(t, <-closeDone)

	_, err := tok.Encode("Hello after close")
	require.ErrorIs(t, err, ErrTokenizerClosed)
}

func TestConcurrentEncodeAndClose(t *testing.T) {
	tok := newLifecycleTestTokenizer(t)

	const goroutines = 8
	const iterationsPerGoroutine = 200

	start := make(chan struct{})
	errs := make(chan error, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < iterationsPerGoroutine; j++ {
				_, err := tok.Encode("Concurrent lifecycle test text")
				if err != nil && !stderrors.Is(err, ErrTokenizerClosed) {
					errs <- err
					return
				}
			}
		}()
	}

	close(start)
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, tok.Close())

	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}
