package tree

import (
	"bytes"
	diff3 "monorepo/twigg/diff/epiclabs-io"
	"reflect"
	"strings"
	"testing"
)

func TestWalkForEmptyRootDir(t *testing.T) {
	root := NewFakeRoot(t)
	root.AddDir(RootPath, nil)
	dfs, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	if !dfs.CanGet() {
		t.Fatal("should be able to get")
	}
	if _, _, s, _ := dfs.Get(); s != FirstVisit {
		t.Fatal("should be first visit")
	}
	CheckIterator(dfs, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ nil,
		/*isFullyKnown*/ true,
		/*size*/ 0,
		t)
	// Skipping the children is ok and won't change anything
	dfs.SkipChildrenOnNext()
	check(dfs.Next(), t)
	if dfs.CanGet() {
		t.Fatal("should be done")
	}
}

func TestPopWithoutRecursingIntoChildrenIsOk(t *testing.T) {
	root := NewFakeRoot(t)
	root.AddDir(RootPath, []string{"a"})
	root.AddDir("a", []string{"a.txt"})
	root.AddFile("a/a.txt", "a", 0)
	dfs, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}

	// Root is not fully known bc children were not yet processed
	CheckIterator(dfs, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)

	check(dfs.Next(), t)
	CheckIterator(dfs, "a",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a.txt"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)

	// Skipping is fine, but it'll make the root remain uncomputed
	dfs.SkipChildrenOnNext()
	check(dfs.Next(), t)
	CheckIterator(dfs, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
}

func TestWalkForRootWithTwoFiles(t *testing.T) {
	root := NewFakeRoot(t)
	root.AddDir(RootPath, []string{"a.txt", "link"})
	root.AddFile("a.txt", "aaa", 0)
	root.AddSymlink("link", "a.txt")
	dfs, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	if !dfs.CanGet() {
		t.Fatal("should be able to get")
	}
	CheckIterator(dfs, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a.txt", "link"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)

	check(dfs.Next(), t)
	CheckIterator(dfs, "a.txt",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*children*/ nil,
		/*isFullyKnown*/ true,
		/*size*/ 3,
		t)
	check(dfs.Next(), t)
	CheckIteratorSymlink(dfs, "link",
		/*depth*/ 1,
		/*symlinkTarget*/ "a.txt",
		t)

	// We should now be back at the root, which should be computed
	check(dfs.Next(), t)
	CheckIterator(dfs, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a.txt", "link"},
		/*isFullyKnown*/ true,
		/*size*/ 3,
		t)
	check(dfs.Next(), t)
	if dfs.CanGet() {
		t.Fatal("should be done")
	}

}

func TestWalkRootDataForManySubfolders(t *testing.T) {
	root := NewFakeRoot(t)
	root.AddDir(RootPath, []string{"a", "b"})
	root.AddDir("a", []string{"a.txt"})
	root.AddFile("a/a.txt", "aaaa", 0)
	root.AddDir("b", []string{"b.txt"})
	root.AddFile("b/b.txt", "bb", 0)

	dfs, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	for {
		_, _, s, tr := dfs.Get()
		// Keep iterating until we're at the root and it's fully computed
		if tr.Data().BaseName == RootPath && s == SecondVisit {
			break
		}
		check(dfs.Next(), t)
	}
	_, _, _, tr := dfs.Get()
	if !tr.DataIsComplete() {
		t.Fatal("root should be fully known")
	}
	if tr.Data().Size != 6 {
		t.Fatalf("expected size 6 for root, got %d", tr.Data().Size)
	}

	// Write true/false for exec permission followed by content
	aFileHash := NewHasher().WriteBool(false).WriteString("aaaa")
	aDirHash := NewHasher()
	aDirHash.WriteString("a.txt")
	aDirHash.WriteSum(aFileHash.Sum())

	bFileHash := NewHasher().WriteBool(false).WriteString("bb")
	bDirHash := NewHasher()
	bDirHash.WriteString("b.txt")
	bDirHash.WriteSum(bFileHash.Sum())

	rootHash := NewHasher()
	rootHash.WriteString("a")
	rootHash.WriteSum(aDirHash.Sum())
	rootHash.WriteString("b")
	rootHash.WriteSum(bDirHash.Sum())
	if tr.Data().ContentHash != rootHash.Sum() {
		t.Fatal("wrong root hash")
	}
}

func TestWalkRootWithOneFileWithCachedData(t *testing.T) {
	root := NewFakeRoot(t)
	root.AddDirWithCachedData(RootPath,
		Data{BaseName: RootPath, IsDir: true, Size: 3,
			ChildrenBaseNames: []string{"a.txt"}})
	root.AddFile("a.txt", "aaa", 0)
	dfs, err := walk(root)
	if err != nil {
		t.Fatal(err)
	}
	if !dfs.CanGet() {
		t.Fatal("should be able to get")
	}
	CheckIterator(dfs, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a.txt"},
		/*isFullyKnown*/ true,
		/*size*/ 3,
		t)

	// We Skip the children bc the node is already fully computed.
	dfs.SkipChildrenOnNext()
	check(dfs.Next(), t)
	if dfs.CanGet() {
		t.Fatal("whould be empty bc children were skipped")
	}

	// Re-do the same thing but without skipping. This should return all trees.
	dfs, _ = Walk(root)
	if !dfs.CanGet() {
		t.Fatal("should be able to get")
	}
	check(dfs.Next(), t)
	CheckIterator(dfs, "a.txt",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*children*/ nil,
		/*isFullyKnown*/ true,
		/*size*/ 3,
		t)
	check(dfs.Next(), t)
	CheckIterator(dfs, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a.txt"},
		/*isFullyKnown*/ true,
		/*size*/ 3,
		t)
}

func TestEmptyRebase(t *testing.T) {
	A := NewFakeRoot(t)
	A.AddDir(RootPath, nil)
	B := NewFakeRoot(t)
	B.AddDir(RootPath, nil)
	P := NewFakeRoot(t)
	P.AddDir(RootPath, nil)

	iter, err := Rebase(A, "A", B, "B", P)
	if err != nil {
		t.Fatal(err)
	}
	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 0,
		t)
	check(iter.Next(), t)
	if iter.CanGet() {
		t.Fatal("should be done")
	}
}

func TestSimpleRebaseUp(t *testing.T) {
	// P is empty
	P := NewFakeRoot(t)
	P.AddDir(RootPath, nil)
	// A creates a.txt=a and symlink to a
	// (use zzz just to force zzz to come after a.txt)
	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"a.txt", "zzzsla"})
	A.AddFile("a.txt", "a", 0)
	A.AddSymlink("zzzsla", "a.txt")
	// B creates b.txt=bb and symlink to b
	B := NewFakeRoot(t)
	B.AddDir(RootPath, []string{"b.txt", "zzzslb"})
	B.AddFile("b.txt", "bb", 0)
	B.AddSymlink("zzzslb", "b.txt")

	// Tree illustration:
	//
	// A (a.txt=a, sla)
	// |
	// P (empty)   B (b.txt=bb, slb)
	// ~           ~

	iter, err := Rebase(A, "A", B, "B", P)
	if err != nil {
		t.Fatal(err)
	}

	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a.txt", "b.txt", "zzzsla", "zzzslb"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	check(iter.Next(), t)
	CheckIterator(iter,
		"a.txt",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 1,
		t)
	check(iter.Next(), t)
	CheckIterator(iter,
		"b.txt",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 2,
		t)
	check(iter.Next(), t)
	CheckIteratorSymlink(iter,
		"zzzsla",
		/*depth*/ 1,
		/*symlinkTarget*/ "a.txt",
		t)
	check(iter.Next(), t)
	CheckIteratorSymlink(iter,
		"zzzslb",
		/*depth*/ 1,
		/*symlinkTarget*/ "b.txt",
		t)
	check(iter.Next(), t)
	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a.txt", "b.txt", "zzzsla", "zzzslb"},
		/*isFullyKnown*/ true,
		/*size*/ 3,
		t)

	check(iter.Next(), t)
	if iter.CanGet() {
		t.Fatal("should be done")
	}
}

