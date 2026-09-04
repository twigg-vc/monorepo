package tree

import (
	"errors"
)

func walk4(A, BaseA, B, BaseB Root) (ParallelIterator, error) {
	return Walk2(
		iter4Root{
			a:     A,
			aBase: BaseA,
			b:     B,
			bBase: BaseB,
		},
		iter4Root{
			a:     B,
			aBase: BaseB,
			b:     A,
			bBase: BaseA,
		},
	)
}

type iter4Root struct {
	a     Root
	aBase Root
	b     Root
	bBase Root
}

func (m iter4Root) Tree(path string) (tr Tree, err error) {
	defer func() {
		tr = Flatten(tr)
	}()
	aTr, err := m.a.Tree(path)
	if err != nil {
		return
	}
	if aTr.IsRemovedChild() {
		tr = aTr
		err = nil
		return
	}
	if !aTr.DataIsComplete() {
		tr = aTr
		err = nil
		return
	}

	// If the tree was modified by A, return it
	modByA, err := treeWasModified(aTr, path, m.aBase)
	if err != nil {
		return
	}
	if modByA {
		tr = aTr
		err = nil
		return
	}

	// Else,check B first.
	// We want to return the tree on A if it was modified by B.
	// If the tree doesnt show up on B or was not modified by B, we're fitlering
	// it on purpose, so we return `true,ErrTreeNotFound`
	bTr, err := m.b.Tree(path)
	if errors.Is(err, ErrTreeNotFound) {
		tr = NewRemovedChildTree()
		err = nil
		return
	}
	if bTr.IsRemovedChild() {
		tr = NewRemovedChildTree()
		err = nil
		return
	}
	if err != nil {
		return
	}
	modByB, err := treeWasModified(bTr, path, m.bBase)
	if err != nil {
		return
	}
	if modByB {
		tr = aTr
		err = nil
		return
	}
	tr = NewRemovedChildTree()
	err = nil
	return
}

// Returns true if the tree was modified with respect to the base.
func treeWasModified(tr Tree, path string, Base Root) (bool, error) {
	if !tr.DataIsComplete() {
		panic("treeWasModified with unknown tree")
	}
	baseTr, err := Base.Tree(path)
	if err != nil && !errors.Is(err, ErrTreeNotFound) {
		return false, err
	}
	// If not on base, it was created
	hasBaseTr := err == nil && !baseTr.IsRemovedChild()
	if !hasBaseTr {
		return true, nil
	}
	if !baseTr.DataIsComplete() {
		// The current implementation requires bases to only provide complete
		// trees just beacuse this function just checks with the base to
		// know what the tree was on the base. If the treeData is not complete,
		// we'd need to walk all its children to compute it. That would be
		// possible I think, but is not currently necessary, because the
		// use case for now is just to be used in cases in which the base
		// already has all tree datas pre computed
		panic("used Walk4 with a base that provides non complete trees")
	}
	// If on base, lets compare the trees to see if they're the same
	ty, _ := getTreeDiffTypeAndData(tr, baseTr)
	if ty == DiffTypeNoChange {
		return false, nil
	}
	if ty == DiffTypeUndefined {
		panic("got undefined diff type")
	}
	return true, nil
}
