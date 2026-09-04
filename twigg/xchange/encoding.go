package xchange

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"io"
	"monorepo/twigg/commit"
	"monorepo/twigg/tree"
)

func writeProtocol(protocol uint8, w io.Writer) error {
	_, err := w.Write([]byte{protocol})
	return err
}
func readProtocol(r io.Reader) (protocol uint8, err error) {
	var buff [1]byte
	_, err = r.Read(buff[:])
	protocol = buff[0]
	return
}
func checkProtocol(p uint8) error {
	if p > CurrentProtocol {
		return ErrOldProtocol
	}
	return nil
}

func writeHeaderAndCommit(c commit.Commit, w io.Writer) error {
	commitBytes := c.Bytes()
	h := newHeader(dataType_commit, int64(len(commitBytes)))
	err := h.writeTo(w)
	if err != nil {
		return err
	}
	_, err = w.Write(commitBytes)
	return err
}
func readHeaderAndCommit(r io.Reader, h *header, c *commit.Commit) (err error) {
	err = h.readFrom(r)
	if err != nil {
		return
	}
	if h.Dt == dataType_end {
		err = io.EOF
		return
	}
	if h.Dt == dataType_unexpectedEnd {
		err = io.ErrUnexpectedEOF
		return
	}
	if h.Dt == dataType_errMsg {
		errMsgReader := io.LimitReader(r, h.Size)
		var errMsgBytes []byte
		errMsgBytes, err = io.ReadAll(errMsgReader)
		if err != nil {
			return
		}
		err = errors.New(string(errMsgBytes))
		return
	}
	if h.Dt != dataType_commit {
		err = errors.New("wrong format: first element must be a commit")
		return
	}
	return c.ReadDataFrom(io.LimitReader(r, h.Size))
}

const baseBytes = int64(8 + 8)

func writeHeaderAndBase(baseCommitL commit.LocalId, baseCommitV uint64,
	w io.Writer) error {
	h := newHeader(dataType_baseCommitLV, baseBytes)
	err := h.writeTo(w)
	if err != nil {
		return err
	}
	buff := make([]byte, 8)
	binary.BigEndian.PutUint64(buff, baseCommitL)
	_, err = w.Write(buff)
	if err != nil {
		return err
	}
	binary.BigEndian.PutUint64(buff, baseCommitV)
	_, err = w.Write(buff)
	return err
}
func readHeaderAndBase(r io.Reader, h *header) (
	baseCommitL commit.LocalId, baseCommitV uint64, err error) {
	err = h.readFrom(r)
	if err != nil {
		return
	}
	if h.Dt != dataType_baseCommitLV {
		err = errors.New("wrong format: expected base commit Id")
		return
	}
	if h.Size != baseBytes {
		err = errors.New("wrong format: expected 32 bytes")
		return
	}
	buff := make([]byte, 8)
	_, err = io.ReadFull(r, buff)
	if err != nil {
		return
	}
	baseCommitL = binary.BigEndian.Uint64(buff)

	_, err = io.ReadFull(r, buff)
	if err != nil {
		return
	}
	baseCommitV = binary.BigEndian.Uint64(buff)

	return
}
func writeHeaderAndPath(path string, w io.Writer) error {
	h := newHeader(dataType_treePath, int64(len(path)))
	err := h.writeTo(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(path))
	return err
}
func readPath(r io.Reader, pathSize int64) (path string, err error) {
	b := make([]byte, pathSize)
	_, err = io.ReadFull(io.LimitReader(r, pathSize), b)
	path = string(b)
	return
}
func writeHeaderAndTreeData(data tree.Data, w io.Writer) error {
	// We must write to a buffer bc gob doesn't provide a direct access to
	// the size that the struct will have
	buff := bytes.NewBuffer(nil)
	encoder := gob.NewEncoder(buff)
	err := encoder.Encode(data)
	if err != nil {
		return err
	}
	// Now that we know the size we can write the header, followed by the value
	h := newHeader(dataType_treeData, int64(buff.Len()))
	err = h.writeTo(w)
	if err != nil {
		return err
	}
	_, err = w.Write(buff.Bytes())
	return err
}
func readHeaderAndTreeData(r io.Reader, h *header) (md tree.Data, err error) {
	err = h.readFrom(r)
	if err != nil {
		return
	}
	if h.Dt != dataType_treeData {
		err = errors.New("wrong format: expected tree data")
		return
	}
	decoder := gob.NewDecoder(io.LimitReader(r, h.Size))
	err = decoder.Decode(&md)
	return
}
func writeHeaderAndBlob(tr tree.Tree, w io.Writer) error {
	h := newHeader(dataType_fileBlob, tr.Data().Size)
	err := h.writeTo(w)
	if err != nil {
		return err
	}
	wt, err := tr.GetFile()
	if err != nil {
		return err
	}
	_, err = wt.WriteTo(w)
	return err
}
func writeEmptyBlobHeader(w io.Writer) error {
	h := newHeader(dataType_emptyFileBlob, 0)
	return h.writeTo(w)
}