func TestSimpleRebaseDown(t *testing.T) {
	// B is empty
	B := NewFakeRoot(t)
	B.AddDir(RootPath, nil)
	// P creates a.txt=a
	P := NewFakeRoot(t)
	P.AddDir(RootPath, []string{"a.txt"})
	P.AddFile("a.txt", "a", 0)
	// A keeps a.txt=a and creates b.txt=bb
	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"a.txt", "b.txt"})
	A.AddFile("a.txt", "a", 0)
	A.AddFile("b.txt", "bb", 0)

	// Tree illustration:
	// A (a.txt=a, b.txt=bb)
	// |
	// P (a.txt=a)
	// |
	// B (empty)
	// ~

	// When rebasing A into B, the rebased A should only contain b
	// (which it created when compared to its parent).
	iter, err := Rebase(A, "A", B, "B", P)
	if err != nil {
		t.Fatal(err)
	}

	// a.txt still appears bc its not fully known
	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a.txt", "b.txt"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	check(iter.Next(), t)
	CheckIterator(iter,
		"a.txt",
		/*depth*/ 1,
		/*isRemovedChild*/ true,
		/*isDir*/ false,
		/*children*/ []string{},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	check(iter.Next(), t)
	CheckIterator(iter,
		"b.txt",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 2,
		t)

	// Now that the root is fully known it should only show b.txt as a child
	check(iter.Next(), t)
	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"b.txt"},
		/*isFullyKnown*/ true,
		/*size*/ 2,
		t)
	check(iter.Next(), t)
	if iter.CanGet() {
		t.Fatal("should be done")
	}
}

