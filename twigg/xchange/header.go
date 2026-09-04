package xchange

import (
	"encoding/binary"
	"io"
)

// Describes what the next chunk of data on a data stream is.
// MUST be of fixed size
type header struct {
	Dt   dataType
	Size int64
}

func newHeader(t dataType, size int64) header {
	return header{t, size}
}

type dataType uint8

const (
	dataType_commit dataType = iota
	dataType_baseCommitLV
	dataType_treePath
	dataType_treeData
	dataType_emptyFileBlob // placeholder when the blob is not added
	dataType_fileBlob
	dataType_end
	dataType_errMsg
	dataType_unexpectedEnd
)

func (h header) writeTo(w io.Writer) error {
	return binary.Write(w, binary.LittleEndian, h)
}

func (h *header) readFrom(r io.Reader) (err error) {
	err = binary.Read(r, binary.LittleEndian, h)
	return
}