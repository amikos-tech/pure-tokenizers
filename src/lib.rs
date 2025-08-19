mod build;

use std::ffi::CStr;
use std::path::PathBuf;
use std::ptr;
use tokenizers::tokenizer::Tokenizer;
use tokenizers::{PaddingParams, PaddingStrategy, TruncationStrategy};

// At the top of lib.rs
const ERROR_INVALID_UTF8: i32 = -1;
const ERROR_ENCODING_FAILED: i32 = -2;
const ERROR_NULL_OUTPUT: i32 = -3;
const ERROR_INVALID_TOKENIZER_REF: i32 = -4;

#[repr(C)]
pub struct TruncationOptions {
    enabled: bool,
    max_len: usize,
    strategy: u8, // 0 = LongestFirst, 1 = OnlyFirst, 2 = OnlySecond (match tokenizers' strategies if you expose more)
    direction: u8, // 0 = Left, 1 = Right, 2 = LongestFirst (match tokenizers' strategies if you expose more)
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
#[allow(clippy::missing_safety_doc)]
#[no_mangle]
pub unsafe extern "C" fn from_bytes(
    bytes: *const u8,
    len: u32,
    opts: *const TokenizerOptions,
) -> *mut Tokenizer {
    if bytes.is_null() || opts.is_null() {
        return std::ptr::null_mut();
    }

    let bytes_slice = unsafe { std::slice::from_raw_parts(bytes, len as usize) };
    let mut tok = match Tokenizer::from_bytes(bytes_slice) {
        Ok(t) => t,
        Err(_) => return std::ptr::null_mut(),
    };

    let opts = unsafe { &*opts };
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
        tok.with_truncation(Some(TruncationParams {
            direction: dir,
            max_length: opts.trunc.max_len,
            strategy: strat,
            stride: opts.trunc.stride,
        }))
        .unwrap();
    }
    if opts.pad.enabled {
        tok.with_padding(Some(PaddingParams {
            strategy: opts.pad.strategy.clone(),
            ..Default::default()
        }));
    }

    Box::into_raw(Box::new(tok))
}

