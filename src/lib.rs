mod build;

use std::ffi::CStr;
use std::path::PathBuf;
use std::ptr;
use tokenizers::tokenizer::Tokenizer;
use tokenizers::{PaddingParams, PaddingStrategy, TruncationStrategy};

// Error codes - expanded for better error reporting
const SUCCESS: i32 = 0;
const ERROR_INVALID_UTF8: i32 = -1;
const ERROR_ENCODING_FAILED: i32 = -2;
const ERROR_NULL_OUTPUT: i32 = -3;
const ERROR_INVALID_TOKENIZER_REF: i32 = -4;
const ERROR_NULL_INPUT: i32 = -5;
const ERROR_TOKENIZER_CREATION_FAILED: i32 = -6;
const ERROR_INVALID_PATH: i32 = -7;
const ERROR_FILE_NOT_FOUND: i32 = -8;
const ERROR_TRUNCATION_FAILED: i32 = -9;
const ERROR_PADDING_FAILED: i32 = -10;
const ERROR_DECODE_FAILED: i32 = -11;
const ERROR_CSTRING_CONVERSION_FAILED: i32 = -12;
const ERROR_INVALID_IDS: i32 = -13;
const ERROR_INVALID_OPTIONS: i32 = -14;

#[repr(C)]
pub struct TruncationOptions {
    enabled: bool,
    max_len: usize,
    strategy: u8,  // 0 = LongestFirst, 1 = OnlyFirst, 2 = OnlySecond
    direction: u8, // 0 = Left, 1 = Right, 2 = LongestFirst
    stride: usize,
}

#[repr(C)]
pub struct PaddingOptions {
    enabled: bool,
    strategy: PaddingStrategy,
}

#[repr(C)]
pub struct TokenizerOptions {
    add_special_tokens: bool,
    trunc: TruncationOptions,
    pad: PaddingOptions,
}

#[repr(C)]
pub struct Buffer {
    ids: *mut u32,
    type_ids: *mut u32,
    special_tokens_mask: *mut u32,
    attention_mask: *mut u32,
    tokens: *mut *mut libc::c_char,
    offsets: *mut usize,
    len: usize,
}

#[repr(C)]
pub struct EncodeOptions {
    add_special_tokens: bool,
    return_type_ids: bool,
    return_tokens: bool,
    return_special_tokens_mask: bool,
    return_attention_mask: bool,
    return_offsets: bool,
}

// Result structures for functions that can fail
#[repr(C)]
pub struct TokenizerResult {
    tokenizer: *mut Tokenizer,
}

#[repr(C)]
pub struct StringResult {
    string: *mut libc::c_char,
}

#[repr(C)]
pub struct VocabSizeResult {
    size: u32,
}

/// Creates a tokenizer from bytes with the given options.
///
/// # Safety
///
/// - `bytes` must be a valid pointer to at least `len` bytes
/// - `opts` must be a valid pointer to a `TokenizerOptions` struct
/// - The memory pointed to by `bytes` must remain valid for the duration of this call
/// - The returned tokenizer pointer must be freed using `free_tokenizer` when no longer needed
#[no_mangle]
pub unsafe extern "C" fn from_bytes(
    bytes: *const u8,
    len: u32,
    opts: *const TokenizerOptions,
    out: &mut TokenizerResult,
) -> i32 {
    if bytes.is_null() {
        return ERROR_NULL_OUTPUT;
    }

    if opts.is_null() {
        return ERROR_INVALID_OPTIONS;
    }

    let bytes_slice = std::slice::from_raw_parts(bytes, len as usize);
    let mut tok = match Tokenizer::from_bytes(bytes_slice) {
        Ok(t) => t,
        Err(_) => {
            return ERROR_TOKENIZER_CREATION_FAILED;
        }
    };

    let opts = &*opts;
    tok.set_encode_special_tokens(opts.add_special_tokens);

    if opts.trunc.enabled {
        use tokenizers::tokenizer::{TruncationDirection, TruncationParams};

        let dir = match opts.trunc.direction {
            0 => TruncationDirection::Left,
            1 => TruncationDirection::Right,
            _ => TruncationDirection::default(),
        };

        let strat = match opts.trunc.strategy {
            0 => TruncationStrategy::LongestFirst,
            1 => TruncationStrategy::OnlyFirst,
            2 => TruncationStrategy::OnlySecond,
            _ => TruncationStrategy::default(),
        };

        if tok
            .with_truncation(Some(TruncationParams {
                direction: dir,
                max_length: opts.trunc.max_len,
                strategy: strat,
                stride: opts.trunc.stride,
            }))
            .is_err()
        {
            return ERROR_TRUNCATION_FAILED;
        }
    }

    if opts.pad.enabled {
        tok.with_padding(Some(PaddingParams {
            strategy: opts.pad.strategy.clone(),
            ..Default::default()
        }));
    }

    let raw = Box::into_raw(Box::new(tok));
    out.tokenizer = raw;
    SUCCESS
}

