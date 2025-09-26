package tokenizers

import (
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
	if buf.AttentionMask != nil {
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
		b := unsafe.Slice((*byte)(q), n)
		out = append(out, string(b))
	}
	return out
}
