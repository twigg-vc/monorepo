package client

import (
	"fmt"
	"io"
	"monorepo/twigg/commit"
	"monorepo/twigg/repo"
	"monorepo/twigg/tree"
	"monorepo/twigg/workdir"
)

func (a *tw) Diff(A, B repo.TreeVersion, l Read) (tree.ParallelIterator, error) {
	return tree.Walk2(a.repo.Root(A, l), a.repo.Root(B, l))
}

func (a *tw) WriteDiff(A, B repo.TreeVersion, filename string, w io.Writer, l Read) error {
	diffBytes, err := tree.GetPathUnifiedDiff(filename, a.repo.Root(A, l), a.repo.Root(B, l))
	if err != nil {
		return err
	}
	_, err = w.Write(diffBytes)
	return err
}
func (a *tw) WriteDiffAll(A, B repo.TreeVersion, w io.Writer, l Read) error {
	return tree.WriteUnifiedDiff(a.repo.Root(A, l), a.repo.Root(B, l), w)
}

func (a *tw) DiffWorkdir(wd workdir.Workdir, A repo.TreeVersion, l Read) (tree.ParallelIterator, error) {
	di, err := tree.Walk2(wd, a.repo.Root(A, l))
	if err != nil {
		return nil, fmt.Errorf("err diffing wording and tree version %d: %s",
			A, err)
	}
	return di, err
}
func (a *tw) SearchFileInChangedDirs(A, B commit.Commit, l Read, filename string) (repo.FileInChangedDirsIter, error) {
	return a.repo.SearchFileInChangedDirs(A.TreeVersion, B.TreeVersion, l, filename)
}