/// Creates a tokenizer from a file path.
///
/// # Safety
///
/// - `config` must be a valid pointer to a null-terminated C string
/// - The returned tokenizer pointer must be freed using `free_tokenizer` when no longer needed
#[no_mangle]
pub unsafe extern "C" fn from_file(config: *const libc::c_char, out: &mut TokenizerResult) -> i32 {
    if config.is_null() {
        return ERROR_NULL_INPUT;
    }

    let config_cstr = CStr::from_ptr(config);
    let config_str = match config_cstr.to_str() {
        Ok(s) => s,
        Err(_) => {
            return ERROR_INVALID_PATH;
        }
    };

    let config_path = PathBuf::from(config_str);

    match Tokenizer::from_file(config_path) {
        Ok(tok) => {
            let raw = Box::into_raw(Box::new(tok));
            out.tokenizer = raw;
            SUCCESS
        }
        Err(e) => {
            // Try to determine if it's a file not found error
            if e.to_string().contains("No such file") || e.to_string().contains("not found") {
                ERROR_FILE_NOT_FOUND
            } else {
                ERROR_TOKENIZER_CREATION_FAILED
            }
        }
    }
}

/// Encodes a message using the tokenizer.
/// Returns 0 on success, negative error code on failure.
///
/// # Safety
///
/// - `ptr` must be a valid pointer to a `Tokenizer` created by `from_bytes` or `from_file`
/// - `message` must be a valid pointer to a null-terminated C string
/// - `options` must be a valid pointer to an `EncodeOptions` struct
/// - `out` must be a valid pointer to a `Buffer` struct that will receive the results
/// - The caller is responsible for freeing the buffer using `free_buffer`
#[no_mangle]
pub unsafe extern "C" fn encode(
    ptr: *mut Tokenizer,
    message: *const libc::c_char,
    options: *const EncodeOptions,
    out: *mut Buffer,
) -> i32 {
    if ptr.is_null() {
        return ERROR_INVALID_TOKENIZER_REF;
    }

    if message.is_null() {
        return ERROR_NULL_INPUT;
    }

    if options.is_null() {
        return ERROR_INVALID_OPTIONS;
    }

    if out.is_null() {
        return ERROR_NULL_OUTPUT;
    }

    let tokenizer: &Tokenizer = match ptr.as_ref() {
        Some(t) => t,
        None => return ERROR_INVALID_TOKENIZER_REF,
    };

    let message_cstr = CStr::from_ptr(message);
    let message_str = match message_cstr.to_str() {
        Ok(s) => s,
        Err(_) => return ERROR_INVALID_UTF8,
    };

    let options = &*options;

    let encoding = match tokenizer.encode(message_str, options.add_special_tokens) {
        Ok(enc) => enc,
        Err(_) => return ERROR_ENCODING_FAILED,
    };

    // Prepare IDs (always needed)
    let mut vec_ids = encoding.get_ids().to_vec();
    vec_ids.shrink_to_fit();
    let ids = vec_ids.as_mut_ptr();
    let len = vec_ids.len();
    std::mem::forget(vec_ids);

    // Prepare type IDs if requested
    let mut type_ids: *mut u32 = ptr::null_mut();
    if options.return_type_ids {
        let mut vec_type_ids = encoding.get_type_ids().to_vec();
        vec_type_ids.shrink_to_fit();
        type_ids = vec_type_ids.as_mut_ptr();
        std::mem::forget(vec_type_ids);
    }

    // Prepare tokens if requested
    let mut tokens: *mut *mut libc::c_char = ptr::null_mut();
    if options.return_tokens {
        let mut vec_tokens = Vec::with_capacity(encoding.get_tokens().len());
        for token in encoding.get_tokens() {
            match std::ffi::CString::new(token.as_str()) {
                Ok(cstr) => vec_tokens.push(cstr.into_raw()),
                Err(_) => {
                    // Clean up already allocated tokens
                    for allocated_token in vec_tokens {
                        drop(std::ffi::CString::from_raw(allocated_token));
                    }
                    return ERROR_CSTRING_CONVERSION_FAILED;
                }
            }
        }
        vec_tokens.shrink_to_fit();
        tokens = vec_tokens.as_mut_ptr();
        std::mem::forget(vec_tokens);
    }

    // Prepare special tokens mask if requested
    let mut special_tokens_mask: *mut u32 = ptr::null_mut();
    if options.return_special_tokens_mask {
        let mut vec_special_tokens_mask = encoding.get_special_tokens_mask().to_vec();
        vec_special_tokens_mask.shrink_to_fit();
        special_tokens_mask = vec_special_tokens_mask.as_mut_ptr();
        std::mem::forget(vec_special_tokens_mask);
    }

    // Prepare attention mask if requested
    let mut attention_mask: *mut u32 = ptr::null_mut();
    if options.return_attention_mask {
        let mut vec_attention_mask = encoding.get_attention_mask().to_vec();
        vec_attention_mask.shrink_to_fit();
        attention_mask = vec_attention_mask.as_mut_ptr();
        std::mem::forget(vec_attention_mask);
    }

    // Prepare offsets if requested
    let mut offsets: *mut usize = ptr::null_mut();
    if options.return_offsets {
        let vec_offsets_tuples = encoding.get_offsets().to_vec();
        let mut vec_offsets = Vec::with_capacity(vec_offsets_tuples.len() * 2);
        for (start, end) in vec_offsets_tuples {
            vec_offsets.push(start);
            vec_offsets.push(end);
        }
        vec_offsets.shrink_to_fit();
        offsets = vec_offsets.as_mut_ptr();
        std::mem::forget(vec_offsets);
    }

    *out = Buffer {
        ids,
        type_ids,
        special_tokens_mask,
        attention_mask,
        tokens,
        offsets,
        len,
    };

    SUCCESS
}

