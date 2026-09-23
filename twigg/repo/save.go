package repo

import (
	"errors"
	"fmt"
	"io"
	"monorepo/twigg/tree"
	"monorepo/twigg/treev"
	"path"
)

func (r repo) Init(l Write) (TreeVersion, [32]byte, error) {
	tr := firstTree{}
	v, err := l.GrabRootTreeVersion(r.id)
	if err != nil {
		return 0, [32]byte{}, err
	}
	if v != RootTreeVersion {
		panic("tried to re-init")
	}
	err = r.saveTree(tree.RootPath, tr, v,
		/*treeBlobVersion*/ 0,
		/*hasOlderTree*/ false,
		/*olderTree*/ tree_{},
		make(map[string]bool), l)
	if err != nil {
		return 0, [32]byte{}, err
	}
	return RootTreeVersion, tr.Data().ContentHash, err
}

func (r repo) GrabRootTreeVersion(l Write) (TreeVersion, error) {
	return l.GrabRootTreeVersion(r.id)
}
func (r repo) SaveFile(rootTreeVersion TreeVersion, treePath string,
	data tree.Data, wt io.WriterTo, baseV TreeVersion, l Write) error {
	if data.IsDir {
		return errors.New("data must be of file")
	}
	err := l.SetTreeData(r.quotaOwner, r.id, treePath,
		rootTreeVersion, treev.TreeDataV{
			Data:             data,
			ChildrenVersions: nil,
			BlobVersion:      rootTreeVersion,
		})
	if err != nil {
		return err
	}
	return l.SetTreeBlob(r.quotaOwner, r.id, treePath, rootTreeVersion, wt)
}
func (r repo) SaveDir(rootTreeVersion TreeVersion, treePath string,
	d tree.Data, baseV TreeVersion, l Write) error {
	if !d.IsDir {
		return errors.New("data must be of directory")
	}
	if len(d.ChildrenBaseNames) > 0 && !d.HasChildrenData {
		return errors.New("SaveDir requires HasChildrenData")
	}
	if len(d.ChildrenData) != len(d.ChildrenBaseNames) {
		return fmt.Errorf(
			"bad tree data: %d ChildrenData and %d ChildrenBaseNames",
			len(d.ChildrenData), len(d.ChildrenBaseNames))
	}
	baseRoot := r.getRoot_(baseV, l)
	_, treeOnBase, err := baseRoot.getTree(treePath)
	if err != nil && !errors.Is(err, tree.ErrTreeNotFound) {
		return err
	}
	hasTreeOnBase := !errors.Is(err, tree.ErrTreeNotFound)

	// Before saving, we must find the versions of all children.
	// The versions will either be rootTreeVersion if this version of the
	// root tree modified that child (when compared to base); or the version
	// that it was in the base.
	n := len(d.ChildrenBaseNames)
	newTree := treev.TreeDataV{
		Data:             d,
		BlobVersion:      0,
		ChildrenVersions: make([]TreeVersion, 0, n),
	}
	for childI := range d.ChildrenBaseNames {
		childName := d.ChildrenBaseNames[childI]
		childData := d.ChildrenData[childI]

		childVersion := func() TreeVersion {
			if !hasTreeOnBase {
				return rootTreeVersion
			}
			if !treeOnBase.Data().IsDir {
				return rootTreeVersion
			}
			// The only case when we'll point to the tree on the base:
			// the base tree has a child with the same name and same content
			for i := range treeOnBase.Data().ChildrenBaseNames {
				if treeOnBase.Data().ChildrenBaseNames[i] == childName &&
					treeOnBase.Data().ChildrenData[i].ContentHash ==
						childData.ContentHash {
					return treeOnBase.d.ChildrenVersions[i]
				}
			}
			return rootTreeVersion
		}()
		newTree.ChildrenVersions = append(newTree.ChildrenVersions, childVersion)
	}

	return l.SetTreeData(r.quotaOwner, r.id, treePath, rootTreeVersion, newTree)
}
func (r repo) saveDirOrSaveFile(rootTreeVersion TreeVersion, treePath string,
	tr tree.Tree, base TreeVersion, l Write) error {
	if !tr.DataIsComplete() || tr.IsRemovedChild() {
		panic("called saveDirOrSaveFile for bad tree")
	}
	if tr.Data().IsDir {
		return r.SaveDir(rootTreeVersion, treePath, tr.Data(), base, l)
	}
	wt, err := tr.GetFile()
	if err != nil {
		return err
	}
	return r.SaveFile(rootTreeVersion, treePath, tr.Data(), wt, base, l)
}
func (r repo) SaveRoot(root tree.Root, base TreeVersion,
	l Write) (TreeVersion, [32]byte, error) {
	baseRootTree, err := r.Tree(tree.RootPath, base, l)
	if err != nil {
		return 0, [32]byte{}, err
	}
	baseHash := baseRootTree.Data().ContentHash

	iter, err := tree.Walk2(root, r.Root(base, l))
	if err != nil {
		return 0, [32]byte{}, err
	}
	var newV TreeVersion
	g := newOnceGrabber(&newV, r.id, l)
	var rootHash [32]byte
	savedRoot := false
	for iter.CanGet() {
		diff := iter.GetDiff()
		switch diff.Type {
		case tree.DiffTypeUndefined:
			err = iter.Next()
			if err != nil {
				return newV, rootHash, err
			}
			continue
		case tree.DiffTypeDeleted, tree.DiffTypeNoChange:
			iter.SkipChildrenOnNext()
			err = iter.Next()
			if err != nil {
				return newV, rootHash, err
			}
			continue
		case tree.DiffTypeCreated, tree.DiffTypeAnyModified:
		default:
			panic("unexpected diff type")
		}

		trPath, trPathDepth, visitStatus, tr := iter.Get()
		// Should be skipped by the undefined diff
		if tr.IsRemovedChild() {
			panic("unexpected removedChild")
		}
		// A file<->dir swap is reported before the new dir's data is
		// complete. Keep iterating until it is.
		if !tr.DataIsComplete() {
			err = iter.Next()
			if err != nil {
				return newV, rootHash, err
			}
			continue
		}
		// Skip directories that have no children; as those are not saved
		if trPathDepth != 0 &&
			tr.Data().IsDir &&
			len(tr.Data().ChildrenBaseNames) == 0 {
			err = iter.Next()
			if err != nil {
				return newV, rootHash, err
			}
			continue
		}
		// Only save directories in the second visit. This is not strictly
		// necessary, but we just need some way to avoid saving them twice.
		if tr.Data().IsDir && visitStatus == tree.FirstVisit {
			err = iter.Next()
			if err != nil {
				return newV, rootHash, err
			}
			continue
		}
		// Walk2 reports a file again on the second visit of the dir it replaced
		if !tr.Data().IsDir && visitStatus == tree.SecondVisit {
			err = iter.Next()
			if err != nil {
				return newV, rootHash, err
			}
			continue
		}

		// Populate the rootHash when we're at the root
		if trPathDepth == 0 {
			rootHash = tr.Data().ContentHash
			savedRoot = true
		}

		// Save the file/dir
		err = g.GrabOnce()
		if err != nil {
			return newV, rootHash, err
		}
		err = r.saveDirOrSaveFile(newV, trPath, tr, base, l)
		if err != nil {
			return newV, rootHash, err
		}

		// Keep iterating
		err = iter.Next()
		if err != nil {
			return newV, rootHash, err
		}
	}
	if !savedRoot {
		return base, baseHash, ErrNoChange
	}
	return newV, rootHash, nil
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
	g := newOnceGrabber(&newV, r.id, l)
	baseRoot := r.getRoot_(baseV, l)

	// The trees saved with newV
	newTreePaths := make(map[string]bool)

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
			err = g.GrabOnce()
			if err != nil {
				return
			}
			err = r.saveTreeBlob(trPath, tr, newV, l)
			if err != nil {
				return
			}
			err = r.saveTree(trPath, tr, newV,
				/*treeBlobVersion*/ newV, hasTreeOnBase, treeOnBase, newTreePaths, l)
			if err != nil {
				return
			}
		} else {
			err = g.GrabOnce()
			if err != nil {
				return
			}
			err = r.saveTree(trPath, tr, newV,
				/*treeBlobVersion*/ 0, hasTreeOnBase, treeOnBase, newTreePaths, l)
			if err != nil {
				return
			}
		}

		err = iter.Next()
		if err != nil {
			return
		}
	}

	if len(newTreePaths) == 0 {
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
	v uint64,
	treeBlobVersion uint64,
	hasTreeOnBase bool,
	treeOnBase tree_,
	newTreePaths map[string]bool,
	l Write) error {
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
		childIsNew := newTreePaths[path.Join(newTreePath, childName)]

		// The version of the child will depend on wheather the child is
		// new or not.
		// If the child is new, it was saved with v. Else, read the parent
		// to figure out what its version of the children was
		if childIsNew {
			newTreeData.ChildrenVersions = append(
				newTreeData.ChildrenVersions, v)
		} else {
			// If the child is not new, we read it from the old version
			if !hasTreeOnBase {
				return errors.New("tree base not found")
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

	err = l.SetTreeData(r.quotaOwner, r.id, newTreePath, v, newTreeData)
	if err != nil {
		return err
	}
	newTreePaths[newTreePath] = true
	return nil
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

func (r repo) saveTreeBlob(treePath string, tr tree.Tree, v uint64, l Write) (err error) {
	if !shouldSaveTreeBlob(tr) {
		panic("tried to save blob of " + treePath)
	}
	wt, err := tr.GetFile()
	if err != nil {
		return
	}
	return l.SetTreeBlob(r.quotaOwner, r.id, treePath, v, wt)
}

// onceGrabber is a simpler helper that calls GrabRootTreeVersion only once.
// Its used so that we can populate a *TreeVersion variable only if needed
// when saving a new repository version to avoid grabbing a version to save
// an unmodified repository.
type onceGrabber struct {
	newV    *TreeVersion
	grabbed bool
	repoId  uint64
	l       Write
}

func newOnceGrabber(newV *TreeVersion, repoId uint64, l Write) *onceGrabber {
	return &onceGrabber{
		newV:    newV,
		grabbed: false,
		repoId:  repoId,
		l:       l,
	}
}

// Calls GrabRootTreeVersion if it wanst called yet
func (g *onceGrabber) GrabOnce() error {
	if g.grabbed {
		return nil
	}
	v, err := g.l.GrabRootTreeVersion(g.repoId)
	if err != nil {
		return err
	}
	g.grabbed = true
	(*g.newV) = v
	return nil
}