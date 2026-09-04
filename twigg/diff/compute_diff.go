package diff

import (
	"bytes"
	"crypto/sha256"
	"io"
	"math"
)

// Works ok if either are nil
func isEqual(content1, content2 io.WriterTo) bool {
	if content1 == nil {
		return content2 == nil
	}
	if content2 == nil {
		return content1 == nil
	}
	hash1 := sha256.New()
	_, err := content1.WriteTo(hash1)
	if err != nil {
		panic("could not write hash1")
	}
	hash2 := sha256.New()
	_, err = content2.WriteTo(hash2)
	if err != nil {
		panic("could not write hash2")
	}
	return bytes.Equal(hash1.Sum(nil), hash2.Sum(nil))
}

// Returns (v2 - v1). Returns nil if files are the same.
func computeTextDiff(v2 []byte, v2Name string,
	v1 []byte, v1Name string) (b []byte, nAdded int64, nRemoved int64, nChanged int64) {

	// The internalDiff implementation didn't provide a way to change
	// the number of context lines. My hacky solution for now was
	// to just pass the number of context lines as parameters to the
	// internalDiff (it used to be a const), and we use MaxUint32 for that
	// value now. This will should work ok without requiring me to
	// change the code much. This should also not cause a problem
	// because if there actually is a file with MaxUint32 lines,
	// it'll be considered large anyway, so the diff won't be computed
	return internalDiff(
		v1Name,
		v1,
		v2Name,
		v2,
		math.MaxUint32)
}