/// Decodes token IDs back to text.
///
/// # Safety
///
/// - `ptr` must be a valid pointer to a `Tokenizer` created by `from_bytes` or `from_file`
/// - `ids` must be a valid pointer to at least `len` `u32` values
/// - The returned string must be freed using `free_string` when no longer needed
#[no_mangle]
pub unsafe extern "C" fn decode(
    ptr: *mut Tokenizer,
    ids: *const u32,
    len: u32,
    skip_special_tokens: bool,
    out: *mut *mut libc::c_char, // <— pointer-to-pointer
) -> i32 {
    if ptr.is_null() {
        return ERROR_INVALID_TOKENIZER_REF;
    }
    if ids.is_null() || len == 0 {
        return ERROR_INVALID_IDS;
    }
    if out.is_null() {
        return ERROR_NULL_OUTPUT;
    }

    let tokenizer: &Tokenizer = match ptr.as_ref() {
        Some(t) => t,
        None => return ERROR_INVALID_TOKENIZER_REF,
    };

    let ids_slice = std::slice::from_raw_parts(ids, len as usize);
    let string = match tokenizer.decode(ids_slice, skip_special_tokens) {
        Ok(s) => s,
        Err(_) => return ERROR_DECODE_FAILED,
    };

    // CString::new fails if `string` contains interior NULs.
    let c_string = match std::ffi::CString::new(string) {
        Ok(s) => s,
        Err(_) => return ERROR_CSTRING_CONVERSION_FAILED,
    };

    // Transfer ownership to caller; they must free with `decode_free`.
    let raw = c_string.into_raw();
    ptr::write(out, raw);
    SUCCESS
}
/// # Safety
/// /// - `ptr` must be a valid pointer to a `Tokenizer` created by `from_bytes` or `from_file`
/// Gets the vocabulary size of the tokenizer.
#[no_mangle]
pub unsafe extern "C" fn vocab_size(ptr: *mut Tokenizer, out: *mut i32) -> i32 {
    if ptr.is_null() {
        return ERROR_INVALID_TOKENIZER_REF;
    }

    let tokenizer: &Tokenizer = match ptr.as_ref() {
        Some(t) => t,
        None => {
            return ERROR_INVALID_TOKENIZER_REF;
        }
    };
    let size = tokenizer.get_vocab_size(true) as i32;
    ptr::write(out, size);
    SUCCESS
}

