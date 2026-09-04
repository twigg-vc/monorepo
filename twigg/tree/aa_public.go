package tree

import (
	"errors"
	"io"
	"testing"
)

// Represents a file or a directory
type Tree interface {
	// Represents a tree that is purposefully removed, despite being mentioned
	// earlier as a possible child. If true, all other methods will panic.
	IsRemovedChild() bool
	// Indicates whether the data is fully known.
	// If not, data that depends on children (e.g. Size) and even the
	// list of children itself can't be trusted.
	DataIsComplete() bool
	// Returns data of this entry.
	Data() Data
	// Returns a WriterTo that can be used to write the contents of the file
	// somewhere. Should only be called for `IsDir=false` Trees
	GetFile() (wt io.WriterTo, err error)
}

type Data struct {
	// Last part of the path
	BaseName string
	// Depth in the directory tree (root is zero, increases from there)
	Depth uint32
	// Indicates whether this is a directory or a file
	IsDir bool
	// If its a file, indicates if its executable
	IsExectuableFile bool
	// True for text files and for symlinks.
	// It indicates we can "print" the content.
	IsText bool
	// Indicates the file has merge conflicts
	HasConflicts bool
	// Indicates a child has merge conflicts
	HasChildWithConflicts bool
	// Indicates this is a symlink file
	IsSymlink bool
	// Indicates the target ot the symlink
	SymlinkTarget string

	// BaseNames of the children, i.e. the last part of the path to the children
	ChildrenBaseNames []string

	// For files, indicates the file size. For directories, indicates the sum
	// of the sizes of all children.
	Size int64
	// For files, indicates the last time the file was modified. For dirs,
	// indicates the most recent time one of the children was modified.
	LastModifiedUnixMillis int64
	// Hash of the file or of all the contents of a dir
	ContentHash [32]byte

	// Indicates ChildrenData contains data of the immediate children.
	// When ChildrenData is provided, Walk reads it directly instead of
	// calling the Tree(path) method to read child data; which can drastically
	// improve performance since Tree(path) will usually involve reading
	// from disk. The Root.Tree method must be able to return data for
	// all the children here; they're made pre-available on a best-effort for
	// efficiency.
	HasChildrenData bool
	// See HasChildrenData
	ChildrenData []Data
	// Indicates it the i-th ChildrenData is complete
	ChildrenDataIsComplete []bool
}

// "Hijacks" the Data method so that it returns HasChildrenData=false
func Flatten(tr Tree) Tree {
	return flatten{tr}
}

// Name used to represent the root
const RootPath = "."

type Root interface {
	// Gets the tree at the provided relative path.
	// The path must be provided in forward slashes (none of that windows BS).
	// Must return ErrTreeNotFound for not-found trees.
	Tree(path string) (tr Tree, err error)
}

// Flexible tree iterator.
// It walks the tree from top to bottom, allowing you to choose to recurse
// on the children (the default) or skip them when wanted. It automatically
// computes the tree Data after recursing into all the children.
type Iterator interface {
	// Returns false when done
	CanGet() bool
	// Get the current path, depth, visitStatus and tree.
	// Panics if CanGet() = false.
	Get() (path string, depth uint32, v VisitStatus, tr Tree)

	// If called, the children will be skipped when Next() is called.
	SkipChildrenOnNext()
	// Advances the iterator.
	// By default, recurses into the children. If SkipChildrenOnNext, the
	// children are ignored. Panics if CanGet() = false.
	Next() error
}

