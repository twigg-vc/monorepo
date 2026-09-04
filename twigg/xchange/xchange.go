package xchange

import (
	"compress/flate"
	"errors"
	"io"
	"monorepo/twigg/commit"
	"monorepo/twigg/repo"
	"monorepo/twigg/tree"
)

const flateLevel = flate.BestSpeed

func newWriter(w io.Writer) (cw CommitWriter, close func() error, err error) {
	zipW, err := flate.NewWriter(w, flateLevel)
	if err != nil {
		// Flate only returns err for invalid compression
		panic("crating flate failed: " + err.Error())
	}
	close = zipW.Close
	protocolVersion := CurrentProtocol
	if UseMockProtocolVersionSentByWriters {
		protocolVersion = MockProtocolVersionSentByWriters
	}
	cw = commitWriter{zipW: zipW, protocol: protocolVersion}
	err = writeProtocol(protocolVersion, zipW)
	return
}

type commitWriter struct {
	zipW     io.Writer
	protocol uint8 // indicates the version of the protocol (encoding, etc)
}

func (cw commitWriter) WriteErrMsg(msg string) error {
	msgBytes := []byte(msg)
	h := newHeader(dataType_errMsg, int64(len(msgBytes)))
	err := h.writeTo(cw.zipW)
	if err != nil {
		return err
	}
	_, err = cw.zipW.Write(msgBytes)
	return err
}
func (cw commitWriter) WriteEof() (err error) {
	h := newHeader(dataType_end, 0)
	err = h.writeTo(cw.zipW)
	return
}
func (cw commitWriter) WriteUnexpectedEof() (err error) {
	h := newHeader(dataType_unexpectedEnd, 0)
	err = h.writeTo(cw.zipW)
	return
}

func (cw commitWriter) Write(
	c commit.Commit,
	baseCommitServerL commit.LocalId, baseCommitServerV uint64,
	baseTreeVersion repo.TreeVersion,
	r repo.Repo, l repo.Read) (err error) {
	// First write the commit
	err = writeHeaderAndCommit(c, cw.zipW)
	if err != nil {
		return
	}

	// Write the base
	err = writeHeaderAndBase(
		baseCommitServerL, baseCommitServerV, cw.zipW)
	if err != nil {
		return
	}

	// Now we write the actual tree in the DeltIterator order
	deltaIter, err := r.GetDelta(c.TreeVersion, baseTreeVersion, l)
	if err != nil {
		return
	}
	var path string
	var tr tree.Tree
	for deltaIter.CanGet() {
		path, _, tr = deltaIter.Get()
		err = writeHeaderAndPath(path, cw.zipW)
		if err != nil {
			return
		}

		err = writeHeaderAndTreeData(tr.Data(), cw.zipW)
		if err != nil {
			return
		}

		// If it's a non-symlink file, write the blob
		if !tr.Data().IsDir && !tr.Data().IsSymlink {
			dif := deltaIter.GetDiff()
			if dif.Type == tree.DiffTypeNoChange {
				err = writeEmptyBlobHeader(cw.zipW)
				if err != nil {
					return
				}
			} else {
				err = writeHeaderAndBlob(tr, cw.zipW)
				if err != nil {
					return
				}
			}
		}
		// If it's a symlink file we don't write anything else, since the
		// tree data itself already has all the data needed

		err = deltaIter.Pop()
		if err != nil {
			return
		}
	}

	// Write an `end` header to show we're done with this commit
	h := newHeader(dataType_end, 0)
	err = h.writeTo(cw.zipW)
	return
}

type commitReader struct {
	zipR     io.Reader
	protocol uint8
}

func newReader(r io.Reader) (cr CommitReader, close func() error, err error) {
	zipR := flate.NewReader(r)
	close = zipR.Close
	p, err := readProtocol(zipR)
	if err != nil {
		_ = close()
		close = func() error { return nil }
		return
	}
	err = checkProtocol(p)
	if err != nil {
		_ = close()
		close = func() error { return nil }
		return
	}
	return &commitReader{zipR: zipR, protocol: p}, close, err
}

func (cr *commitReader) Read() (c commit.Commit,
	baseId commit.LocalId, baseV uint64, it repo.DeltaIter, err error) {
	var h header
	err = readHeaderAndCommit(cr.zipR, &h, &c)
	if err != nil {
		return
	}
	baseId, baseV, err = readHeaderAndBase(cr.zipR, &h)
	if err != nil {
		return
	}
	it, err = newIter(cr.zipR)
	return
}

type deltaIter struct {
	r               io.Reader
	isDone          bool
	lastHeader      header
	currentTreeData tree.Data
	currentTreePath string
}

func newIter(r io.Reader) (repo.DeltaIter, error) {
	iter := &deltaIter{
		r:      r,
		isDone: false,
	}
	err := iter.readNextTree()
	return iter, err
}
func (it deltaIter) CanGet() bool {
	return !it.isDone
}
func (it deltaIter) Get() (string, uint32, tree.Tree) {
	return it.currentTreePath, it.currentTreeData.Depth, tree_{
		data: it.currentTreeData,
		it:   it,
	}
}
func (it *deltaIter) Pop() error {
	if it.isDone {
		panic("called Next() even when CanGet()=false")
	}
	return it.readNextTree()
}

// Read the next chunk
func (it *deltaIter) readNextTree() (err error) {
	err = it.lastHeader.readFrom(it.r)
	if err != nil {
		return
	}

	// All tree chunks start with the tree path or an "end" header
	if it.lastHeader.Dt != dataType_treePath &&
		it.lastHeader.Dt != dataType_end {
		err = errors.New("wrong format: expected tree path/end")
		return
	}
	if it.lastHeader.Dt == dataType_end {
		it.isDone = true
		return
	}
	it.currentTreePath, err = readPath(it.r, it.lastHeader.Size)
	if err != nil {
		return
	}
	// Followed by tree data
	it.currentTreeData, err = readHeaderAndTreeData(it.r, &it.lastHeader)
	if err != nil {
		return
	}
	if !it.currentTreeData.IsDir && !it.currentTreeData.IsSymlink {
		// For blobs, the file blob is written
		err = it.lastHeader.readFrom(it.r)
		if err != nil {
			return
		}
		if it.lastHeader.Dt != dataType_fileBlob &&
			it.lastHeader.Dt != dataType_emptyFileBlob {
			err = errors.New("wrong format: expected file blob")
			return
		}
	}
	// For symlinks, nothing else is written

	return
}

type tree_ struct {
	data tree.Data
	it   deltaIter
}

func (tr tree_) IsRemovedChild() bool {
	return false
}
func (tr tree_) DataIsComplete() bool {
	return true
}
func (tr tree_) Data() tree.Data {
	return tr.data
}
func (tr tree_) GetFile() (wt io.WriterTo, err error) {
	if tr.it.lastHeader.Dt == dataType_fileBlob {
		wt = readerToWriterTo{r: io.LimitReader(tr.it.r, tr.it.lastHeader.Size)}
		return
	}
	if tr.it.lastHeader.Dt == dataType_emptyFileBlob {
		wt = readerToWriterTo{r: unavailableReader{}}
		return
	}
	err = errors.New("unexpected GetFile call")
	return
}
