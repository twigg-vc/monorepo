package client

import (
	"errors"
	"fmt"
	"io"
	"monorepo/twigg/commit"
	"monorepo/twigg/repo"
	"monorepo/twigg/tree"
)

func (a tw) Read(c repo.TreeVersion, filename string, w io.Writer, l Read) (err error) {
	defer func() {
		if errors.Is(err, tree.ErrTreeNotFound) {
			err = ErrFileNotFound
		}
	}()

	rt := a.repo.Root(c, l)
	tr, err := rt.Tree(filename)
	if err != nil {
		return
	}
	if tr.IsRemovedChild() {
		panic("repository returned RemovedChild tree")
	}
	if tr.Data().IsDir {
		err = fmt.Errorf("%s is a directory", filename)
		return
	}

	wt, err := tr.GetFile()
	if err != nil {
		return
	}
	_, err = wt.WriteTo(w)
	return
}

func (a tw) GetRoot(c commit.Commit, l Read) tree.Root {
	return a.repo.Root(c.TreeVersion, l)
}