// Walks two trees in parallel to compare them.
// Note: a path can be reached twice, since dirs with children are
// visited before and after them. Use VisitStatus if to avoid double processing.
type ParallelIterator interface {
	// Sefaults to A, but fall back to B for deleted Trees
	Iterator
	// Returns true when the iterator is at A side or "tied" at A and B.
	// It doesn't guarantee the diff/tree is defined.
	CanGetA() bool
	// Returns the aPath, aDepth, aVisit, aTree.
	GetA() (string, uint32, VisitStatus, Tree)
	// Returns true when the iterator is at B side or "tied" at A and B.
	// It doesn't guarantee the diff/tree is defined.
	CanGetB() bool
	// Returns the bPath, bDepth, bVisit, bTree
	GetB() (string, uint32, VisitStatus, Tree)
	// Returns the current diff.
	GetDiff() Diff
	// Returns a boolean indicating if the unified diff is defined, and the
	// unified diff. The diff is not defined if neither tree is a file.
	// Only returns an error if something goes wrong when reading data.
	// Panics if !CanGet.
	// See WriteUnifiedDiff impementation for usage example.
	GetTextDiff() (unifiedDiff []byte, nAdd, nRemoved, nChanged int64, ok bool, err error)
}

// Returns an iterator to traverse the tree
func Walk(root Root) (Iterator, error) {
	return walk(root)
}

// Returns an iterator that traverses two trees in parallel
func Walk2(A, B Root) (ParallelIterator, error) {
	return walkParallel(A, B)
}

// Returns an iterator that traverses two trees in parallel, but only
// considers the changes that each tree has with respect to a base.
// For the current implementation, the base Roots (BaseA and BaseB) must only
// provide trees with `DataIsComplete() = true`. We could remove this
// requirement in the future, but it would require the implementation to be
// changed so that the bases are also walked simultaneously.
func Walk4(A, BaseA, B, BaseB Root) (ParallelIterator, error) {
	return walk4(A, BaseA, B, BaseB)
}

// Returns an iterator that represents the rebase of a into b with p as
// a's parent. aLabel and bLabel are used for showing rebase conflicts
// if they happen.
func Rebase(a Root, aLabel string, b Root, bLabel string, p Root) (Iterator, error) {
	return rebase(a, aLabel, b, bLabel, p)
}

// Gets the unified diff for a specific file between two roots.
// Returns ErrTreeNotFound if a file is not found in none of the roots.
func GetPathUnifiedDiff(filePath string, a Root, b Root) ([]byte, error) {
	return getPathUnifiedDiff(filePath, a, b)
}

// Writes the total unified diff between two roots.
// Non-changed trees are skipped.
func WriteUnifiedDiff(a Root, b Root, w io.Writer) error {
	return writeUnifiedDiff(a, b, w)
}

// TotalDiffCounts contains data about the difference between two roots
type TotalDiffCounts struct {
	LinesCreated  int64
	LinesDeleted  int64
	LinesModified int64
	FilesCreated  int64
	FilesDeleted  int64
	FilesModified int64
}

// Walks two roots in paralel to count num of files/lines changed
func CountDiffs(A, B Root) (TotalDiffCounts, error) {
	return countDiffs(A, B)
}

// Hasher used to compute the tree hashes
type Hasher interface {
	// Never returns error
	Write(b []byte) (int, error)
	// Returns a reference to iself for easy inlining
	WriteString(s string) Hasher
	// Returns a reference to iself for easy inlining
	WriteBytes(b []byte) Hasher
	// Returns a reference to iself for easy inlining
	WriteBool(b bool) Hasher
	// Returns a reference to iself for easy inlining
	WriteSum(b [32]byte) Hasher
	// Returns the sum (i.e. the actual hash value)
	Sum() [32]byte
}

// Returns the hasher in an empty state
func NewHasher() Hasher {
	return newHasher()
}

// Returns data of a tree that is fully known
func NewData(baseName string, depth uint32, isDir, isExectuableFile, isText, hasConflicts, hasChildWithConflicts bool,
	isSymLink bool, symlinkTarget string,
	size int64, lastModifiedUnixMillis int64, contentHash [32]byte, children []string) Data {
	return Data{
		BaseName:               baseName,
		Depth:                  depth,
		IsDir:                  isDir,
		IsExectuableFile:       isExectuableFile,
		IsText:                 isText,
		HasConflicts:           hasConflicts,
		HasChildWithConflicts:  hasChildWithConflicts,
		IsSymlink:              isSymLink,
		SymlinkTarget:          symlinkTarget,
		Size:                   size,
		LastModifiedUnixMillis: lastModifiedUnixMillis,
		ContentHash:            contentHash,
		ChildrenBaseNames:      children,
	}
}

