package tokenizers

import (
	"os"
	"unsafe"

	"github.com/Masterminds/semver/v3"
	"github.com/ebitengine/purego"
	"github.com/pkg/errors"
)

const (
	SUCCESS                    = 0
	ErrInvalidUTF8             = -1
	ErrEncodingFailed          = -2
	ErrNullOutput              = -3
	ErrInvalidTokenizerRef     = -4
	ErrNullInput               = -5
	ErrTokenizerCreationFailed = -6
	ErrInvalidPath             = -7
	ErrFileNotFound            = -8
	ErrTruncationFailed        = -9
	ErrPaddingFailed           = -10
	ErrDecodeFailed            = -11
	ErrCStringConversionFailed = -12
	ErrInvalidIDs              = -13
	ErrInvalidOptions          = -14
)

// AbiCompatibilityConstraint defines the required version range for ABI compatibility.
// The library version from Cargo.toml is used as the ABI version.
// Update this constraint when making breaking changes to the FFI interface.
const AbiCompatibilityConstraint = "^0.1.x"

// result structs

type TokenizerResult struct {
	Tokenizer unsafe.Pointer
	ErrorCode int32
}

type StringResult struct {
	String    *string
	ErrorCode int32
}

type VocabSizeResult struct {
	VocabSize uint32
	ErrorCode int32
}

type TruncationDirection uint8

type TruncationStrategy uint8

const (
	TruncationDirectionLeft TruncationDirection = iota
	TruncationDirectionRight
)
const TruncationDirectionDefault TruncationDirection = TruncationDirectionRight
const (
	TruncationStrategyLongestFirst TruncationStrategy = iota
	TruncationStrategyOnlyFirst
	TruncationStrategyOnlySecond
)
const TruncationStrategyDefault TruncationStrategy = TruncationStrategyLongestFirst
const TruncationMaxLengthDefault uintptr = 512 // Default truncation length, can be overridden by user

type PaddingStrategyTag int

const (
	PaddingStrategyBatchLongest PaddingStrategyTag = iota
	PaddingStrategyFixed
)

type PaddingStrategy struct {
	Tag       PaddingStrategyTag
	FixedSize uintptr // Only valid if Tag == PaddingStrategyFixed
}

type EncodeOptions struct {
	AddSpecialTokens        bool
	ReturnTypeIDs           bool
	ReturnTokens            bool
	ReturnSpecialTokensMask bool
	ReturnAttentionMask     bool
	ReturnOffsets           bool
}

type Buffer struct {
	IDs               *uint32
	TypeIDs           *uint32
	SpecialTokensMask *uint32
	AttentionMask     *uint32
	Tokens            **byte
	Offsets           *uintptr
	Len               uintptr
}

type EncodeResult struct {
	IDs               []uint32
	TypeIDs           []uint32
	SpecialTokensMask []uint32
	AttentionMask     []uint32
	Tokens            []string
	Offsets           []uint32
}

type TruncationOptions struct {
	Enabled   bool
	MaxLen    uintptr
	Strategy  TruncationStrategy
	Direction TruncationDirection
	Stride    uintptr
}
type PaddingOptions struct {
	Enabled  bool
	Strategy PaddingStrategy
}
type TokenizerOptions struct {
	AddSpecialTokens bool
	Trunc            TruncationOptions
	Pad              PaddingOptions
}

type EncodeOption func(eo *EncodeOptions) error

func WithReturnAllAttributes() EncodeOption {
	return func(eo *EncodeOptions) error {
		eo.ReturnTypeIDs = true
		eo.ReturnSpecialTokensMask = true
		eo.ReturnAttentionMask = true
		eo.ReturnTokens = true
		eo.ReturnOffsets = true
		eo.AddSpecialTokens = true
		return nil
	}
}

func WithReturnTypeIDs() EncodeOption {
	return func(eo *EncodeOptions) error {
		eo.ReturnTypeIDs = true
		return nil
	}
}

func WithAddSpecialTokens() EncodeOption {
	return func(eo *EncodeOptions) error {
		eo.AddSpecialTokens = true
		return nil
	}
}

func WithReturnSpecialTokensMask() EncodeOption {
	return func(eo *EncodeOptions) error {
		eo.ReturnSpecialTokensMask = true
		return nil
	}
}

func WithReturnTokens() EncodeOption {
	return func(eo *EncodeOptions) error {
		eo.ReturnTokens = true
		return nil
	}
}

