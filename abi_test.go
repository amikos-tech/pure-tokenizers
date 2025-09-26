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
			// Create a mock tokenizer with version
			tokenizer := &Tokenizer{
				getVersion: func() string {
					return tt.abiVersion
				},
			}

			constraint, _ := semver.NewConstraint(AbiCompatibilityConstraint)
			err := tokenizer.abiCheck(constraint)

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

func TestVersionCheck(t *testing.T) {
	t.Run("Uses version for compatibility check", func(t *testing.T) {
		tokenizer := &Tokenizer{
			getVersion: func() string {
				return "0.1.0"
			},
		}

		constraint, _ := semver.NewConstraint(AbiCompatibilityConstraint)
		err := tokenizer.abiCheck(constraint)
		assert.NoError(t, err, "Should use version for compatibility check")
	})

	t.Run("Returns error when version not available", func(t *testing.T) {
		tokenizer := &Tokenizer{
			getVersion: nil,
		}

		constraint, _ := semver.NewConstraint(AbiCompatibilityConstraint)
		err := tokenizer.abiCheck(constraint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "getVersion function is not initialized")
	})
}

func TestABIErrorMessages(t *testing.T) {
	tokenizer := &Tokenizer{
		getVersion: func() string {
			return "0.2.0" // Incompatible version
		},
	}

	constraint, _ := semver.NewConstraint(AbiCompatibilityConstraint)
	err := tokenizer.abiCheck(constraint)
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