package deltastream

// This package provides functions to compress/uncompress streaming data.
// It provides a Writer that automatically picks the best compression as you
// Write to it.

import (
	"io"
)

type CompressionMethod int

const (
	// go-delta binary diff
	CompressionMethodGodelta CompressionMethod = iota
	// Simple flate compression (non delta) with BestSpeed
	CompressionMethodSpeedFlate
)

type Writer interface {
	io.Writer

	// Panics if called before closing
	Data() CompressionData
}

type CompressionData struct {
	Method CompressionMethod
}

// Also works if old=nil
func GetCompressor(old io.Reader, destination io.Writer) (w Writer, close func() error) {
	return newCompressor(old, destination)
}

// Old can be nil for non delta compressed methods
func GetDecompressor(old, compressed io.Reader, m CompressionMethod) (r io.Reader, close func() error) {
	return getDecompressor(old, compressed, m)
}