// Returns data that represents a directory for which the whole data is not
// yet fully known as it depends of children data.
func NewUnknownDirData(baseName string, depth uint32, children []string) Data {
	return Data{BaseName: baseName, Depth: depth,
		IsDir: true, ChildrenBaseNames: children, IsText: false, IsSymlink: false}
}

// Returns a Tree that returns IsRemovedChild()=true and panics in all other methods
func NewRemovedChildTree() Tree {
	return removedChildTree{}
}

// Returns if the trees are equal based on their metadata.
// Panics if any of the trees is not fully known yet
func IsEqual(tr1, tr2 Tree) bool {
	return isEqual(tr1, tr2)
}

// Returns a string representation of a symlink.
// This can be used to display to users and also to compute a "hash" of the
// symlink.
func SymlinkString(target string) string {
	return symlinkString(target)
}

type VisitStatus int

const (
	FirstVisit VisitStatus = iota
	SecondVisit
)

type DiffType int

const (
	// Must call Next() because the current state is undefined
	DiffTypeUndefined DiffType = iota
	// Tree was created
	DiffTypeCreated
	// Tree was deleted
	DiffTypeDeleted
	// Tree was modified in any way (content, dir-ness, exec)
	DiffTypeAnyModified
	// Tree did not change in any way (content, dir-ness, exec)
	DiffTypeNoChange
)

type Diff struct {
	Type DiffType
	Data Data
}

var (
	// Use to indicate non exsting tree
	ErrTreeNotFound = errors.New("tree not found")
)

// Fake root for testing
type FakeRoot interface {
	Root
	AddFile(path, content string, unixMilisModTime int64)
	AddExecutableFile(path, content string, unixMilisModTime int64)
	AddSymlink(path, to string)
	AddDir(path string, children []string)
	AddDirWithCachedData(path string, md Data)
}

// Create a fake root for testing
func NewFakeRoot(t testing.TB) FakeRoot {
	return newFakeRoot()
}

// This takes in a root and changes its behavior so that
// it only returns trees with `DataIsComplete() = true`.
// This has terrible performance as it walks the whole tree every time
// to pre-compute the data, and should be used for testing only.
func MakeComplete(r Root, t testing.TB) Root {
	return makeComplete(r, t)
}

// Helper to check a tree in tests
func CheckTree(tr Tree, baseName string, depth uint32,
	isRemovedChild bool, isDir, isExecutableFile bool, childrenNames []string,
	isFullyKnown bool, size int64, t *testing.T) {
	checkTree(depth, tr, baseName, isRemovedChild, isDir, isExecutableFile, childrenNames, isFullyKnown,
		/*isSymlink*/ false,
		/*symlinkTarget*/ "", size, t)
}

// Helper to check a tree iter in tests
func CheckIterator(ti Iterator, path string, depth uint32, isRemovedChild, isDir bool, childrenNames []string,
	isFullyKnown bool, size int64, t *testing.T) {
	t.Helper()
	checkTreeIter(ti, path, depth, isRemovedChild, isDir, childrenNames, isFullyKnown, size, t)
}

// Helper to check that a tree iterator is currently on a symlink
func CheckIteratorSymlink(ti Iterator, path string, depth uint32, symlinkTarget string, t *testing.T) {
	checkTreeIterSymlink(ti, path, depth, symlinkTarget, t)
}

// Helper to check diff
func CheckDiff(di ParallelIterator, dt DiffType, path string, depth uint32,
	isRemovedChild bool, isDir bool, isExecutableFile bool,
	childrenNames []string,
	isFullyComputed bool, size int64, t *testing.T) {
	checkDiff(di, dt, path, depth, isRemovedChild, isDir, isExecutableFile,
		childrenNames, isFullyComputed, size, t)
}

const (
	MaxFileSizeToDiff = 1024 * 1024 // 1MB
)

// Returns a "placeholder" unified diff that just represents the file is
// binary and can't be diffed.
func FakeDiffForBinaryFiles(filename string) []byte {
	return fakeDiffForBinaryFiles(filename)
}