func WithReturnAttentionMask() EncodeOption {
	return func(eo *EncodeOptions) error {
		eo.ReturnAttentionMask = true
		return nil
	}
}

func WithReturnOffsets() EncodeOption {
	return func(eo *EncodeOptions) error {
		eo.ReturnOffsets = true
		return nil
	}
}

type TokenizerOption func(t *Tokenizer) error

// WithLibraryPath sets the path to the shared library for the tokenizer. This must be the path to the .so/dylib/dll file that contains the tokenizer implementation.
func WithLibraryPath(path string) TokenizerOption {
	return func(t *Tokenizer) error {
		if path == "" {
			return errors.New("library path cannot be empty")
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return errors.Errorf("shared library does not exist at path: %s", path)
		}
		t.LibraryPath = path
		return nil
	}
}

func WithTruncation(maxLen uintptr, direction TruncationDirection, strategy TruncationStrategy) TokenizerOption {
	return func(t *Tokenizer) error {
		if maxLen == 0 {
			return errors.New("truncation max length must be greater than 0")
		}
		t.TruncationEnabled = true
		t.TruncationMaxLength = maxLen
		t.TruncationDirection = direction
		t.TruncationStrategy = strategy
		return nil
	}
}

func WithPadding(enabled bool, strategy PaddingStrategy) TokenizerOption {
	return func(t *Tokenizer) error {
		t.PaddingEnabled = enabled
		t.PaddingStrategy = strategy
		return nil
	}
}

type Tokenizer struct {
	LibraryPath         string // Path to the shared library
	libh                uintptr
	tokenizerh          unsafe.Pointer // Pointer to the tokenizer instance
	fromFile            func(config string, result *TokenizerResult) int32
	fromBytes           func(config []byte, bytesLen uint32, opts *TokenizerOptions, result *TokenizerResult) int32
	encode              func(ptr unsafe.Pointer, message string, options *EncodeOptions, buffer *Buffer) int32
	freeTokenizer       func(ptr unsafe.Pointer)
	freeBuffer          func(buffer *Buffer)
	freeString          func(ptr unsafe.Pointer)
	decode              func(ptr unsafe.Pointer, ids *uint32, len uint32, skipSpecialTokens bool, result *unsafe.Pointer) int32
	vocabSize           func(ptr unsafe.Pointer, size *uint32) int32
	getVersion          func() string
	defaultEncodingOpts EncodeOptions
	TruncationEnabled   bool
	TruncationDirection TruncationDirection
	TruncationStrategy  TruncationStrategy
	TruncationMaxLength uintptr // Maximum length for truncation
	PaddingEnabled      bool
	PaddingStrategy     PaddingStrategy // Strategy for padding
	hfConfig            *HFConfig       // HuggingFace configuration

}

const LibName = "tokenizers"

func FromFile(configFile string, opts ...TokenizerOption) (*Tokenizer, error) {
	if configFile == "" {
		return nil, errors.New("config file path cannot be empty")
	}
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, errors.Errorf("config file does not exist at path: %s", configFile)
	} else if err != nil {
		return nil, errors.Wrapf(err, "failed to access config file: %s", configFile)
	}
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read config file: %s", configFile)
	}
	return FromBytes(data, opts...)
}

