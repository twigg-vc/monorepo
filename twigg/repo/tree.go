package repo

import (
	"bytes"
	"io"
	"math"
	"monorepo/twigg/tree"
	"monorepo/twigg/treev"
)

func (r repo) Tree(name string, v TreeVersion, l Read) (tree.Tree, error) {
	return r.Root(v, l).Tree(name)
}

type tree_ struct {
	repoId   uint64
	treePath string
	d        treev.TreeDataV
	l        Read
}

func (ft tree_) IsRemovedChild() bool {
	return false
}
func (ft tree_) DataIsComplete() bool {
	return true
}
func (tr tree_) Data() tree.Data {
	return tr.d.Data
}
func (tr tree_) GetFile() (wt io.WriterTo, err error) {
	if tr.d.Data.IsDir {
		panic("called ReadFile on directory tree")
	}
	if tr.d.Data.IsSymlink {
		wt = bytes.NewBufferString(
			tree.SymlinkString(tr.Data().SymlinkTarget))
		return
	}
	r, closeR, isNotFoundErr, err := tr.l.GetTreeBlob(tr.repoId, tr.treePath, tr.d.BlobVersion)
	if isNotFoundErr {
		err = tree.ErrTreeNotFound
	}
	wt = readerToWriter{r: r, closeR: closeR}
	return
}

type firstTree struct{}

func (ft firstTree) IsRemovedChild() bool {
	return false
}
func (ft firstTree) DataIsComplete() bool {
	return true
}
func (ft firstTree) Data() tree.Data {
	hash := tree.NewHasher()
	return tree.NewData(
		tree.RootPath,
		0,
		true,
		false,
		false,
		false,
		false,
		false,
		"",
		0,
		math.MinInt64,
		hash.Sum(),
		nil,
	)
}
func (ft firstTree) GetFile() (wt io.WriterTo, err error) {
	panic("tried to read file on directory tree")
}

type readerToWriter struct {
	r      io.Reader
	closeR func()
}

func (wt readerToWriter) WriteTo(w io.Writer) (int64, error) {
	n, err := io.Copy(w, wt.r)
	wt.closeR()
	return n, err
}
