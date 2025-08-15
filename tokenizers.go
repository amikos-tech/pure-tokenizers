package tokenizers

import (
	"fmt"
	"github.com/ebitengine/purego"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"
)

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
			return fmt.Errorf("library path cannot be empty")
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("shared library does not exist at path: %s", path)
		}
		t.LibraryPath = path
		return nil
	}
}

func WithTruncation(maxLen uintptr, direction TruncationDirection, strategy TruncationStrategy) TokenizerOption {
	return func(t *Tokenizer) error {
		if maxLen == 0 {
			return fmt.Errorf("truncation max length must be greater than 0")
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

func WithDownloadLibrary() TokenizerOption {
	return func(t *Tokenizer) error {
		// Set library path to cache location - download will happen automatically in LoadTokenizerLibrary
		cacheDir := getCacheDir()
		t.LibraryPath = filepath.Join(cacheDir, getLibraryName())
		return nil
	}
}

type Tokenizer struct {
	LibraryPath         string // Path to the shared library
	libh                uintptr
	tokenizerh          unsafe.Pointer // Pointer to the tokenizer instance
	fromFile            func(config string) unsafe.Pointer
	fromBytes           func(config []byte, bytesLen uint32, opts *TokenizerOptions) unsafe.Pointer
	encode              func(ptr unsafe.Pointer, message string, options *EncodeOptions) Buffer
	freeBuffer          func(buffer Buffer)
	freeTokenizer       func(ptr unsafe.Pointer)
	freeString          func(ptr *string)
	decode              func(ptr unsafe.Pointer, ids *uint32, len uint32, skipSpecialTokens bool) *string
	vocabSize           func(ptr unsafe.Pointer) uint32
	defaultEncodingOpts EncodeOptions
	TruncationEnabled   bool
	TruncationDirection TruncationDirection
	TruncationStrategy  TruncationStrategy
	TruncationMaxLength uintptr // Maximum length for truncation
	PaddingEnabled      bool
	PaddingStrategy     PaddingStrategy // Strategy for padding

}

const LibName = "tokenizers"

func getLibraryName() string {
	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf("lib%s.dll", LibName)
	case "darwin":
		return fmt.Sprintf("lib%s.dylib", LibName)
	default: // linux and others
		return fmt.Sprintf("lib%s.so", LibName)
	}
}

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

func LoadTokenizerLibrary(userProvidedPath string) (uintptr, error) {
	// 1. Check explicit user path
	if userProvidedPath != "" {
		if _, err := os.Stat(userProvidedPath); os.IsNotExist(err) {
			return 0, fmt.Errorf("shared library does not exist at user-provided path: %s", userProvidedPath)
		}
		if lib, err := loadLibrary(userProvidedPath); err == nil {
			return lib, nil
		} else {
			return 0, fmt.Errorf("failed to load library from user-provided path: %w", err)
		}
	}

	// 2. Check environment variable
	if envPath := os.Getenv("TOKENIZERS_LIB_PATH"); envPath != "" {
		if lib, err := loadLibrary(envPath); err == nil {
			return lib, nil
		}
	}

	// 3. Try cache location
	cacheDir := getCacheDir()
	cachedPath := filepath.Join(cacheDir, getLibraryName())

	if isLibraryValid(cachedPath) {
		if lib, err := loadLibrary(cachedPath); err == nil {
			return lib, nil
		}
	}

	// 4. Download to cache and load
	if err := downloadLibrary(cachedPath); err != nil {
		return 0, fmt.Errorf("failed to download library: %w", err)
	}

	return loadLibrary(cachedPath)
}

func FromFile(configFile string, opts ...TokenizerOption) (*Tokenizer, error) {
	if configFile == "" {
		return nil, fmt.Errorf("config file path cannot be empty")
	}
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file does not exist at path: %s", configFile)
	}
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	return FromBytes(data, opts...)
}

func FromBytes(config []byte, opts ...TokenizerOption) (*Tokenizer, error) {

	tokenizer := &Tokenizer{
		defaultEncodingOpts: EncodeOptions{
			ReturnTokens: true,
		},
	}
	for _, opt := range opts {
		if err := opt(tokenizer); err != nil {
			return nil, fmt.Errorf("failed to apply tokenizer option: %w", err)
		}
	}
	libh, err := LoadTokenizerLibrary(tokenizer.LibraryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load shared library: %w", err)
	}
	tokenizer.libh = libh
	purego.RegisterLibFunc(&tokenizer.fromFile, tokenizer.libh, "from_file")
	purego.RegisterLibFunc(&tokenizer.fromBytes, tokenizer.libh, "from_bytes")
	//purego.RegisterLibFunc(&tokenizer.fromBytesWithTruncation, tokenizer.libh, "from_bytes_with_truncation")
	purego.RegisterLibFunc(&tokenizer.encode, tokenizer.libh, "encode")
	purego.RegisterLibFunc(&tokenizer.freeBuffer, tokenizer.libh, "free_buffer")
	purego.RegisterLibFunc(&tokenizer.freeTokenizer, tokenizer.libh, "free_tokenizer")
	purego.RegisterLibFunc(&tokenizer.freeString, tokenizer.libh, "free_string")
	purego.RegisterLibFunc(&tokenizer.decode, tokenizer.libh, "decode")
	purego.RegisterLibFunc(&tokenizer.vocabSize, tokenizer.libh, "vocab_size")

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
	tokenizer.tokenizerh = tokenizer.fromBytes(config, uint32(len(config)), tOpts)

	if tokenizer.tokenizerh == nil {
		return nil, fmt.Errorf("failed to initialize tokenizer")
	}
	return tokenizer, nil
}
func (t *Tokenizer) Close() error {
	if t.freeTokenizer != nil && t.tokenizerh != nil {
		t.freeTokenizer(t.tokenizerh)
		t.tokenizerh = nil
	}
	err := purego.Dlclose(t.libh)
	if err != nil {
		return fmt.Errorf("failed to close shared library: %w", err)
	}
	return nil
}

func (t *Tokenizer) Encode(message string, opts ...EncodeOption) (*EncodeResult, error) {
	if t.encode == nil || t.tokenizerh == nil {
		return nil, fmt.Errorf("encode function is not initialized or tokenizer is not loaded")
	}
	options := t.defaultEncodingOpts
	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, fmt.Errorf("failed to apply encoding option: %w", err)
		}
	}
	buff := t.encode(t.tokenizerh, message, &options)
	result := &EncodeResult{}
	if buff.IDs != nil {
		result.IDs = unsafe.Slice(buff.IDs, buff.Len)
	}
	if buff.TypeIDs != nil {
		result.TypeIDs = unsafe.Slice(buff.TypeIDs, buff.Len)
	}
	specialTokensMask, attentionMask := MasksFromBuf(buff)
	if specialTokensMask != nil {
		result.SpecialTokensMask = specialTokensMask
	}
	if attentionMask != nil {
		result.AttentionMask = attentionMask
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
		return "", fmt.Errorf("decode function is not initialized or tokenizer is not loaded")
	}
	idsPtr := (*uint32)(unsafe.Pointer(&ids[0]))
	idLen := uint32(len(ids))
	resultPtr := t.decode(t.tokenizerh, idsPtr, idLen, skipSpecialTokens)
	if resultPtr == nil {
		return "", fmt.Errorf("failed to decode ids, result pointer is nil")
	}
	defer t.freeString(resultPtr)
	result := (*string)(unsafe.Pointer(resultPtr))
	if result == nil {
		return "", fmt.Errorf("failed to decode ids, result is nil")
	}
	return *result, nil

}
func (t *Tokenizer) VocabSize() (uint32, error) {
	if t.vocabSize == nil || t.tokenizerh == nil {
		return 0, fmt.Errorf("vocabSize function is not initialized or tokenizer is not loaded")
	}
	return t.vocabSize(t.tokenizerh), nil
}

