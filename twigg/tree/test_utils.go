package tree

import (
	"bytes"
	"io"
	"path"
	"reflect"
	"strings"
	"testing"
)

func checkTree(depth uint32, tr Tree, baseName string, isRemovedChild, isDir, isExecutableFile bool, childrenNames []string,
	isFullyKnown bool, isSymlink bool, symlinkTarget string, size int64, t *testing.T) {
	t.Helper()
	if isRemovedChild != tr.IsRemovedChild() {
		t.Fatalf("expected isRemovedChild %v got %v", isRemovedChild, tr.IsRemovedChild())
	}
	if isRemovedChild {
		return
	}
	dt := tr.Data()
	if dt.BaseName != baseName {
		t.Fatalf("wrong name: expected %s got %s", baseName, tr.Data().BaseName)
	}
	if dt.Depth != depth {
		t.Fatalf("wrong depth for tree %s: expected %d got %d", baseName, depth, dt.Depth)
	}
	if dt.IsDir != isDir {
		t.Fatalf("wrong isDir for tree %s: expected %v got %v", baseName, isDir, dt.IsDir)
	}
	if dt.IsExectuableFile != isExecutableFile {
		t.Fatalf("wrong IsExectuableFile for tree %s: expected %v got %v", baseName, isExecutableFile, dt.IsExectuableFile)
	}
	if isFullyKnown != tr.DataIsComplete() {
		t.Fatalf("expected isFullyKnown %v got %v", isFullyKnown, tr.DataIsComplete())
	}
	if isFullyKnown {
		if dt.Size != size {
			t.Fatalf("wrong size: expected %d got %d", size, dt.Size)
		}
	}
	if len(childrenNames) != 0 || len(tr.Data().ChildrenBaseNames) != 0 {
		if !reflect.DeepEqual(tr.Data().ChildrenBaseNames, childrenNames) {
			t.Fatalf("expected children %v\n got %v\n",
				childrenNames, tr.Data().ChildrenBaseNames)
		}

	}
	if tr.Data().IsSymlink != isSymlink {
		t.Fatalf("expected %s isSymlink to be %v, got %v", baseName, isSymlink,
			tr.Data().IsSymlink)
	}

	if tr.Data().SymlinkTarget != symlinkTarget {
		t.Fatalf("expected %s isSymlinkTarget to be %v, got %v",
			baseName, symlinkTarget,
			tr.Data().SymlinkTarget)
	}
}

func checkTreeIter(ti Iterator, expectedPath string, depth uint32,
	isRemovedChild, isDir bool, childrenNames []string,
	isFullyKnown bool, size int64, t *testing.T) {
	t.Helper()
	if !ti.CanGet() {
		t.Fatalf("unable to get tree %s", expectedPath)
	}
	gotP, gotD, _, tr := ti.Get()
	if gotP != expectedPath {
		t.Fatalf("expected path %s got %s", expectedPath, gotP)
	}
	if gotD != depth {
		t.Fatalf("expected depth %d got %d", depth, gotD)
	}

	checkTree(depth, tr, path.Base(expectedPath), isRemovedChild,
		isDir,
		/*isExecutableFile*/ false,
		childrenNames, isFullyKnown,
		/*isSymlink*/ false,
		/*simlinkTarget*/ "", size, t)
}
func checkTreeIterSymlink(ti Iterator, expectedPath string, depth uint32, symlinkTarget string, t *testing.T) {
	if !ti.CanGet() {
		t.Fatalf("unable to get tree %s", expectedPath)
	}
	gotP, gotD, _, tr := ti.Get()
	if gotP != expectedPath {
		t.Fatalf("expected path %s got %s", expectedPath, gotP)
	}
	if gotD != depth {
		t.Fatalf("expected depth %d got %d", depth, gotD)
	}
	checkTree(depth, tr, path.Base(expectedPath),
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*isExecutableFile*/ false,
		/*childNames*/ nil,
		/*isFullyKnown*/ true,
		/*isSymlink*/ true,
		/*simlinkTarget*/ symlinkTarget,
		/*size*/ 0, t)
}

