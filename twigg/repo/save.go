package repo

import (
	"errors"
	"monorepo/twigg/tree"
	"monorepo/twigg/treev"
	"path"
)

func (r repo) Init(l Write) (TreeVersion, [32]byte, error) {
	tr := firstTree{}
	newTreePathToVersion := make(map[string]uint64)
	treeVersion, err := r.saveTree(tree.RootPath, tr,
		/*treeBlobVersion*/ 0,
		/*hasOlderTree*/ false,
		/*olderTree*/ tree_{},
		newTreePathToVersion, l)
	if treeVersion != RootTreeVersion {
		panic("tried to re-init")
	}
	if err != nil {
		return 0, [32]byte{}, err
	}
	return RootTreeVersion, tr.Data().ContentHash, err
}

func (r repo) Save(root tree.Root, baseV TreeVersion, l Write) (TreeVersion, [32]byte, error) {
	iter, err := tree.Walk(root)
	if err != nil {
		return 0, [32]byte{}, err
	}
	newV, hash, _, err := r.save(iter, baseV, l)
	return newV, hash, err
}

func (r repo) save(
	iter tree.Iterator,
	baseV TreeVersion,
	l Write) (newV TreeVersion, hash [32]byte, gotConflict bool, err error) {

	lastVersOfRoot, _, err := l.GetLastVersionOfRootTree(r.id)
	if err != nil {
		return
	}
	newV = lastVersOfRoot + 1

	baseRoot := r.getRoot_(baseV, l)

	// Maps new trees to their newest versions
	newTreePathToVersion := make(map[string]uint64)

	var trPath string
	var trPathDepth uint32
	var tr tree.Tree
	var treeOnBase tree_
	var st tree.VisitStatus
	for iter.CanGet() {
		trPath, trPathDepth, st, tr = iter.Get()
		if tr.IsRemovedChild() {
			err = iter.Next()
			if err != nil {
				return
			}
			continue
		}
		// If the tree is not yet fully computed bc its the first visit,
		// advance to process the children. The data should become computed
		// on the second visit.
		if !tr.DataIsComplete() {
			if st == tree.SecondVisit {
				panic("data not complete on second visit")
			}
			err = iter.Next()
			if err != nil {
				return
			}
			continue
		}

		// Populate data from the root dir (depth=0)
		if trPathDepth == 0 {
			hash = tr.Data().ContentHash
			gotConflict = tr.Data().HasChildWithConflicts
		}

		// Skip directories that have no children; as those are not saved
		if trPathDepth != 0 &&
			tr.Data().IsDir &&
			len(tr.Data().ChildrenBaseNames) == 0 {
			err = iter.Next()
			if err != nil {
				return
			}
			continue
		}

		// Get the older tree and compare it with this one to decide wheather
		// we can skip saving this one bc it didn't change
		_, treeOnBase, err = baseRoot.getTree(trPath)
		if err != nil && !errors.Is(err, tree.ErrTreeNotFound) {
			return
		}
		hasTreeOnBase := !errors.Is(err, tree.ErrTreeNotFound)
		if hasTreeOnBase && tree.IsEqual(treeOnBase, tr) {
			iter.SkipChildrenOnNext()
			err = iter.Next()
			if err != nil {
				return
			}
			continue
		}

		// Even if they are already fully known, directories are always saved
		// only on the second visit. This is necessary because they must
		// reference data from their children - which makes it mecessary for
		// the children to be saved first.
		// We only don't skip in the case of the root dir being empty; because
		// at least the root dir must always be saved.
		if tr.Data().IsDir && st == tree.FirstVisit &&
			len(tr.Data().ChildrenBaseNames) > 0 {
			err = iter.Next()
			if err != nil {
				return
			}
			continue
		}

		if shouldSaveTreeBlob(tr) {
			var blobVersion uint64
			blobVersion, err = r.saveTreeBlob(trPath, tr, l)
			if err != nil {
				return
			}
			_, err = r.saveTree(trPath, tr,
				blobVersion, hasTreeOnBase, treeOnBase, newTreePathToVersion, l)
			if err != nil {
				return
			}
		} else {
			_, err = r.saveTree(trPath, tr,
				0, hasTreeOnBase, treeOnBase, newTreePathToVersion, l)
			if err != nil {
				return
			}
		}

		err = iter.Next()
		if err != nil {
			return
		}
	}

	if len(newTreePathToVersion) == 0 {
		newV = baseV
		err = ErrNoChange
	}
	return
}

// This function assumes that the children are already processed, because
// it must now what the versions of the children are.
// treeBlobVersion can be anything for directory trees, as that's only used
// to read a tree
func (r repo) saveTree(
	newTreePath string,
	newTree tree.Tree,
	treeBlobVersion uint64,
	hasTreeOnBase bool,
	treeOnBase tree_,
	newTreePathToVersion map[string]uint64,
	l Write) (TreeVersion, error) {
	var err error

	// Before saving, we must find the names and versions of all children.
	// Iterate through all of them to add the name and version.
	newTreeData := treev.TreeDataV{
		Data:        newTree.Data(),
		BlobVersion: treeBlobVersion,
	}
	n := len(newTree.Data().ChildrenBaseNames)
	newTreeData.Data.ChildrenBaseNames = make([]string, 0, n)
	newTreeData.ChildrenVersions = make([]TreeVersion, 0, n)
	for _, childName := range newTree.Data().ChildrenBaseNames {
		newTreeData.Data.ChildrenBaseNames = append(newTreeData.Data.ChildrenBaseNames, childName)
		newChildVersion, childIsNew := newTreePathToVersion[path.Join(newTreePath, childName)]

		// The version of the child will depend on wheather the child is
		// new or not.
		// If the child is new, use its newest version. Else, read the parent
		// to figure out what its version of the children was
		if childIsNew {
			newTreeData.ChildrenVersions = append(
				newTreeData.ChildrenVersions, newChildVersion)
		} else {
			// If the child is not new, we read it from the old version
			if !hasTreeOnBase {
				return 0, errors.New("tree base not found")
			}
			parentChildrenNames := treeOnBase.d.Data.ChildrenBaseNames
			parentChildrenVersions := treeOnBase.d.ChildrenVersions
			found := false
			for i := range parentChildrenNames {
				if parentChildrenNames[i] == childName {
					newTreeData.ChildrenVersions = append(
						newTreeData.ChildrenVersions,
						parentChildrenVersions[i])
					found = true
					break
				}
			}
			if !found {
				panic("child is not new, but could not find it in parent")
			}
		}
	}

	version, err := l.SetTreeData(r.quotaOwner, r.id, newTreePath, newTreeData)
	if err != nil {
		return 0, err
	}
	newTreePathToVersion[newTreePath] = version
	return version, nil
}

func shouldSaveTreeBlob(tr tree.Tree) bool {
	if tr.Data().IsSymlink {
		return false
	}
	if tr.Data().IsDir {
		return false
	}
	return true
}

func (r repo) saveTreeBlob(treePath string, tr tree.Tree, l Write) (v uint64, err error) {
	if !shouldSaveTreeBlob(tr) {
		panic("tried to save blob of " + treePath)
	}
	wt, err := tr.GetFile()
	if err != nil {
		return
	}
	v, err = l.SetTreeBlob(r.quotaOwner, r.id, treePath, wt)
	return
}