func FromBytes(config []byte, opts ...TokenizerOption) (*Tokenizer, error) {

	tokenizer := &Tokenizer{
		defaultEncodingOpts: EncodeOptions{
			ReturnTokens: true,
		},
	}
	constraint, err := semver.NewConstraint(AbiCompatibilityConstraint)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse ABI version constraint: %s", AbiCompatibilityConstraint)
	}
	for _, opt := range opts {
		if err := opt(tokenizer); err != nil {
			return nil, errors.Wrapf(err, "failed to apply tokenizer option")
		}
	}
	libh, err := LoadTokenizerLibrary(tokenizer.LibraryPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load shared library")
	}
	tokenizer.libh = libh
	purego.RegisterLibFunc(&tokenizer.fromFile, tokenizer.libh, "from_file")
	purego.RegisterLibFunc(&tokenizer.fromBytes, tokenizer.libh, "from_bytes")
	purego.RegisterLibFunc(&tokenizer.encode, tokenizer.libh, "encode")
	purego.RegisterLibFunc(&tokenizer.freeBuffer, tokenizer.libh, "free_buffer")
	purego.RegisterLibFunc(&tokenizer.freeTokenizer, tokenizer.libh, "free_tokenizer")
	purego.RegisterLibFunc(&tokenizer.freeString, tokenizer.libh, "free_string")
	purego.RegisterLibFunc(&tokenizer.decode, tokenizer.libh, "decode")
	purego.RegisterLibFunc(&tokenizer.vocabSize, tokenizer.libh, "vocab_size")
	purego.RegisterLibFunc(&tokenizer.getVersion, tokenizer.libh, "get_version")

	// Initialize library version for HuggingFace User-Agent
	if tokenizer.getVersion != nil {
		SetLibraryVersion(tokenizer.getVersion())
	}

	tOpts := &TokenizerOptions{}
	if tokenizer.TruncationEnabled {
		tOpts = &TokenizerOptions{
			AddSpecialTokens: tokenizer.defaultEncodingOpts.AddSpecialTokens,
			Trunc: TruncationOptions{
				Enabled:   tokenizer.TruncationEnabled,
				MaxLen:    tokenizer.TruncationMaxLength,
				Direction: tokenizer.TruncationDirection,
				Strategy:  tokenizer.TruncationStrategy,
			},
		}
	}
	if tokenizer.PaddingEnabled {
		tOpts.Pad = PaddingOptions{
			Enabled:  tokenizer.PaddingEnabled,
			Strategy: tokenizer.PaddingStrategy,
		}
	}
	var result TokenizerResult
	errCode := tokenizer.fromBytes(config, uint32(len(config)), tOpts, &result)
	if errCode != SUCCESS {
		lastError := getErrorForCode(errCode)
		return nil, errors.Wrapf(lastError, "failed to create tokenizer from bytes")
	}
	tokenizer.tokenizerh = result.Tokenizer

	if err = tokenizer.abiCheck(constraint); err != nil {
		defer func() {
			_ = tokenizer.Close()
		}()
		return nil, errors.Wrap(err, "failed to check tokenizer abi")
	}
	return tokenizer, nil
}

// abiCheck check the ABI version of the Rust lib to check for compatibility
func (t *Tokenizer) abiCheck(constraint *semver.Constraints) error {
	if constraint == nil {
		return errors.New("ABI version constraint cannot be nil")
	}

	// Get the library version for ABI compatibility checking
	if t.getVersion == nil {
		return errors.New("getVersion function is not initialized, cannot check compatibility")
	}
	versionStr := t.getVersion()

	// Update global library version for HuggingFace User-Agent
	SetLibraryVersion(versionStr)

	ver, err := semver.NewVersion(versionStr)
	if err != nil {
		return errors.Wrapf(err, "failed to parse version string: %s", versionStr)
	}

	if !constraint.Check(ver) {
		// Enhanced error message with guidance
		errMsg := errors.Errorf("tokenizer library ABI version %s is not compatible with required version constraint %s", versionStr, constraint.String())
		return errors.Wrap(errMsg, `
To resolve this issue:
1. Set TOKENIZERS_LIB_PATH to a compatible library version
2. Clear the library cache and re-download: rm -rf ~/.cache/tokenizers/lib
3. Use a compatible library version by setting TOKENIZERS_VERSION environment variable`)
	}
	return nil
}

func (t *Tokenizer) Close() error {
	if t.tokenizerh != nil {
		t.freeTokenizer(t.tokenizerh)
		t.tokenizerh = nil
	}
	err := closeLibrary(t.libh)
	if err != nil {
		return errors.Errorf("failed to close shared library: %s", err.Error())
	}
	return nil
}

func (t *Tokenizer) Encode(message string, opts ...EncodeOption) (*EncodeResult, error) {
	if t.encode == nil || t.tokenizerh == nil {
		return nil, errors.New("encode function is not initialized or tokenizer is not loaded")
	}
	options := t.defaultEncodingOpts
	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, errors.Wrap(err, "failed to apply encoding option")
		}
	}
	var buff Buffer
	rc := t.encode(t.tokenizerh, message, &options, &buff)
	if rc < 0 {
		lastError := getErrorForCode(rc)
		return nil, errors.Wrap(lastError, "failed to encode message")
	}
	defer func() {
		t.freeBuffer(&buff)
	}()
	result := &EncodeResult{}
	if buff.IDs != nil {
		result.IDs = append([]uint32(nil), unsafe.Slice(buff.IDs, buff.Len)...)
	}
	if buff.TypeIDs != nil {
		result.TypeIDs = append([]uint32(nil), unsafe.Slice(buff.TypeIDs, buff.Len)...)
	}
	specialTokensMask, attentionMask := MasksFromBuf(buff)
	if specialTokensMask != nil {
		result.SpecialTokensMask = make([]uint32, 0, len(specialTokensMask))
		result.SpecialTokensMask = append(result.SpecialTokensMask, specialTokensMask...)
	}
	if attentionMask != nil {
		result.AttentionMask = make([]uint32, 0, len(attentionMask))
		result.AttentionMask = append(result.AttentionMask, attentionMask...)
	}
	result.Tokens = TokensFromBuf(buff)
	if buff.Offsets != nil {
		offsets := unsafe.Slice((*[2]uint)(unsafe.Pointer(buff.Offsets)), buff.Len)
		result.Offsets = make([]uint32, 0, len(offsets)*2)
		for _, offset := range offsets {
			result.Offsets = append(result.Offsets, uint32(offset[0]), uint32(offset[1]))
		}
	}

	return result, nil
}

