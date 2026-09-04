package deltastream

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestAll(t *testing.T) {
	testCases := []struct {
		desc           string
		oldContent     string
		newContent     string
		expectedMethod CompressionMethod
	}{
		{
			desc:           "both empty",
			oldContent:     "",
			newContent:     "",
			expectedMethod: CompressionMethodSpeedFlate,
		},
		{
			desc:           "only old empty",
			oldContent:     "",
			newContent:     "a",
			expectedMethod: CompressionMethodSpeedFlate,
		},
		{
			desc:           "only new empty",
			oldContent:     "a",
			newContent:     "",
			expectedMethod: CompressionMethodSpeedFlate,
		},
		{
			desc:           "both small, old = new",
			oldContent:     "aaaa",
			newContent:     "bbbb",
			expectedMethod: CompressionMethodSpeedFlate,
		},
		{
			desc:           "both small, old > new",
			oldContent:     "aaaa",
			newContent:     "bbb",
			expectedMethod: CompressionMethodSpeedFlate,
		},
		{
			desc:           "both small, old < new",
			oldContent:     "aaa",
			newContent:     "bbbb",
			expectedMethod: CompressionMethodSpeedFlate,
		},
		{
			desc:           "both small but large enough for godelta",
			oldContent:     strings.Repeat("a", minSizeForGodelta+1),
			newContent:     strings.Repeat("b", minSizeForGodelta+1),
			expectedMethod: CompressionMethodGodelta,
		},
		{
			desc:           "old is too small for godelta",
			oldContent:     strings.Repeat("a", minSizeForGodelta-5),
			newContent:     strings.Repeat("b", minSizeForGodelta+1),
			expectedMethod: CompressionMethodSpeedFlate,
		},
		{
			desc:           "new is too small for godelta",
			oldContent:     strings.Repeat("a", minSizeForGodelta+1),
			newContent:     strings.Repeat("b", minSizeForGodelta-5),
			expectedMethod: CompressionMethodSpeedFlate,
		},
		{
			desc:           "both too large for godelta",
			oldContent:     strings.Repeat("a", peekSize),
			newContent:     strings.Repeat("b", peekSize),
			expectedMethod: CompressionMethodSpeedFlate,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {

			old := bytes.NewBufferString(tC.oldContent)
			new := bytes.NewBufferString(tC.newContent)

			// compressedBuff stores the compressed data
			compressedBuff := bytes.NewBuffer(nil)
			compressor, closeW := GetCompressor(old, compressedBuff)

			wrote, err := io.Copy(compressor, new)
			if err != nil {
				t.Fatal(err)
			}
			if wrote != int64(len(tC.newContent)) {
				t.Errorf(
					"%s failed. Expected to write %v, got %v",
					tC.desc, len(tC.newContent), wrote)
			}
			err = closeW()
			if err != nil {
				t.Fatal(err)
			}

			d := compressor.Data()
			if d.Method != tC.expectedMethod {
				t.Errorf(
					"%s failed. Expected to method %s, got %v",
					tC.desc, tC.expectedMethod.S(), d.Method.S())
			}

			// Must recreate buffer to read from it again
			old = bytes.NewBufferString(tC.oldContent)
			decompressor, close := GetDecompressor(old,
				compressedBuff, d.Method)
			defer close()

			// decompressedBuff will store the decompressed content
			decompressedBuff := bytes.NewBuffer(nil)
			decomprDataSize, err := io.Copy(decompressedBuff, decompressor)
			if err != nil {
				t.Fatal(err)
			}

			if decomprDataSize != int64(len(tC.newContent)) {
				t.Errorf(
					"%s failed. Expected decompressed data size: %v, got %v",
					tC.desc, len(tC.newContent), decomprDataSize)
			}

			got := decompressedBuff.String()
			if got != tC.newContent {
				t.Errorf(
					"%s failed. Expected %s, got %s",
					tC.desc, tC.newContent, got)
			}
		})
	}
}

