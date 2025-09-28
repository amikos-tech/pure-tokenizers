package tokenizers

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeWithEmptyIDs(t *testing.T) {
	libpath := checkLibraryExists(t)
	data, err := os.ReadFile("./tokenizer.json")
	require.NoError(t, err, "Failed to read tokenizer file")

	tok, err := FromBytes(data, WithLibraryPath(libpath))
	require.NoError(t, err)
	defer func() { _ = tok.Close() }()

	// Test with empty IDs slice - this should not crash
	decoded, err := tok.Decode([]uint32{}, false)
	assert.NoError(t, err)
	assert.Equal(t, "", decoded)
}

func TestDecodeWithValidIDs(t *testing.T) {
	libpath := checkLibraryExists(t)
	data, err := os.ReadFile("./tokenizer.json")
	require.NoError(t, err, "Failed to read tokenizer file")

	tok, err := FromBytes(data, WithLibraryPath(libpath))
	require.NoError(t, err)
	defer func() { _ = tok.Close() }()

	// First encode something
	text := "Hello world"
	encoding, err := tok.Encode(text)
	require.NoError(t, err)
	require.NotNil(t, encoding)
	require.Greater(t, len(encoding.IDs), 0)

	// Now decode the IDs
	decoded, err := tok.Decode(encoding.IDs, false)
	assert.NoError(t, err)
	assert.NotEmpty(t, decoded)

	// With skip special tokens
	decodedSkip, err := tok.Decode(encoding.IDs, true)
	assert.NoError(t, err)
	assert.NotEmpty(t, decodedSkip)
}