func TestRebaseToTargetThatHasCommonFile(t *testing.T) {
	// P contains b.txt=bb
	P := NewFakeRoot(t)
	P.AddDir(RootPath, []string{"b.txt"})
	P.AddFile("b.txt", "bb", 0)
	// A creates a.txt (but still contains b.txt=bb)
	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"a.txt", "b.txt"})
	A.AddFile("a.txt", "a", 0)
	A.AddFile("b.txt", "bb", 0)
	// B contains b.txt=bb and c.txt=ccc
	B := NewFakeRoot(t)
	B.AddDir(RootPath, []string{"b.txt", "c.txt"})
	B.AddFile("b.txt", "bb", 0)
	B.AddFile("c.txt", "ccc", 0)

	// Tree illustration:
	//
	// A (a.txt=aaa, b.txt=bb)
	// |
	// P (b.txt=bb)             B (b.txt=bb, c.txt=ccc)
	// ~                         ~

	iter, err := Rebase(A, "A", B, "B", P)
	if err != nil {
		t.Fatal(err)
	}

	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a.txt", "b.txt", "c.txt"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	check(iter.Next(), t)
	CheckIterator(iter,
		"a.txt",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 1,
		t)
	check(iter.Next(), t)
	CheckIterator(iter,
		"b.txt",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 2,
		t)
	check(iter.Next(), t)
	CheckIterator(iter,
		"c.txt",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 3,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a.txt", "b.txt", "c.txt"},
		/*isFullyKnown*/ true,
		/*size*/ 6,
		t)

	check(iter.Next(), t)
	if iter.CanGet() {
		t.Fatal("should be done")
	}
}

func TestSimpleRebaseWithMerge(t *testing.T) {
	// P contains a.txt=123
	P := NewFakeRoot(t)
	P.AddDir(RootPath, []string{"a.txt"})
	P.AddFile("a.txt", "1\n2\n3", 0)
	// A modifies first line to X
	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"a.txt"})
	A.AddFile("a.txt", "X\n2\n3", 0)
	// B modifies third line to Y
	B := NewFakeRoot(t)
	B.AddDir(RootPath, []string{"a.txt"})
	B.AddFile("a.txt", "1\n2\nY", 0)

	iter, err := Rebase(A, "A", B, "B", P)
	if err != nil {
		t.Fatal(err)
	}
	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a.txt"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)

	check(iter.Next(), t)

	// Before checking the iter, check the actual merge result
	_, _, _, tr := iter.Get()
	f, _ := tr.GetFile()
	buff := bytes.NewBuffer(nil)
	f.WriteTo(buff)
	merged := buff.String()
	expected := "X\n2\nY"
	if merged != expected {
		t.Fatalf("expected merged `%s` got `%s`",
			strings.ReplaceAll(expected, "\n", "\\n"),
			strings.ReplaceAll(merged, "\n", "\\n"))
	}

	CheckIterator(iter,
		"a.txt",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 5,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a.txt"},
		/*isFullyKnown*/ true,
		/*size*/ 5,
		t)
	check(iter.Next(), t)
	if iter.CanGet() {
		t.Fatal("should be done")
	}
}