func TestGodeltaIsBetterThanFlate(t *testing.T) {

	// Create similar contents and check that godelta compresses them better
	// that what flate would. To force flate, we use a string just a little
	// over peekSize.
	old := bytes.NewBufferString(strings.Repeat("a", peekSize-5) + "xxxx")
	new := bytes.NewBufferString(strings.Repeat("a", peekSize-5) + "yyyy")
	compressedBuff := bytes.NewBuffer(nil)
	compressor, closeW := GetCompressor(old, compressedBuff)
	io.Copy(compressor, new)
	closeW()
	if compressor.Data().Method != CompressionMethodGodelta {
		t.Fatal("should have used godelta")
	}
	godeltaSize := compressedBuff.Len()

	// The extra character will make the content too big to use delta encoding
	// and will force flate to be used
	old = bytes.NewBufferString(strings.Repeat("a", peekSize-5) + "xxxxx" + "x")
	new = bytes.NewBufferString(strings.Repeat("a", peekSize-5) + "yyyyy" + "y")
	compressedBuff = bytes.NewBuffer(nil)
	compressor, closeW = GetCompressor(old, compressedBuff)
	io.Copy(compressor, new)
	closeW()
	if compressor.Data().Method != CompressionMethodSpeedFlate {
		t.Fatal("should have used flate")
	}
	flateSize := compressedBuff.Len()

	if godeltaSize > flateSize/2 {
		t.Fatal("godelta should compress much better than flate")
	}
}

func TestCompressorsWithFiles(t *testing.T) {
	oldFile, err := os.CreateTemp("", "testfile-old-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(oldFile.Name())
	newFile, err := os.CreateTemp("", "testfile-new-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(newFile.Name())

	oldContent := strings.Repeat("abcdef", 1_000)
	oldFile.Write([]byte(oldContent))
	oldFile.Seek(0, 0)
	newContent := strings.Repeat("fgh", 1_000)
	newFile.Write([]byte(newContent))
	newFile.Seek(0, 0)

	compressedStore := bytes.NewBuffer(nil)
	w, cl := GetCompressor(oldFile, compressedStore)
	wrote, err := io.Copy(w, newFile)
	if err != nil {
		t.Fatal(err)
	}
	if wrote != int64(len(newContent)) {
		t.Fatal("expected to write the whole new content")
	}
	err = cl()
	if err != nil {
		t.Fatal(err)
	}

	oldFile.Seek(0, 0)
	r, cl := GetDecompressor(oldFile, compressedStore, w.Data().Method)
	defer cl()
	uncompressedStore := bytes.NewBuffer(nil)
	wrote, err = io.Copy(uncompressedStore, r)
	if err != nil {
		t.Fatal(err)
	}
	if wrote != int64(len(newContent)) {
		t.Fatal("expected to write the whole uncompressed new content")
	}

	if uncompressedStore.String() != newContent {
		t.Fatal("wrong content")
	}
}

func TestCompressorWithNilOld(t *testing.T) {
	new := bytes.NewBufferString("abcde")
	compressedBuff := bytes.NewBuffer(nil)
	compressor, closeW := GetCompressor(nil, compressedBuff)
	io.Copy(compressor, new)
	err := closeW()
	if err != nil {
		t.Fatal(err)
	}
	if compressor.Data().Method != CompressionMethodSpeedFlate {
		t.Fatal("should have used flate")
	}

	decomp, closeDecomp := GetDecompressor(nil,
		compressedBuff, CompressionMethodSpeedFlate)
	uncompressedBuff := bytes.NewBuffer(nil)
	wrote, err := io.Copy(uncompressedBuff, decomp)
	if err != nil {
		t.Fatal(err)
	}
	if wrote != 5 {
		t.Fatal("expected to copy 5 bytes")
	}
	if uncompressedBuff.String() != "abcde" {
		t.Fatal("wrong content")
	}

	err = closeDecomp()
	if err != nil {
		t.Fatal(err)
	}

}

func (c CompressionMethod) S() string {
	switch c {
	case CompressionMethodSpeedFlate:
		return "SpeedFlate"
	case CompressionMethodGodelta:
		return "godelta"
	default:
		panic("not implemented")
	}
}
