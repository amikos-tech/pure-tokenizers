package tokenizers

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestABIVersionChecking(t *testing.T) {
	tests := []struct {
		name           string
		abiVersion     string
		constraint     string
		shouldPass     bool
		expectedError  string
	}{
		{
			name:       "Compatible version - exact match",
			abiVersion: "0.1.0",
			constraint: "^0.1.x",
			shouldPass: true,
		},
		{
			name:       "Compatible version - patch version",
			abiVersion: "0.1.5",
			constraint: "^0.1.x",
			shouldPass: true,
		},
		{
			name:          "Incompatible version - major version",
			abiVersion:    "1.0.0",
			constraint:    "^0.1.x",
			shouldPass:    false,
			expectedError: "not compatible",
		},
		{
			name:          "Incompatible version - minor version",
			abiVersion:    "0.2.0",
			constraint:    "^0.1.x",
			shouldPass:    false,
			expectedError: "not compatible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constraint, err := semver.NewConstraint(tt.constraint)
			require.NoError(t, err, "Failed to create constraint")

			// Create a mock tokenizer with ABI version
			tokenizer := &Tokenizer{
				getAbiVersion: func() string {
					return tt.abiVersion
				},
			}

			err = tokenizer.abiCheck(constraint)

			if tt.shouldPass {
				assert.NoError(t, err, "Expected ABI check to pass")
			} else {
				assert.Error(t, err, "Expected ABI check to fail")
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			}
		})
	}
}

func TestABICheckWithFallback(t *testing.T) {
	constraint, err := semver.NewConstraint("^0.1.x")
	require.NoError(t, err, "Failed to create constraint")

	t.Run("Uses ABI version when available", func(t *testing.T) {
		tokenizer := &Tokenizer{
			getAbiVersion: func() string {
				return "0.1.0"
			},
			getVersion: func() string {
				return "1.0.0" // Different from ABI version
			},
		}

		err := tokenizer.abiCheck(constraint)
		assert.NoError(t, err, "Should use ABI version for compatibility check")
	})

	t.Run("Falls back to package version when ABI version not available", func(t *testing.T) {
		tokenizer := &Tokenizer{
			getAbiVersion: nil, // No ABI version function
			getVersion: func() string {
				return "0.1.0"
			},
		}

		err := tokenizer.abiCheck(constraint)
		assert.NoError(t, err, "Should fallback to package version")
	})

	t.Run("Returns error when neither version available", func(t *testing.T) {
		tokenizer := &Tokenizer{
			getAbiVersion: nil,
			getVersion:    nil,
		}

		err := tokenizer.abiCheck(constraint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "neither getAbiVersion nor getVersion")
	})
}

func TestABIErrorMessages(t *testing.T) {
	constraint, err := semver.NewConstraint("^0.1.x")
	require.NoError(t, err)

	tokenizer := &Tokenizer{
		getAbiVersion: func() string {
			return "0.2.0" // Incompatible version
		},
	}

	err = tokenizer.abiCheck(constraint)
	require.Error(t, err)

	// Check that error message includes helpful guidance
	errorMsg := err.Error()
	assert.Contains(t, errorMsg, "not compatible")
	assert.Contains(t, errorMsg, "TOKENIZERS_LIB_PATH")
	assert.Contains(t, errorMsg, "TOKENIZERS_VERSION")
}

func TestGetPlatformAssetNameForABI(t *testing.T) {
	// This test verifies that getPlatformAssetName returns
	// the correct asset name for the current platform
	assetName := getPlatformAssetName()

	// Asset name should contain platform identifier
	assert.NotEmpty(t, assetName)
	assert.Contains(t, assetName, "libtokenizers")
	assert.Contains(t, assetName, ".tar.gz")
}

func TestCacheDirCreation(t *testing.T) {
	cacheDir := getCacheDir()

	// Cache directory should be non-empty
	assert.NotEmpty(t, cacheDir)

	// Should contain tokenizers in the path
	assert.Contains(t, cacheDir, "tokenizers")
}