func TestSimpleRebaseConflict(t *testing.T) {
	// P contains a.txt=123
	P := NewFakeRoot(t)
	P.AddDir(RootPath, []string{"a.txt"})
	P.AddFile("a.txt", "1\n2\n3", 0)
	// A modifies first line to X
	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"a.txt"})
	A.AddFile("a.txt", "X\n2\n3", 0)
	// B modifies first line to Y
	B := NewFakeRoot(t)
	B.AddDir(RootPath, []string{"a.txt"})
	B.AddFile("a.txt", "Y\n2\n3", 0)

	iter, err := Rebase(A, "A", B, "B", P)
	if err != nil {
		t.Fatal(err)
	}
	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a.txt"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	_, _, _, tr := iter.Get()
	if tr.Data().HasConflicts {
		t.Fatal("root should not have conflicts")
	}

	// Check a.txt content
	check(iter.Next(), t)
	_, _, _, tr = iter.Get()
	if !tr.Data().HasConflicts {
		t.Fatal("should have conflicts")
	}
	if tr.Data().HasChildWithConflicts {
		t.Fatal("file has no children")
	}
	f, _ := tr.GetFile()
	buff := bytes.NewBuffer(nil)
	f.WriteTo(buff)
	merged := buff.String()
	if !strings.Contains(merged, diff3.ConflictStart) {
		t.Fatal("file should contain conflict markers")
	}

	// Root should now be marked to have child conflicts
	check(iter.Next(), t)
	_, _, _, tr = iter.Get()
	if tr.Data().BaseName != RootPath {
		t.Fatal("expected root tree")
	}
	if tr.Data().HasConflicts {
		t.Fatal("root should not have conflicts")
	}
	if !tr.Data().HasChildWithConflicts {
		t.Fatal("root should have children with conflicts")
	}

}

func TestRebaseSymlink(t *testing.T) {
	P := NewFakeRoot(t)
	// A: x -> a.txt
	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"x"})
	A.AddSymlink("x", "a.txt")
	// B: x -> b.txt
	B := NewFakeRoot(t)
	B.AddDir(RootPath, []string{"x"})
	B.AddSymlink("x", "b.txt")

	// Rebased value from A is taken: x -> a.txt
	iter, err := Rebase(A, "A", B, "B", P)
	if err != nil {
		t.Fatal(err)
	}
	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"x"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	_, _, _, tr := iter.Get()
	if tr.Data().HasConflicts {
		t.Fatal("root should not have conflicts")
	}
	check(iter.Next(), t)
	CheckIteratorSymlink(iter,
		"x",
		/*depth*/ 1,
		/*symlinkTarget*/ "a.txt",
		t)
}

func TestRebaseFileToDir(t *testing.T) {
	P := NewFakeRoot(t)
	// A: `a` is a file
	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"a"})
	A.AddFile("a", "aaa", 0)
	// B: `a` is a directory
	B := NewFakeRoot(t)
	B.AddDir(RootPath, []string{"a"})
	B.AddDir("a", []string{"b.txt"})
	B.AddFile("a/b.txt", "bbbb", 0)

	// Value from A overwrites
	iter, err := Rebase(A, "A", B, "B", P)
	if err != nil {
		t.Fatal(err)
	}
	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	_, _, _, tr := iter.Get()
	if tr.Data().HasConflicts {
		t.Fatal("root should not have conflicts")
	}
	check(iter.Next(), t)
	CheckIterator(iter, "a",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*children*/ nil,
		/*isFullyKnown*/ true,
		/*size*/ 3,
		t)
}

func TestRebaseDirToFile(t *testing.T) {
	P := NewFakeRoot(t)
	// A: `a` is a directory
	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"a"})
	A.AddDir("a", []string{"b.txt"})
	A.AddFile("a/b.txt", "bbbb", 0)
	// B: `a` is a directory
	B := NewFakeRoot(t)
	B.AddDir(RootPath, []string{"a"})
	B.AddFile("a", "aaa", 0)

	// Value from A overwrites
	iter, err := Rebase(A, "A", B, "B", P)
	if err != nil {
		t.Fatal(err)
	}
	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	_, _, _, tr := iter.Get()
	if tr.Data().HasConflicts {
		t.Fatal("root should not have conflicts")
	}
	check(iter.Next(), t)
	CheckIterator(iter, "a",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"b.txt"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
}