func newFakeFile(path, content string, isExecutable bool, unixMilisModTime int64) Tree {
	return fakeFile{path: path, content: content,
		isExecutable: isExecutable, unixMilisModTime: unixMilisModTime}
}

type fakeFile struct {
	path             string
	content          string
	isExecutable     bool
	unixMilisModTime int64
}

func (f fakeFile) IsRemovedChild() bool {
	return false
}
func (f fakeFile) DataIsComplete() bool {
	return true
}
func (f fakeFile) IsSymLinkTo() string {
	return ""
}
func (f fakeFile) Data() Data {
	return NewData(
		path.Base(f.path),
		/*depth*/ uint32(strings.Count(f.path, "/")+1),
		/*isDir*/ false,
		f.isExecutable,
		/*isText*/ true,
		/*hasConflicts*/ false,
		/*hasChildWithConflicts*/ false,
		/*isSymlink*/ false,
		/*symlinkTarget*/ "",
		/*size*/ int64(len(f.content)),
		/*lastModified*/ 0,
		/*contentHash*/ NewHasher().WriteBool(f.isExecutable).
			WriteString(f.content).Sum(),
		/*children*/ nil,
	)
}
func (f fakeFile) GetFile() (wt io.WriterTo, err error) {
	wt = bytes.NewBufferString(f.content)
	return
}

func newFakeSymlink(path, to string) Tree {
	return fakeSymlink{path: path, to: to}
}

type fakeSymlink struct {
	path string
	to   string
}

func (f fakeSymlink) IsRemovedChild() bool {
	return false
}
func (f fakeSymlink) DataIsComplete() bool {
	return true
}
func (f fakeSymlink) IsSymLinkTo() string {
	return f.to
}
func (f fakeSymlink) Data() Data {
	return NewData(
		path.Base(f.path),
		/*depth*/ uint32(strings.Count(f.path, "/")+1),
		/*isDir*/ false,
		/*isExecutableFile*/ false,
		/*isText*/ true,
		/*hasConflicts*/ false,
		/*hasChildWithConflicts*/ false,
		/*isSymlink*/ true,
		/*symlinkTarget*/ f.to,
		/*size*/ int64(0),
		/*lastModified*/ 0,
		/*contentHash*/ NewHasher().WriteBool(false).WriteString(SymlinkString(f.to)).Sum(),
		/*children*/ nil,
	)
}
func (f fakeSymlink) GetFile() (wt io.WriterTo, err error) {
	wt = bytes.NewBufferString(symlinkString(f.to))
	return
}

func newFakeDir(path_ string, children []string) Tree {
	d := uint32(strings.Count(path_, "/") + 1)
	if path_ == RootPath {
		d = 0
	}
	return fakeDir{
		hasCachedDt: false,
		dt:          NewUnknownDirData(path.Base(path_), d, children)}
}

func newFakeDirWithCachedData(dt Data) Tree {
	return fakeDir{
		hasCachedDt: true,
		dt:          dt,
	}
}

type fakeDir struct {
	hasCachedDt bool
	dt          Data
}

func (fd fakeDir) IsRemovedChild() bool {
	return false
}
func (fd fakeDir) DataIsComplete() bool {
	return fd.hasCachedDt
}
func (f fakeDir) IsSymLinkTo() string {
	return ""
}
func (fd fakeDir) Data() Data {
	return fd.dt
}
func (fd fakeDir) GetFile() (wt io.WriterTo, err error) {
	panic("called GetFile for directory")
}

func newFakeRoot() fakeRoot {
	return fakeRoot{
		trees: map[string]Tree{},
	}
}
func (f fakeRoot) AddFile(path, content string, unixMilisModTime int64) {
	f.trees[path] = newFakeFile(path, content, false, unixMilisModTime)
}
func (f fakeRoot) AddExecutableFile(path, content string, unixMilisModTime int64) {
	f.trees[path] = newFakeFile(path, content, true, unixMilisModTime)
}
func (f fakeRoot) AddSymlink(path, to string) {
	f.trees[path] = newFakeSymlink(path, to)
}
func (f fakeRoot) AddDir(path string, children []string) {
	f.trees[path] = newFakeDir(path, children)
}
func (f fakeRoot) AddDirWithCachedData(path string, md Data) {
	f.trees[path] = newFakeDirWithCachedData(md)
}