/// Frees a tokenizer instance.
///
/// # Safety
///
/// - `ptr` must be either null or a valid pointer to a `Tokenizer` created by `from_bytes` or `from_file`
/// - After calling this function, the tokenizer is invalid and must not be used
/// - This function must only be called once per tokenizer
#[no_mangle]
pub unsafe extern "C" fn free_tokenizer(ptr: *mut Tokenizer) {
    if ptr.is_null() {
        return;
    }
    drop(Box::from_raw(ptr));
}

/// Frees a buffer returned by encode.
///
/// # Safety
///
/// - `buf` must be either null or a valid pointer to a `Buffer` previously returned by `encode`
/// - After calling this function, the buffer and all its contents are invalid and must not be used
/// - This function must only be called once per buffer
#[no_mangle]
pub unsafe extern "C" fn free_buffer(buf: *mut Buffer) {
    if buf.is_null() {
        return;
    }

    let buf = &mut *buf;

    // Free the memory allocated for the fields in the Buffer struct
    if !buf.ids.is_null() {
        drop(Vec::from_raw_parts(buf.ids, buf.len, buf.len));
    }

    if !buf.type_ids.is_null() {
        drop(Vec::from_raw_parts(buf.type_ids, buf.len, buf.len));
    }

    if !buf.special_tokens_mask.is_null() {
        drop(Vec::from_raw_parts(
            buf.special_tokens_mask,
            buf.len,
            buf.len,
        ));
    }

    if !buf.attention_mask.is_null() {
        drop(Vec::from_raw_parts(buf.attention_mask, buf.len, buf.len));
    }

    if !buf.offsets.is_null() {
        drop(Vec::from_raw_parts(buf.offsets, buf.len * 2, buf.len * 2));
    }

    if !buf.tokens.is_null() {
        let strings = Vec::from_raw_parts(buf.tokens, buf.len, buf.len);
        for s in strings {
            drop(std::ffi::CString::from_raw(s));
        }
    }
}

/// Frees a string returned by decode.
///
/// # Safety
///
/// - `ptr` must be either null or a valid pointer to a string previously returned by `decode`
/// - After calling this function, the string is invalid and must not be used
/// - This function must only be called once per string
#[no_mangle]
pub unsafe extern "C" fn free_string(ptr: *mut libc::c_char) {
    if ptr.is_null() {
        return;
    }
    drop(std::ffi::CString::from_raw(ptr));
}

/// Gets a human-readable error message for the given error code.
/// The returned string is static and should not be freed.
#[no_mangle]
pub extern "C" fn get_error_message(code: i32) -> *const libc::c_char {
    let message = match code {
        SUCCESS => "Success\0",
        ERROR_INVALID_UTF8 => "Invalid UTF-8 string\0",
        ERROR_ENCODING_FAILED => "Encoding failed\0",
        ERROR_NULL_OUTPUT => "Output buffer is null\0",
        ERROR_INVALID_TOKENIZER_REF => "Invalid tokenizer reference\0",
        ERROR_NULL_INPUT => "Input parameter is null\0",
        ERROR_TOKENIZER_CREATION_FAILED => "Failed to create tokenizer\0",
        ERROR_INVALID_PATH => "Invalid file path\0",
        ERROR_FILE_NOT_FOUND => "File not found\0",
        ERROR_TRUNCATION_FAILED => "Failed to set truncation parameters\0",
        ERROR_PADDING_FAILED => "Failed to set padding parameters\0",
        ERROR_DECODE_FAILED => "Decoding failed\0",
        ERROR_CSTRING_CONVERSION_FAILED => {
            "Failed to convert string to C string (contains null bytes)\0"
        }
        ERROR_INVALID_IDS => "Invalid or empty token IDs\0",
        ERROR_INVALID_OPTIONS => "Invalid options parameter\0",
        _ => "Unknown error\0",
    };

    message.as_ptr() as *const libc::c_char
}