func TestSimpleWalkDirWithChildThatEndsUpNotFound(t *testing.T) {
	// Mock root:
	// root
	// |
	// a
	// |
	// b
	// |
	// c (not found child)
	//
	rootMock := newRootMock(
		/*trees*/ map[string]Tree{
			RootPath: treeMock{
				DataIsComplete_: false,
				Data_:           NewUnknownDirData(RootPath, 0, []string{"a"}),
			},
			"a": treeMock{
				DataIsComplete_: false,
				Data_:           NewUnknownDirData("a", 1, []string{"b"}),
			},
			"a/b": treeMock{
				DataIsComplete_: false,
				Data_:           NewUnknownDirData("b", 2, []string{"c"}),
			},
			"a/b/c": NewRemovedChildTree(),
		},
		t,
	)
	iter, err := Walk(rootMock)
	check(err, t)

	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)

	check(iter.Next(), t)
	CheckIterator(iter, "a",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"b"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, "a/b",
		/*depth*/ 2,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"c"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)

	check(iter.Next(), t)
	CheckIterator(iter, "a/b/c",
		/*depth*/ 3,
		/*isRemovedChild*/ true,
		/*isDir*/ true,
		/*children*/ []string{},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, "a/b",
		/*depth*/ 2,
		/*isRemovedChild*/ true,
		/*isDir*/ true,
		/*children*/ []string{},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, "a",
		/*depth*/ 1,
		/*isRemovedChild*/ true,
		/*isDir*/ true,
		/*children*/ []string{},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 0,
		t)
}

func TestWalkRootWithDirWithSomeNotFindChildren(t *testing.T) {
	// Mock root (* are not find children):
	// root
	// |
	// a
	// | \
	// b  c
	// |\
	// |  \
	// d*  e*
	//
	rootMock := newRootMock(
		/*trees*/ map[string]Tree{
			RootPath: treeMock{
				DataIsComplete_: false,
				Data_:           NewUnknownDirData(RootPath, 0, []string{"a"}),
			},
			"a": treeMock{
				DataIsComplete_: false,
				Data_:           NewUnknownDirData("a", 1, []string{"b", "c"}),
			},
			"a/c": treeMock{
				DataIsComplete_: true,
				Data_:           Data{BaseName: "c", Depth: 2, IsDir: false, Size: 5},
				FileContent:     "ccccc",
			},
			"a/b": treeMock{
				DataIsComplete_: false,
				Data_:           NewUnknownDirData("b", 2, []string{"d", "e"}),
			},
			"a/b/d": NewRemovedChildTree(),
			"a/b/e": NewRemovedChildTree(),
		},
		t,
	)
	iter, err := Walk(rootMock)
	check(err, t)

	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)

	check(iter.Next(), t)
	CheckIterator(iter, "a",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"b", "c"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, "a/b",
		/*depth*/ 2,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"d", "e"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, "a/b/d",
		/*depth*/ 3,
		/*isRemovedChild*/ true,
		/*isDir*/ true,
		/*children*/ []string{},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, "a/b/e",
		/*depth*/ 3,
		/*isRemovedChild*/ true,
		/*isDir*/ true,
		/*children*/ []string{},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, "a/b",
		/*depth*/ 2,
		/*isRemovedChild*/ true,
		/*isDir*/ true,
		/*children*/ []string{},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, "a/c",
		/*depth*/ 2,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 5,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, "a",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"c"},
		/*isFullyKnown*/ true,
		/*size*/ 5,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a"},
		/*isFullyKnown*/ true,
		/*size*/ 5,
		t)
}

func TestWalkWithAllChildrenDataInMemory(t *testing.T) {
	// Mock root with all children data in memory. In this case, the root
	// only has to provide the root node and all other ones are read from
	// memory.
	//
	// root
	// |
	// a
	// | \
	// b  c
	bData := Data{BaseName: "b", Depth: 2, IsDir: false, Size: 1}
	cData := Data{BaseName: "c", Depth: 2, IsDir: false, Size: 2}
	aData := Data{
		BaseName:               "a",
		Depth:                  1,
		IsDir:                  true,
		Size:                   3,
		ChildrenBaseNames:      []string{"b", "c"},
		HasChildrenData:        true,
		ChildrenData:           []Data{bData, cData},
		ChildrenDataIsComplete: []bool{true, true},
	}
	rootData := Data{
		BaseName:               RootPath,
		Depth:                  0,
		IsDir:                  true,
		Size:                   3,
		ChildrenBaseNames:      []string{"a"},
		HasChildrenData:        true,
		ChildrenData:           []Data{aData},
		ChildrenDataIsComplete: []bool{true},
	}
	rootMock := newRootMock(
		/*trees*/ map[string]Tree{
			RootPath: treeMock{
				DataIsComplete_: true,
				Data_:           rootData,
			},
		},
		t,
	)
	iter, err := Walk(rootMock)
	check(err, t)

	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a"},
		/*isFullyKnown*/ true,
		/*size*/ 3,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, "a",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"b", "c"},
		/*isFullyKnown*/ true,
		/*size*/ 3,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, "a/b",
		/*depth*/ 2,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 1,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, "a/c",
		/*depth*/ 2,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 2,
		t)

	if !reflect.DeepEqual(rootMock.treeMethodCalls, []string{RootPath}) {
		t.Fatalf("Tree method called for: %v", rootMock.treeMethodCalls)
	}
}