type fakeRoot struct {
	trees map[string]Tree
}

func (fr fakeRoot) Tree(relativePath string) (Tree, error) {
	tr, ok := fr.trees[relativePath]
	if !ok {
		return nil, ErrTreeNotFound
	}
	return tr, nil
}

func check(err error, t testing.TB) {
	if err != nil {
		t.Fatal("check failed:", err)
	}
}

func checkDiff(di ParallelIterator, dt DiffType, expectedPath string, depth uint32,
	isRemovedChild bool, isDir bool, isExecutableFile bool,
	childrenNames []string,
	isFullyComputed bool, size int64, t *testing.T) {
	p, gotDepth, _, tr := di.Get()
	if p != expectedPath {
		t.Fatalf("expected path %s got %s", expectedPath, p)
	}
	if gotDepth != depth {
		t.Fatalf("expected depth %d got %d", depth, gotDepth)
	}
	d := di.GetDiff()
	if d.Type != dt {
		t.Fatalf("wrong diff type for %s. Expected %v, got %v", expectedPath, dt, d.Type)
	}
	if di.CanGetA() {
		aPath, aDepth, _, _ := di.GetA()
		if aPath != p || aDepth != gotDepth {
			t.Fatalf("CanGetA but iterator is on other side")
		}
	}
	if di.CanGetB() {
		bPath, bDepth, _, _ := di.GetB()
		if bPath != p || bDepth != gotDepth {
			t.Fatalf("CanGetB but iterator is on other side")
		}
	}

	CheckTree(tr,
		path.Base(expectedPath), depth, isRemovedChild, isDir, isExecutableFile,
		childrenNames, isFullyComputed, size, t)
}

type completeRoot struct {
	treeByPath map[string]Tree
	t          testing.TB
}

func makeComplete(r Root, t testing.TB) Root {
	cr := completeRoot{
		treeByPath: map[string]Tree{},
		t:          t,
	}
	iter, err := Walk(r)
	if err != nil {
		t.Fatal(err)
	}
	for iter.CanGet() {
		trPath, _, _, tr := iter.Get()
		if tr.IsRemovedChild() || tr.DataIsComplete() {
			cr.treeByPath[trPath] = tr
		}
		err = iter.Next()
		if err != nil {
			t.Fatal(err)
		}
	}
	return cr
}

func (r completeRoot) Tree(path string) (Tree, error) {
	tr, ok := r.treeByPath[path]
	if ok {
		return tr, nil
	}
	return nil, ErrTreeNotFound
}

type treeMock struct {
	DataIsComplete_ bool
	Data_           Data
	IsFile          bool
	FileContent     string
}

func (tm treeMock) IsRemovedChild() bool {
	return false
}
func (tm treeMock) Data() Data {
	return tm.Data_
}
func (tm treeMock) DataIsComplete() bool {
	return tm.DataIsComplete_
}
func (tm treeMock) GetFile() (wt io.WriterTo, err error) {
	if !tm.IsFile {
		panic("tried to read file from non-file tree")
	}
	return bytes.NewBufferString(tm.FileContent), nil
}

type rootMock struct {
	treesByPath     map[string]Tree
	t               testing.TB
	treeMethodCalls []string
}

// Create a mock by passing the value of tree and isChildNotFoundErr
// for each path. It fails the test for any path that was not provided.
func newRootMock(trees map[string]Tree, t testing.TB) *rootMock {
	return &rootMock{
		treesByPath: trees,
		t:           t,
	}
}

func (rm *rootMock) Tree(path string) (Tree, error) {
	rm.treeMethodCalls = append(rm.treeMethodCalls, path)
	tr, ok := rm.treesByPath[path]
	if !ok {
		return nil, ErrTreeNotFound
	}
	return tr, nil
}
