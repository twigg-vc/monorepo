package repo

import (
	"errors"
	"monorepo/twigg/tree"
)

type deltaIter struct {
	a TreeVersion
	b TreeVersion
	d tree.ParallelIterator
}

func (r repo) GetDelta(a, b TreeVersion, l Read) (DeltaIterWithDiff, error) {
	d, err := tree.Walk2(r.Root(a, l), r.Root(b, l))
	if err != nil {
		return nil, err
	}
	di := deltaIter{a: a, b: b, d: d}
	return di, di.advanceWhileNeeded()
}
func (d deltaIter) CanGet() bool {
	return d.d.CanGet()
}
func (d deltaIter) Get() (string, uint32, tree.Tree) {
	p, depth, _, tr := d.d.Get()
	return p, depth, tr
}
func (d deltaIter) GetDiff() tree.Diff {
	return d.d.GetDiff()
}
func (d deltaIter) Pop() error {
	d.d.SkipChildrenOnNext()
	err := d.d.Next()
	if err != nil {
		return err
	}
	return d.advanceWhileNeeded()
}

func (d deltaIter) advanceWhileNeeded() error {
	if !d.d.CanGet() {
		return nil
	}
	dif := d.d.GetDiff()
	_, depth, st, tr := d.d.Get()
	for {
		// Stop if we reached a file that was not deleted
		if !dif.Data.IsDir && dif.Type != tree.DiffTypeDeleted {
			break
		}
		// Stop if we reached a NoChange or if this is a second visit (i.e. we
		// got here after a Pop)
		if dif.Type == tree.DiffTypeNoChange || st == tree.SecondVisit {
			break
		}
		// Stop if we read the root tree that is empty. This is an edge case
		// because in this case the folder would just be skipped on next, but
		// we must always save at least the root dir
		if depth == 0 && len(tr.Data().ChildrenBaseNames) == 0 {
			break
		}
		if dif.Type == tree.DiffTypeDeleted {
			d.d.SkipChildrenOnNext()
		}
		err := d.d.Next()
		if err != nil {
			return err
		}
		if !d.d.CanGet() {
			return nil
		}
		dif = d.d.GetDiff()
		_, _, st, _ = d.d.Get()
	}
	return nil
}

func (r repo) SaveDelta(d DeltaIter, base TreeVersion, l Write) (newV TreeVersion, rootDirHash [32]byte, err error) {
	lastVersionOfRoot, _, err := l.GetLastVersionOfRootTree(r.id)
	if err != nil {
		return
	}
	newV = lastVersionOfRoot + 1

	// Maps new trees to their most recent versions
	newTreesVersions := make(map[string]uint64)

	// Provides the trees at the base version
	baseRoot := r.getRoot_(base, l)

	for d.CanGet() {
		treePath, treePathDepth, tr := d.Get()
		if !tr.DataIsComplete() {
			err = errors.New("got delta with non complete tree")
			return
		}
		if treePathDepth == 0 {
			rootDirHash = tr.Data().ContentHash
		}

		// Get the tree in base to figure out if we need to save this one
		// or if it's unchanged
		var baseTr_ tree_
		_, baseTr_, err = baseRoot.getTree(treePath)
		if err != nil && !errors.Is(err, tree.ErrTreeNotFound) {
			return
		}
		hasBaseTree := !errors.Is(err, tree.ErrTreeNotFound)
		// If it's the same as the base, no need to save; just continue
		if hasBaseTree && tree.IsEqual(baseTr_, tr) {
			err = d.Pop()
			if err != nil {
				return
			}
			continue
		}

		if shouldSaveTreeBlob(tr) {
			var blobTreeVersion uint64
			blobTreeVersion, err = r.saveTreeBlob(treePath, tr, l)
			if err != nil {
				return
			}
			_, err = r.saveTree(treePath, tr, blobTreeVersion, hasBaseTree, baseTr_,
				newTreesVersions, l)
			if err != nil {
				return
			}
		} else {
			_, err = r.saveTree(treePath,
				tr, 0, hasBaseTree, baseTr_, newTreesVersions, l)
			if err != nil {
				return
			}
		}

		err = d.Pop()
		if err != nil {
			return
		}
	}

	if len(newTreesVersions) == 0 {
		newV = base
		err = ErrNoChange
	}
	return
}