func TestWalkWithSomeChildrenDataInMemory(t *testing.T) {
	// Mock root with some children in memory
	//
	// root
	// |
	// a---\
	// |    \
	// b(m)  c(m)
	bData := Data{BaseName: "b", Depth: 2, IsDir: false, Size: 1}
	cData := Data{BaseName: "c", Depth: 2, IsDir: false, Size: 2}
	aData := Data{
		BaseName:               "a",
		Depth:                  1,
		IsDir:                  true,
		Size:                   3,
		ChildrenBaseNames:      []string{"b", "c"},
		HasChildrenData:        true,
		ChildrenData:           []Data{bData, cData},
		ChildrenDataIsComplete: []bool{true, true},
	}
	rootData := Data{
		BaseName:          RootPath,
		Depth:             0,
		IsDir:             true,
		Size:              3,
		ChildrenBaseNames: []string{"a"},
	}
	rootMock := newRootMock(
		/*trees*/ map[string]Tree{
			RootPath: treeMock{
				DataIsComplete_: false,
				Data_:           rootData,
			},
			"a": treeMock{
				DataIsComplete_: true,
				Data_:           aData,
			},
		},
		t,
	)
	iter, err := Walk(rootMock)
	check(err, t)

	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a"},
		/*isFullyKnown*/ false,
		/*size*/ 3,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, "a",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"b", "c"},
		/*isFullyKnown*/ true,
		/*size*/ 3,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, "a/b",
		/*depth*/ 2,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 1,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, "a/c",
		/*depth*/ 2,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 2,
		t)

	if !reflect.DeepEqual(rootMock.treeMethodCalls, []string{RootPath, "a"}) {
		t.Fatalf("Tree method called for: %v", rootMock.treeMethodCalls)
	}
}

func TestWalkWithNonCompleteChildInMemory(t *testing.T) {
	// Mock root with a child that is a directory that is in memory but
	// not yet complete.
	//
	// root
	// |
	// a(m)
	// |
	// b
	rootMock := newRootMock(
		/*trees*/ map[string]Tree{
			RootPath: treeMock{
				DataIsComplete_: false,
				Data_: Data{
					BaseName:          RootPath,
					Depth:             0,
					IsDir:             true,
					Size:              3,
					ChildrenBaseNames: []string{"a"},
					HasChildrenData:   true,
					ChildrenData: []Data{
						NewUnknownDirData("a", 1, []string{"b"}),
					},
					ChildrenDataIsComplete: []bool{false},
				},
			},
			"a/b": treeMock{
				DataIsComplete_: true,
				Data_: Data{
					BaseName: "b",
					Depth:    2,
					IsDir:    false,
					Size:     1,
				},
			},
		},
		t,
	)
	iter, err := Walk(rootMock)
	check(err, t)

	CheckIterator(iter, RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, "a",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"b"},
		/*isFullyKnown*/ false,
		/*size*/ 0,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, "a/b",
		/*depth*/ 2,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 1,
		t)
	check(iter.Next(), t)
	CheckIterator(iter, "a",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*children*/ []string{"b"},
		/*isFullyKnown*/ true,
		/*size*/ 1,
		t)
	// Note that it'll try to read `a` because that tree doesn't have
	// the children in the root tree. The tree walking tries that because it's
	// always preferable to read all children from a parent instead of
	// one-by-one. There's no guarantees in the public API of this, but
	// it does handle the case in which the child is only provided on the parent
	// (note that `a` is not registered in rootMock).
	if !reflect.DeepEqual(rootMock.treeMethodCalls, []string{
		RootPath, "a", "a/b"}) {
		t.Fatalf("Tree method called for: %v", rootMock.treeMethodCalls)
	}
}
