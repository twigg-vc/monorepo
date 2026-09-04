package tree

import (
	"bytes"
	"errors"
	"io"
	"monorepo/twigg/diff"
	"time"
)

func rebase(A Root, aLabel string, B Root, bLabel string, P Root) (Iterator, error) {
	r := &rebaseRoot{A: A, ALabel: aLabel, B: B, BLabel: bLabel, P: P}
	return Walk(r)
}

type rebaseRoot struct {
	A      Root
	ALabel string
	B      Root
	BLabel string
	P      Root
}

func (rb *rebaseRoot) Tree(relativePath string) (Tree, error) {
	hasA := true
	aTr, err := rb.A.Tree(relativePath)
	if err != nil && !errors.Is(err, ErrTreeNotFound) {
		return nil, err
	}
	if errors.Is(err, ErrTreeNotFound) || aTr.IsRemovedChild() {
		hasA = false
	}
	hasB := true
	bTr, err := rb.B.Tree(relativePath)
	if err != nil && !errors.Is(err, ErrTreeNotFound) {
		return nil, err
	}
	if errors.Is(err, ErrTreeNotFound) || bTr.IsRemovedChild() {
		hasB = false
	}
	hasP := true
	pTr, err := rb.P.Tree(relativePath)
	if err != nil && !errors.Is(err, ErrTreeNotFound) {
		return nil, err
	}
	if errors.Is(err, ErrTreeNotFound) || pTr.IsRemovedChild() {
		hasP = false
	}
	if !hasA && !hasB {
		return nil, ErrTreeNotFound
	}

	return getRebaseTree(hasA, aTr, rb.ALabel, hasP, pTr, hasB, bTr, rb.BLabel)
}
func getRebaseTree(hasA bool, aTr Tree, aLabel string,
	hasP bool, pTr Tree, hasB bool, bTr Tree, bLabel string) (Tree, error) {
	if !hasA && !hasB {
		panic("tree not in A nor B")
	}
	// Tree is not in A -> Return B
	if !hasA {
		if bTr.Data().IsDir {
			return dirRebase{
				data: NewUnknownDirData(bTr.Data().BaseName,
					bTr.Data().Depth, bTr.Data().ChildrenBaseNames),
			}, nil
		}
		return bTr, nil
	}

	// If the tree is also in the parent of A, we must check if A modified it
	// or not. If A didn't modify it, we just return B if it exists or child
	// not found, because we only want to rebase the changes that A made
	// with respect to its parent
	if hasP {
		diffType, _ := getTreeDiffTypeAndData(aTr, pTr)
		if diffType == DiffTypeNoChange {
			if hasB {
				return bTr, nil
			}
			return NewRemovedChildTree(), nil
		}
	}
	// Tree is not in B and A modified it -> Return A
	if !hasB {
		if aTr.Data().IsDir {
			return dirRebase{
				data: NewUnknownDirData(aTr.Data().BaseName,
					aTr.Data().Depth, aTr.Data().ChildrenBaseNames),
			}, nil
		}
		return aTr, nil
	}

	// Some cases can't be merged; so A takes precedence
	canMerge := true
	if aTr.Data().IsSymlink || bTr.Data().IsSymlink {
		canMerge = false
	}
	if aTr.Data().IsDir != bTr.Data().IsDir {
		canMerge = false
	}
	if !canMerge {
		return aTr, nil
	}

	if aTr.Data().IsDir {
		if hasP && pTr.Data().IsDir {
			return newDirRebase(aTr, pTr, bTr), nil
		}
		return newDirRebase(aTr, nil, bTr), nil
	}
	// If P is not a regular file, it's as if it didn't exist
	if hasP && (pTr.Data().IsDir || pTr.Data().IsSymlink) {
		hasP = false
	}
	return newFileRebase(aTr, aLabel, bTr, bLabel, hasP, pTr)
}

// Represents a single file that is the result of a rebase.
// The file CANT be a symlink.
// It necessarily is a rebase of two file trees which have the same name.
type fileRebase struct {
	data Data
	wt   io.WriterTo
}

