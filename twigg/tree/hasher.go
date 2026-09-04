package tree

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
)

type hasher struct {
	h hash.Hash
}

func newHasher() hasher { return hasher{h: sha256.New()} }

func (h hasher) Write(b []byte) (int, error) {
	n, err := h.h.Write(b)
	if err != nil {
		panic(fmt.Sprintf("got error when writing to sha256 hash: %s", err))
	}
	return n, nil
}
func (h hasher) WriteString(s string) Hasher {
	io.WriteString(h.h, s)
	return h
}
func (h hasher) WriteBytes(b []byte) Hasher {
	h.h.Write(b)
	return h
}
func (h hasher) WriteBool(b bool) Hasher {
	if b {
		h.h.Write([]byte{1})
	} else {
		h.h.Write([]byte{0})
	}
	return h
}
func (h hasher) WriteSum(b [32]byte) Hasher {
	h.h.Write(b[:])
	return h
}
func (h hasher) Sum() [32]byte {
	var v [32]byte
	sum := h.h.Sum(nil)
	n := copy(v[:], sum)
	if n != 32 {
		panic("decoding hash to [32] failed")
	}
	return v
}
