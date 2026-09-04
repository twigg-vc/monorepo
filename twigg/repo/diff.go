package repo

import (
	"fmt"
	"monorepo/twigg/tree"
)

func (r repo) SearchFileInChangedDirs(A, B TreeVersion, l Read, filename string) (FileInChangedDirsIter, error) {
	diffs, err := tree.Walk2(r.Root(A, l), r.Root(B, l))
	if err != nil {
		return FileInChangedDirsIter{}, err
	}
	it := fileInChangedDirsIterator{filename: filename, diffs: diffs}
	err = it.goNextWhileNeeded()
	if err != nil {
		return FileInChangedDirsIter{}, err
	}
	return FileInChangedDirsIter{it: &it}, nil
}

type fileInChangedDirsIterator struct {
	filename     string
	diffs        tree.ParallelIterator
	aSideMatched bool
	bSideMatched bool
}

func (m fileInChangedDirsIterator) CanGet() bool {
	return m.diffs.CanGet()
}
func (m fileInChangedDirsIterator) GetFile() (isCreated, isModified, isDeleted bool, path string, depth uint32, tr tree.Tree, aTr tree.Tree, bTr tree.Tree) {
	defer func() {
		trueCount := 0
		if isCreated {
			trueCount += 1
		}
		if isModified {
			trueCount += 1
		}
		if isDeleted {
			trueCount += 1
		}
		if trueCount != 1 {
			panic(fmt.Sprintf("isCreated=%v isModified=%v isDeleted=%v", isCreated, isModified, isDeleted))
		}
	}()

	if m.bSideMatched {
		_, _, _, bTr = m.diffs.GetB()
		path, depth, _, tr = m.diffs.GetB()
	}
	if m.aSideMatched {
		_, _, _, aTr = m.diffs.GetA()
		path, depth, _, tr = m.diffs.GetA()
	}
	isCreated = m.aSideMatched && !m.bSideMatched
	isModified = m.aSideMatched && m.bSideMatched
	isDeleted = !m.aSideMatched && m.bSideMatched
	return
}
func (m *fileInChangedDirsIterator) Next() error {
	err := m.diffs.Next()
	if err != nil {
		return err
	}
	return m.goNextWhileNeeded()
}

func (m *fileInChangedDirsIterator) shouldStopGoingNext() bool {
	hasAnyMatch := false
	if m.diffs.CanGetA() {
		_, _, _, aTr := m.diffs.GetA()
		if !aTr.Data().IsDir && aTr.Data().BaseName == m.filename {
			m.aSideMatched = true
			hasAnyMatch = true
		}
	}
	if m.diffs.CanGetB() {
		_, _, _, bTr := m.diffs.GetB()
		if !bTr.Data().IsDir && bTr.Data().BaseName == m.filename {
			m.bSideMatched = true
			hasAnyMatch = true
		}
	}
	return hasAnyMatch
}

func (m *fileInChangedDirsIterator) goNextWhileNeeded() error {
	m.aSideMatched = false
	m.bSideMatched = false
	for m.diffs.CanGet() {
		diffType := m.diffs.GetDiff().Type
		_, _, visit, tr := m.diffs.Get()
		// Skip if diff is still undefined or this is the second visit
		if diffType == tree.DiffTypeUndefined ||
			!tr.DataIsComplete() ||
			visit == tree.SecondVisit {
			err := m.diffs.Next()
			if err != nil {
				return err
			}
			continue
		}
		if m.shouldStopGoingNext() {
			break
		}
		// Completelly skip files and non modified dirs
		isUnchagedDir := tr.Data().IsDir && diffType == tree.DiffTypeNoChange
		if isUnchagedDir {
			m.diffs.SkipChildrenOnNext()
		}
		err := m.diffs.Next()
		if err != nil {
			return err
		}
	}
	return nil
}