//
//func Init() {
//	arcjetlib, err := purego.Dlopen("target/debug/libtokenizers.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
//
//	if err != nil {
//		// Handle error
//		fmt.Println("Error loading library:", err)
//	}
//	defer purego.Dlclose(arcjetlib)
//
//	var fromFile func(config string) unsafe.Pointer
//	var encode func(ptr unsafe.Pointer, message string, options *EncodeOptions) Buffer
//	var freeBuffer func(buffer Buffer)
//	purego.RegisterLibFunc(&fromFile, arcjetlib, "from_file")
//	purego.RegisterLibFunc(&encode, arcjetlib, "encode")
//	purego.RegisterLibFunc(&freeBuffer, arcjetlib, "free_buffer")
//	//cURLPtr1, cleanup1 := CString("./tokenizer.json") // Convert Go string to C ptr
//	//defer cleanup1()
//	libh := fromFile("./tokenizer.json")
//	fmt.Println(libh)
//	opts := EncodeOptions{
//		AddSpecialTokens:        true,
//		ReturnTypeIDs:           true,
//		ReturnTokens:            true,
//		ReturnSpecialTokensMask: true,
//		ReturnAttentionMask:     true,
//		ReturnOffsets:           true,
//	}
//	buff := encode(libh, "Hello world", &opts)
//	fmt.Println(buff)
//
//	idsSlice := unsafe.Slice((*int32)(unsafe.Pointer(buff.IDs)), buff.Len)
//	fmt.Println(idsSlice)
//	typeIDsSlice := unsafe.Slice((*int32)(unsafe.Pointer(buff.TypeIDs)), buff.Len)
//	fmt.Println(typeIDsSlice)
//	var tokens = TokensFromBuf(buff)
//	fmt.Println(tokens)
//
//	specialTokensMask, attentionMask := MasksFromBuf(buff)
//	fmt.Println(specialTokensMask)
//	fmt.Println(attentionMask)
//	defer freeBuffer(buff)
//
//}