func (t *Tokenizer) Decode(ids []uint32, skipSpecialTokens bool) (string, error) {
	if t.decode == nil || t.tokenizerh == nil {
		return "", errors.New("decode function is not initialized or tokenizer is not loaded")
	}

	// Handle empty IDs slice
	if len(ids) == 0 {
		return "", nil
	}

	idsPtr := (*uint32)(unsafe.Pointer(&ids[0]))
	idLen := uint32(len(ids))
	var cStrPtr unsafe.Pointer
	errCode := t.decode(t.tokenizerh, idsPtr, idLen, skipSpecialTokens, &cStrPtr)
	if errCode != SUCCESS {
		lastError := getErrorForCode(errCode)
		return "", errors.Wrapf(lastError, "failed to decode ids")
	}

	if cStrPtr == nil {
		return "", errors.New("decode returned null pointer")
	}

	// Convert C string to Go string
	// We need to manually convert from C string to Go string
	result := goStringFromPtr(cStrPtr)

	// Free the C string
	t.freeString(cStrPtr)

	return result, nil
}

// goStringFromPtr converts a C string to a Go string
func goStringFromPtr(ptr unsafe.Pointer) string {
	if ptr == nil {
		return ""
	}
	// Calculate string length
	p := (*byte)(ptr)
	n := 0
	for *(*byte)(unsafe.Pointer(uintptr(ptr) + uintptr(n))) != 0 {

		n++
	}
	// Create a Go string from the bytes
	return string(unsafe.Slice(p, n))
}
func (t *Tokenizer) VocabSize() (uint32, error) {
	if t.vocabSize == nil || t.tokenizerh == nil {
		return 0, errors.New("vocabSize function is not initialized or tokenizer is not loaded")
	}
	var size uint32
	errCode := t.vocabSize(t.tokenizerh, &size)
	if errCode != SUCCESS {
		lastError := getErrorForCode(errCode)
		return 0, errors.Wrapf(lastError, "failed to get vocab size")
	}
	return size, nil
}

func getErrorForCode(errCode int32) error {
	if errCode == SUCCESS {
		return nil // No error
	}
	switch errCode {
	case ErrInvalidUTF8:
		return errors.New("invalid UTF-8 in input message")
	case ErrEncodingFailed:
		return errors.New("tokenization failed")
	case ErrNullOutput:
		return errors.New("internal error: null output buffer")
	case ErrInvalidTokenizerRef:
		return errors.New("invalid tokenizer reference")
	case ErrNullInput:
		return errors.New("null input provided")
	case ErrTokenizerCreationFailed:
		return errors.New("failed to create tokenizer instance")
	case ErrInvalidPath:
		return errors.New("invalid file path provided")
	case ErrFileNotFound:
		return errors.New("file not found at specified path")
	case ErrTruncationFailed:
		return errors.New("truncation failed")
	case ErrPaddingFailed:
		return errors.New("padding failed")
	case ErrDecodeFailed:
		return errors.New("decoding failed")
	case ErrCStringConversionFailed:
		return errors.New("C string conversion failed")
	case ErrInvalidIDs:
		return errors.New("invalid IDs provided for decoding")
	case ErrInvalidOptions:
		return errors.New("invalid options provided for encoding/decoding")
	default:
		return errors.Errorf("unknown error code: %d", errCode)
	}
}

// GetLibraryVersion returns the version of the tokenizer library
func (t *Tokenizer) GetLibraryVersion() string {
	if t.getVersion == nil {
		return "unknown"
	}
	return t.getVersion()
}