const maxFileSizeToMerge = 1024 * 1024 // 1MB

// Populates metadata and writerto
func newFileRebase(aFile Tree, aLabel string,
	bFile Tree, bLabel string, hasPFile bool, pFile Tree) (fileRebase, error) {
	baseName := aFile.Data().BaseName
	f := fileRebase{}

	if aFile.Data().IsDir || bFile.Data().IsDir || (hasPFile && pFile.Data().IsDir) {
		panic("fileRabase used for dir trees")
	}
	if aFile.Data().IsSymlink || bFile.Data().IsSymlink {
		panic("fileRabase used for symlink file")
	}

	// Check if we can merge the files.
	shouldMerge :=
		aFile.Data().IsText &&
			bFile.Data().IsText &&
			aFile.Data().Size < maxFileSizeToMerge &&
			bFile.Data().Size < maxFileSizeToMerge
	if hasPFile {
		shouldMerge = shouldMerge && pFile.Data().IsText &&
			pFile.Data().Size < maxFileSizeToMerge
	}
	// Else, return the A version
	if !shouldMerge {
		f.data = aFile.Data()
		wt, err := aFile.GetFile()
		if err != nil {
			return f, err
		}
		f.wt = wt
		return f, nil
	}

	aWt, err := aFile.GetFile()
	if err != nil {
		return f, err
	}
	aBuff := bytes.NewBuffer(nil)
	_, err = aWt.WriteTo(aBuff)
	if err != nil {
		return f, err
	}
	bWt, err := bFile.GetFile()
	if err != nil {
		return f, err
	}
	bBuff := bytes.NewBuffer(nil)
	_, err = bWt.WriteTo(bBuff)
	if err != nil {
		return f, err
	}
	pBuff := bytes.NewBuffer(nil)
	if hasPFile {
		pWt, err := pFile.GetFile()
		if err != nil {
			return f, err
		}
		_, err = pWt.WriteTo(pBuff)
		if err != nil {
			return f, err
		}
	}
	mergedFileBytes, mergeConflicted := diff.Merge(
		pBuff.Bytes(),
		aBuff.Bytes(),
		aLabel,
		bBuff.Bytes(),
		bLabel)

	// In this implementation, if there is no parent A takes priority.
	// If the parent exists, its A if A changed, else B.
	// This way changes in execute permission don't cause conflicts (
	// which is convenient but may cause problems). We can change this
	// later on if wanted.
	var isExecutableFile bool
	if !hasPFile {
		isExecutableFile = aFile.Data().IsExectuableFile
	} else {
		isExecutableFile = bFile.Data().IsExectuableFile
		if aFile.Data().IsExectuableFile != pFile.Data().IsExectuableFile {
			isExecutableFile = aFile.Data().IsExectuableFile
		}
	}

	f.data = NewData(
		baseName,
		/*depth*/ aFile.Data().Depth,
		/*isDir*/ false,
		isExecutableFile,
		/*isText*/ true,
		/*hasConflicts*/ mergeConflicted,
		/*childHasConflicts*/ false,
		/*isSymlink*/ false,
		/*symlinkTarget*/ "",
		/*size*/ int64(len(mergedFileBytes)),
		/*lastModifiedUnix*/ time.Now().UnixMilli(),
		NewHasher().WriteBool(isExecutableFile).WriteBytes(mergedFileBytes).Sum(),
		/*ChildNames*/ nil,
	)
	f.wt = bytes.NewBuffer(mergedFileBytes)
	return f, nil
}
func (f fileRebase) IsSymLinkTo() string {
	return ""
}
func (rt fileRebase) IsRemovedChild() bool {
	return false
}
func (rt fileRebase) DataIsComplete() bool {
	return true
}
func (rt fileRebase) Data() Data {
	return rt.data
}
func (rt fileRebase) GetFile() (wt io.WriterTo, err error) {
	wt = rt.wt
	return
}

