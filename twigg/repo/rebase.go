package repo

import (
	"monorepo/twigg/tree"
)

func (r repo) Rebase(a TreeVersion, aLabel string,
	b TreeVersion, bLabel string, p TreeVersion, l Write) (TreeVersion, [32]byte, bool, error) {
	iter, err := tree.Rebase(r.Root(a, l), aLabel,
		r.Root(b, l), bLabel, r.Root(p, l))
	if err != nil {
		return 0, [32]byte{}, false, err
	}
	return r.save(iter, b, l)
}

func (r repo) CanRebaseWithoutConflict(a TreeVersion, b TreeVersion,
	p TreeVersion, l Read) (bool, error) {
	// non important; label is only used for displaying conflicts
	const dummyLabel = ""
	iter, err := tree.Rebase(r.Root(a, l), dummyLabel,
		r.Root(b, l), dummyLabel, r.Root(p, l))
	if err != nil {
		return false, err
	}

	for iter.CanGet() {
		_, _, _, tr := iter.Get()
		// Only analyze trees once the data is fully computed.
		// Also advance if the tree is a removed child
		if tr.IsRemovedChild() || !tr.DataIsComplete() {
			err = iter.Next()
			if err != nil {
				return false, err
			}
			continue
		}
		if tr.Data().HasChildWithConflicts || tr.Data().HasConflicts {
			return false, nil
		}
		iter.SkipChildrenOnNext()
		err = iter.Next()
		if err != nil {
			return false, err
		}
	}
	return true, nil
}
