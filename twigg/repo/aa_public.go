package repo

import (
	"errors"
	"io"
	"monorepo/twigg/tree"
	"monorepo/twigg/treev"
	"monorepo/twigg/workdir"
)

type Read interface {
	// Returns last version of treePath=root.RootPath
	GetLastVersionOfRootTree(repoId uint64) (v uint64, isNotFoundErr bool, err error)
	GetTreeData(repoId uint64, treePath string, v uint64) (td treev.TreeDataV, isNotFoundErr bool, err error)
	GetTreeBlob(repoId uint64, treePath string, v uint64) (r io.Reader, closeR func(), isNotFoundErr bool, err error)
}

type Write interface {
	Read
	SetTreeData(quotaOwner string, repoId uint64, treePath string, td treev.TreeDataV) (v uint64, err error)
	SetTreeBlob(quotaOwner string, repoId uint64, treePath string, wt io.WriterTo) (v uint64, err error)
}

const RootTreeVersion = 0

type Repo interface {
	// Save the first version.
	Init(l Write) (v TreeVersion, rootDirHash [32]byte, err error)
	// Saves a new version of the repository using `base` as the older
	// version of the tree. `base` can be any pervious version, but preffer
	// using a version that is similar to the current one for better
	// storage efficiency, since only the trees that changed wrt base will
	// be saved.
	// Returns ErrNoChange if tre tree didn't change compared to the parent.
	// In that case, returns the parent version.
	Save(root tree.Root, base TreeVersion, l Write) (v TreeVersion, rootDirHash [32]byte, err error)
	// Returns a root tree at the provided version
	Root(v TreeVersion, l Read) tree.Root
	// Load a version of the repo into a workir by writing/deleting files
	Load(v TreeVersion, wd workdir.Workdir, l Read) error
	// Creates a tree that is equivalent to b+(a-p):
	//
	// b + (a-p)
	// |
	// b      a
	// |      |
	// ~      p
	//        |
	//        ~
	// Returns ErrNoChange if a new tree is not created.
	// In that case, returns the latest tree version.
	// aLabel and bLabel are used for showing rebase conflicts if they happen.
	Rebase(a TreeVersion, aLabel string, b TreeVersion, bLabel string,
		p TreeVersion, l Write) (v TreeVersion, rootDirHash [32]byte, conflict bool, err error)
	// Returns true if `a` can be rebased into `b` without causing a conflict.
	// See the `Rebase` for an illustration
	CanRebaseWithoutConflict(a TreeVersion, b TreeVersion, p TreeVersion, l Read) (bool, error)

	// Returns an iterator over all files with the given
	// name that are located in directories whose contents were modified by A.
	// It is primarily intended for efficiently discovering configuration or
	// metadata files such as CI/CD definitions that are in changed paths.
	// The iterator makes no guarantees about the order in which matching files
	// are returned.
	SearchFileInChangedDirs(A, B TreeVersion, l Read, filename string) (FileInChangedDirsIter, error)

	// Returns a DeltaIter
	GetDelta(a, base TreeVersion, l Read) (DeltaIterWithDiff, error)
	// Just like Save, but for a DeltaIter.
	// It must use the same base that was used to creat the DeltaIter.
	SaveDelta(d DeltaIter, base TreeVersion, l Write) (v TreeVersion, rootDirHash [32]byte, err error)
}

// This is basically a "delta-compressed" tree iterator.
// It walks bottom-up (i.e. post-order traversal) and contains only nodes that
// are modified and their immediate children.
// I.e. if a node is the same in A and B, it'll appear once and its children
// will be ommited (you can get them from B).
type DeltaIter interface {
	CanGet() bool
	Get() (path string, pathDepth uint32, tr tree.Tree)
	Pop() error
}

type DeltaIterWithDiff interface {
	DeltaIter
	GetDiff() tree.Diff
}

// Returns all files with a certain name that are located in directories
// whose contents changed
type FileInChangedDirsIter struct {
	it *fileInChangedDirsIterator
}

// Exactly one of [isCreated, isModified, isDeleted] will be true.
// If isCreated -> tr and aTr are the A-side file, bTr is nil
// If isModified -> tr and aTr are the A-side file, bTr is the B-side tree
// If isDeleted -> tr and bTr are the B-side file, aTr is nil
func (m FileInChangedDirsIter) GetFile() (isCreated, isModified, isDeleted bool, path string, depth uint32, tr tree.Tree, aTr tree.Tree, bTr tree.Tree) {
	return m.it.GetFile()
}
func (m FileInChangedDirsIter) CanGet() bool {
	return m.it.CanGet()
}
func (m FileInChangedDirsIter) Next() error {
	return m.it.Next()
}

// `id` is used to avoid collisions, so that the methods can be used for
// different repositories.
func New(quotaOwner string, id uint64) Repo {
	return newRepo(quotaOwner, id)
}

var ErrNoChange = errors.New("no change")
var ErrInvalidIter = errors.New("invalid iterator")

type TreeVersion = uint64