#[allow(clippy::missing_safety_doc)]
#[no_mangle]
pub unsafe extern "C" fn from_file(config: *const libc::c_char) -> *mut libc::c_void {
    let config_cstr = unsafe { CStr::from_ptr(config) };
    let config = config_cstr.to_str().unwrap();
    let config = PathBuf::from(config);
    match Tokenizer::from_file(config) {
        Ok(tokenizer) => {
            let ptr = Box::into_raw(Box::new(tokenizer));
            ptr.cast()
        }
        Err(_) => ptr::null_mut(),
    }
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
#[allow(clippy::missing_safety_doc)]
#[no_mangle]
pub unsafe extern "C" fn encode(
    ptr: *mut libc::c_void,
    message: *const libc::c_char,
    options: &EncodeOptions,
    out: *mut Buffer,
) -> i32 {
    let tokenizer: &Tokenizer = match ptr.cast::<Tokenizer>().as_ref() {
        Some(t) => t,
        None => return ERROR_INVALID_TOKENIZER_REF, // Return -4 if the tokenizer reference is invalid
    };
    let message_cstr = unsafe { CStr::from_ptr(message) };
    let message = message_cstr.to_str();
    if message.is_err() {
        return ERROR_INVALID_UTF8; // Return -1 if the message cannot be converted to a string
    }

    let encoding = match tokenizer.encode(message.unwrap(), options.add_special_tokens) {
        Ok(enc) => enc,
        Err(_) => return ERROR_ENCODING_FAILED,
    };
    let mut vec_ids = encoding.get_ids().to_vec();
    vec_ids.shrink_to_fit();
    let ids = vec_ids.as_mut_ptr();
    let len = vec_ids.len();
    std::mem::forget(vec_ids);

    let mut type_ids: *mut u32 = ptr::null_mut();
    if options.return_type_ids {
        let mut vec_type_ids = encoding.get_type_ids().to_vec();
        vec_type_ids.shrink_to_fit();
        type_ids = vec_type_ids.as_mut_ptr();
        std::mem::forget(vec_type_ids);
    }

    let mut tokens: *mut *mut libc::c_char = ptr::null_mut();
    if options.return_tokens {
        let mut vec_tokens = encoding
            .get_tokens()
            .iter()
            .map(|s| std::ffi::CString::new(s.as_str()).unwrap().into_raw())
            .collect::<Vec<_>>();
        vec_tokens.shrink_to_fit();
        tokens = vec_tokens.as_mut_ptr();
        std::mem::forget(vec_tokens);
    }

    let mut special_tokens_mask: *mut u32 = ptr::null_mut();
    if options.return_special_tokens_mask {
        let mut vec_special_tokens_mask = encoding.get_special_tokens_mask().to_vec();
        vec_special_tokens_mask.shrink_to_fit();
        special_tokens_mask = vec_special_tokens_mask.as_mut_ptr();
        std::mem::forget(vec_special_tokens_mask);
    }

    let mut attention_mask: *mut u32 = ptr::null_mut();
    if options.return_attention_mask {
        let mut vec_attention_mask = encoding.get_attention_mask().to_vec();
        vec_attention_mask.shrink_to_fit();
        attention_mask = vec_attention_mask.as_mut_ptr();
        std::mem::forget(vec_attention_mask);
    }

    let mut offsets: *mut usize = ptr::null_mut();
    if options.return_offsets {
        let vec_offsets_tuples = encoding.get_offsets().to_vec();
        let mut vec_offsets = Vec::with_capacity(vec_offsets_tuples.len() * 2);
        for i in vec_offsets_tuples {
            vec_offsets.push(i.0);
            vec_offsets.push(i.1);
        }
        vec_offsets.shrink_to_fit();
        offsets = vec_offsets.as_mut_ptr();
        std::mem::forget(vec_offsets);
    }
    if !out.is_null() {
        *out = Buffer {
            ids,
            type_ids,
            special_tokens_mask,
            attention_mask,
            tokens,
            offsets,
            len,
        };
    } else {
        // If out is null, we cannot return the results, so we return an error code
        return ERROR_NULL_OUTPUT; // Return -3 if the output buffer is null
    }
    0
}
/// # Safety
/// The caller must ensure that `ptr` is a valid pointer to a heap-allocated Tokenizer object
/// previously created by `from_bytes` or `from_file`, and must not use `ptr` after calling this function.
/// The `ids` pointer must point to a valid slice of `u32` integers,
/// and `len` must be the length of that slice.
/// The `skip_special_tokens` parameter indicates whether to skip special tokens during decoding.
/// The function returns a pointer to a heap-allocated C string containing the decoded text,
/// or `null` if decoding fails or if the input is invalid.
/// The caller is responsible for freeing the returned string using `free_string`.
/// The function may panic if the input is invalid or if the tokenizer cannot decode the input.
/// The caller should handle the potential panic by ensuring that the input is valid and that the tokenizer
/// is properly initialized before calling this function.
#[no_mangle]
pub unsafe extern "C" fn decode(
    ptr: *mut libc::c_void,
    ids: *const u32,
    len: u32,
    skip_special_tokens: bool,
) -> *mut libc::c_char {
    let tokenizer: &Tokenizer;
    unsafe {
        tokenizer = ptr
            .cast::<Tokenizer>()
            .as_ref()
            .expect("failed to cast tokenizer");
    }
    let ids_slice = unsafe { std::slice::from_raw_parts(ids, len as usize) };

    let string = tokenizer
        .decode(ids_slice, skip_special_tokens)
        .expect("failed to decode input");
    match std::ffi::CString::new(string) {
        Ok(c_string) => c_string.into_raw(),
        Err(_) => ptr::null_mut(),
    }
}

#[no_mangle]
pub extern "C" fn vocab_size(ptr: *mut libc::c_void) -> u32 {
    let tokenizer: &Tokenizer;
    unsafe {
        tokenizer = ptr
            .cast::<Tokenizer>()
            .as_ref()
            .expect("failed to cast tokenizer");
    }
    tokenizer.get_vocab_size(true) as u32
}

#[no_mangle]
pub extern "C" fn free_tokenizer(ptr: *mut ::libc::c_void) {
    if ptr.is_null() {
        return;
    }
    unsafe {
        drop(Box::from_raw(ptr.cast::<Tokenizer>()));
    }
}

/// # Safety
/// The caller must ensure that `buf` is a valid pointer to a `Buffer` struct
/// previously returned by the `encode` function.
#[no_mangle]
pub unsafe extern "C" fn free_buffer(buf: *mut Buffer) {
    if buf.is_null() {
        return;
    }
    let buf = &mut *buf;
    // Free the memory allocated for the fields in the Buffer struct
    if !buf.ids.is_null() {
        unsafe {
            Vec::from_raw_parts(buf.ids, buf.len, buf.len);
        }
    }
    if !buf.type_ids.is_null() {
        unsafe {
            Vec::from_raw_parts(buf.type_ids, buf.len, buf.len);
        }
    }
    if !buf.special_tokens_mask.is_null() {
        unsafe {
            Vec::from_raw_parts(buf.special_tokens_mask, buf.len, buf.len);
        }
    }
    if !buf.attention_mask.is_null() {
        unsafe {
            Vec::from_raw_parts(buf.attention_mask, buf.len, buf.len);
        }
    }
    if !buf.offsets.is_null() {
        unsafe {
            Vec::from_raw_parts(buf.offsets, buf.len * 2, buf.len * 2);
        }
    }
    if !buf.tokens.is_null() {
        unsafe {
            let strings = Vec::from_raw_parts(buf.tokens, buf.len, buf.len);
            for s in strings {
                drop(std::ffi::CString::from_raw(s.cast::<libc::c_char>()));
            }
        }
    }
}
/// # Safety
/// The caller must ensure that `ptr` is a valid pointer to a heap-allocated C string
/// previously returned by `CString::into_raw`, and must not use `ptr` after calling this function.
#[no_mangle]
pub unsafe extern "C" fn free_string(ptr: *mut libc::c_char) {
    if ptr.is_null() {
        return;
    }
    unsafe {
        drop(std::ffi::CString::from_raw(ptr));
    }
}