// Represents a single directory that is the result of a rebase.
// It necessarily is a rebase of two directory trees which have the same name.
// Note that the children will contain ALL children that dirA has, not
// only the ones that were modified/created by A. They need to be filtered.
type dirRebase struct {
	data Data
}

func newDirRebase(dirA, dirP, dirB Tree) dirRebase {
	aChildren := dirA.Data().ChildrenBaseNames
	bChildren := dirB.Data().ChildrenBaseNames
	var pChildren []string
	if dirP != nil {
		pChildren = dirP.Data().ChildrenBaseNames
	}
	rb := dirRebase{
		data: NewUnknownDirData(dirA.Data().BaseName, dirA.Data().Depth, nil),
	}

	// The final list of children is the union of children of A and B,
	// minus the children that A deleted. Because if A deleted a child, it'll
	// also delete that child when applied on B.
	a := 0
	canGetA := func() bool { return a < len(aChildren) }
	p := 0
	canGetP := func() bool { return p < len(pChildren) }
	// If A and P have a child, that child might be unchanged or modified.
	// That will be figured out later. But I'll use "adds" to represent
	// those files. The deletions, though, must be removed from children of B.
	adds := []string{}
	deletes := []string{}
	for canGetA() || canGetP() {
		if canGetA() && !canGetP() {
			adds = append(adds, aChildren[a])
			a++
			continue
		}
		if !canGetA() && canGetP() {
			deletes = append(deletes, pChildren[p])
			p++
			continue
		}
		aName := aChildren[a]
		pName := pChildren[p]
		if aName == pName {
			adds = append(adds, aName)
			a++
			p++
			continue
		}
		if aName < pName {
			adds = append(adds, aName)
			a++
			continue
		}
		deletes = append(deletes, pChildren[p])
		p++
	}

	// First lets merge B and the adds. Then we filter out the deletions.
	union := make([]string, 0, len(adds))
	a = 0
	canGetAd := func() bool { return a < len(adds) }
	b := 0
	canGetB := func() bool { return b < len(bChildren) }
	for canGetAd() || canGetB() {
		if canGetAd() && !canGetB() {
			union = append(union, adds[a])
			a++
			continue
		}
		if !canGetAd() && canGetB() {
			union = append(union, bChildren[b])
			b++
			continue
		}
		addName := adds[a]
		bName := bChildren[b]
		if addName == bName {
			union = append(union, addName)
			a++
			b++
			continue
		}
		if addName < bName {
			union = append(union, addName)
			a++
			continue
		}
		union = append(union, bName)
		b++
	}

	// Finally set the final result by adding everything from the union
	// that is not in the deleted list
	rb.data.ChildrenBaseNames = make([]string, 0, len(adds))
	d := 0
	for i := 0; i < len(union); i++ {
		for d < len(deletes) && deletes[d] < union[i] {
			d++
			continue
		}
		if d == len(deletes) {
			rb.data.ChildrenBaseNames = append(rb.data.ChildrenBaseNames,
				union[i])
			continue
		}
		if deletes[d] == union[i] {
			d++
			continue
		}
		rb.data.ChildrenBaseNames = append(rb.data.ChildrenBaseNames,
			union[i])
	}
	return rb
}
func (rt dirRebase) IsSymLinkTo() string {
	return ""
}
func (rt dirRebase) IsRemovedChild() bool {
	return false
}
func (rt dirRebase) DataIsComplete() bool {
	return false
}
func (rt dirRebase) Data() Data {
	return rt.data
}
func (rt dirRebase) GetFile() (wt io.WriterTo, err error) {
	panic("tried to GetFile from directory tree")
}

type removedChildTree struct{}

func (rc removedChildTree) IsRemovedChild() bool {
	return true
}
func (rc removedChildTree) DataIsComplete() bool {
	panic("called DataIsComplete() on IsRemovedChild tree")
}
func (rc removedChildTree) Data() Data {
	panic("called Data() on IsRemovedChild tree")
}
func (rc removedChildTree) GetFile() (wt io.WriterTo, err error) {
	panic("called GetFile() on IsRemovedChild tree")
}
