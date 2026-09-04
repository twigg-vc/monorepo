package tree

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestBothEmpty(t *testing.T) {

	// Auxiliary function to test advancing with many options
	testOnce := func(advanceMode int) {
		r0 := NewFakeRoot(t)
		r0.AddDir(RootPath, nil)
		r1 := NewFakeRoot(t)
		r1.AddDir(RootPath, nil)
		di, err := Walk2(r0, r1)
		check(err, t)
		checkDiff(di,
			DiffTypeNoChange,
			RootPath,
			/*depth*/ 0,
			/*isRemovedChild*/ false,
			/*isDir*/ true,
			/*isExecutableFile=*/ false,
			/*children*/ []string{},
			/*isFullyComputed*/ true,
			/*size*/ 0,
			t)
		switch advanceMode {
		case 0:
			di.SkipChildrenOnNext()
			di.Next()
		case 1:
			di.Next()
		default:
			panic("unsuported advance mode")
		}
		if di.CanGet() {
			t.Fatal("should be done")
		}
	}
	testOnce(0)
	testOnce(1)
}

func TestSingleFileAdded(t *testing.T) {
	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"a.txt"})
	A.AddFile("a.txt", "aa", 0)

	B := NewFakeRoot(t)
	B.AddDir(RootPath, nil)

	di, err := Walk2(A, B)
	check(err, t)

	// The trees have a != num of children, but even in that scenario the
	// diff is still unknown bc the children might all be empty dirs (which is
	// as if they didn't exist).
	checkDiff(di,
		DiffTypeUndefined,
		RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"a.txt"},
		/*isFullyComputed*/ false,
		/*size*/ 0,
		t)

	check(di.Next(), t)
	checkDiff(di,
		DiffTypeCreated,
		"a.txt",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*isExecutableFile=*/ false,
		/*children*/ []string{},
		/*isFullyComputed*/ true,
		/*size*/ 2,
		t)

	check(di.Next(), t)
	checkDiff(di,
		DiffTypeAnyModified,
		RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"a.txt"},
		/*isFullyComputed*/ true,
		/*size*/ 2,
		t)
	check(di.Next(), t)
	if di.CanGet() {
		t.Fatal("should be done")
	}
}

func TestSingleFileDeleted(t *testing.T) {
	// Same as "SingleFileAdded" but in reverse

	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"a.txt"})
	A.AddFile("a.txt", "aa", 0)

	B := NewFakeRoot(t)
	B.AddDir(RootPath, nil)

	di, err := Walk2(B, A)
	check(err, t)

	checkDiff(di,
		DiffTypeUndefined,
		RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{},
		/*isFullyComputed*/ true,
		/*size*/ 0,
		t)

	check(di.Next(), t)
	checkDiff(di,
		DiffTypeDeleted,
		"a.txt",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*isExecutableFile=*/ false,
		/*children*/ []string{},
		/*isFullyComputed*/ true,
		/*size*/ 2,
		t)

	// After popping back, the diff should be identified as modified
	// because now A must be fully computed
	check(di.Next(), t)
	checkDiff(di,
		DiffTypeAnyModified,
		RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{},
		/*isFullyComputed*/ true,
		/*size*/ 0,
		t)
	check(di.Next(), t)
	if di.CanGet() {
		t.Fatal("should be done")
	}
}

func TestDifferentFiles(t *testing.T) {
	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"a.txt"})
	A.AddFile("a.txt", "aa", 0)

	B := NewFakeRoot(t)
	B.AddDir(RootPath, []string{"b.txt"})
	B.AddFile("b.txt", "b", 0)

	di, err := Walk2(A, B)
	check(err, t)

	checkDiff(di,
		DiffTypeUndefined,
		RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"a.txt"},
		/*isFullyComputed*/ false,
		/*size*/ 0,
		t)

	check(di.Next(), t)
	checkDiff(di,
		DiffTypeCreated,
		"a.txt",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*isExecutableFile=*/ false,
		/*children*/ []string{},
		/*isFullyComputed*/ true,
		/*size*/ 2,
		t)

	check(di.Next(), t)
	checkDiff(di,
		DiffTypeDeleted,
		"b.txt",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*isExecutableFile=*/ false,
		/*children*/ []string{},
		/*isFullyComputed*/ true,
		/*size*/ 1,
		t)

	check(di.Next(), t)
	checkDiff(di,
		DiffTypeAnyModified,
		RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"a.txt"},
		/*isFullyComputed*/ true,
		/*size*/ 2,
		t)

	check(di.Next(), t)
	if di.CanGet() {
		t.Fatal("should be done")
	}
}

func TestManyDirsOnlyOneChange(t *testing.T) {
	B := NewFakeRoot(t)
	B.AddDir(RootPath, []string{"a", "b", "c"})
	B.AddDir("a", []string{"a.txt"})
	B.AddDir("b", []string{"b.txt"})
	B.AddDir("c", []string{"c.txt"})
	B.AddFile("a/a.txt", "a", 0)
	B.AddFile("b/b.txt", "bb", 0)
	B.AddFile("c/c.txt", "ccc", 0)

	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"a", "b", "c"})
	A.AddDir("a", []string{"a.txt"})
	A.AddDir("b", []string{"b.txt"})
	A.AddDir("c", []string{"c.txt"})
	A.AddFile("a/a.txt", "a", 0)
	A.AddFile("b/b.txt", "BBBBB", 0)
	A.AddFile("c/c.txt", "ccc", 0)

	di, err := Walk2(A, B)
	check(err, t)

	dif := di.GetDiff()
	for dif.Type != DiffTypeAnyModified {
		err = di.Next()
		check(err, t)
		dif = di.GetDiff()
	}
	checkDiff(di,
		DiffTypeAnyModified,
		"b/b.txt",
		/*depth*/ 2,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*isExecutableFile=*/ false,
		/*children*/ []string{},
		/*isFullyComputed*/ true,
		/*size*/ 5,
		t)

	check(di.Next(), t)
	checkDiff(di,
		DiffTypeAnyModified,
		"b",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"b.txt"},
		/*isFullyComputed*/ true,
		/*size*/ 5,
		t)

	check(di.Next(), t)
	checkDiff(di,
		DiffTypeUndefined,
		"c",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"c.txt"},
		/*isFullyComputed*/ false,
		/*size*/ 0,
		t)
	check(di.Next(), t)
	checkDiff(di,
		DiffTypeNoChange,
		"c/c.txt",
		/*depth*/ 2,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*isExecutableFile=*/ false,
		/*children*/ []string{},
		/*isFullyComputed*/ true,
		/*size*/ 3,
		t)
	check(di.Next(), t)
	checkDiff(di,
		DiffTypeNoChange,
		"c",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"c.txt"},
		/*isFullyComputed*/ true,
		/*size*/ 3,
		t)
	check(di.Next(), t)
	checkDiff(di,
		DiffTypeAnyModified,
		RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"a", "b", "c"},
		/*isFullyComputed*/ true,
		/*size*/ 9,
		t)
}

func TestDirToFile(t *testing.T) {
	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"fileOnA"})
	A.AddFile("fileOnA", "aa", 0)

	B := NewFakeRoot(t)
	B.AddDir(RootPath, []string{"fileOnA"})
	B.AddDir("fileOnA", []string{"a.txt"})
	B.AddFile("fileOnA/a.txt", "a", 0)

	di, err := Walk2(A, B)
	check(err, t)

	checkDiff(di,
		DiffTypeUndefined,
		RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"fileOnA"},
		/*isFullyComputed*/ false,
		/*size*/ 0,
		t)
	check(di.Next(), t)
	checkDiff(di,
		DiffTypeAnyModified,
		"fileOnA",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*isExecutableFile=*/ false,
		/*children*/ []string{},
		/*isFullyComputed*/ true,
		/*size*/ 2,
		t)

	check(di.Next(), t)
	checkDiff(di,
		DiffTypeDeleted,
		"fileOnA/a.txt",
		/*depth*/ 2,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*isExecutableFile=*/ false,
		/*children*/ []string{},
		/*isFullyComputed*/ true,
		/*size*/ 1,
		t)

	// fileOnA is visited for the second time now
	check(di.Next(), t)
	if _, _, visit, _ := di.Get(); visit != SecondVisit {
		t.Fatal("expected SecondVisit")
	}
	checkDiff(di,
		DiffTypeAnyModified,
		"fileOnA",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*isExecutableFile=*/ false,
		/*children*/ []string{},
		/*isFullyComputed*/ true,
		/*size*/ 2,
		t)

	check(di.Next(), t)
	checkDiff(di,
		DiffTypeAnyModified,
		RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"fileOnA"},
		/*isFullyComputed*/ true,
		/*size*/ 2,
		t)
	check(di.Next(), t)
	if di.CanGet() {
		t.Fatal("should be done")
	}
}

func TestFileToDir(t *testing.T) {

	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"dirOnA"})
	A.AddDir("dirOnA", []string{"a.txt"})
	A.AddFile("dirOnA/a.txt", "a", 0)

	B := NewFakeRoot(t)
	B.AddDir(RootPath, []string{"dirOnA"})
	B.AddFile("dirOnA", "aa", 0)

	di, err := Walk2(A, B)
	check(err, t)

	checkDiff(di,
		DiffTypeUndefined,
		RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"dirOnA"},
		/*isFullyComputed*/ false,
		/*size*/ 0,
		t)

	// Even though its data is not fully computed yet, the dir-ness is
	// already known and we now the file became a dir
	check(di.Next(), t)
	checkDiff(di,
		DiffTypeAnyModified,
		"dirOnA",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"a.txt"},
		/*isFullyComputed*/ false,
		/*size*/ 0,
		t)
	check(di.Next(), t)
	checkDiff(di,
		DiffTypeCreated,
		"dirOnA/a.txt",
		/*depth*/ 2,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*isExecutableFile=*/ false,
		/*children*/ []string{},
		/*isFullyComputed*/ true,
		/*size*/ 1,
		t)
	check(di.Next(), t)

	// dirOnA is visited for the second time now
	if _, _, visit, _ := di.Get(); visit != SecondVisit {
		t.Fatal("expected the repeated swap to be reported as SecondVisit")
	}
	checkDiff(di,
		DiffTypeAnyModified,
		"dirOnA",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"a.txt"},
		/*isFullyComputed*/ true,
		/*size*/ 1,
		t)
	check(di.Next(), t)
	checkDiff(di,
		DiffTypeAnyModified,
		RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"dirOnA"},
		/*isFullyComputed*/ true,
		/*size*/ 1,
		t)
	check(di.Next(), t)
	if di.CanGet() {
		t.Fatal("should be done")
	}
}

func TestGetPathUnifiedDiffErrTreeNotFound(t *testing.T) {
	emptyRoot := NewFakeRoot(t)
	emptyRoot.AddDir(RootPath, nil)

	rootWithFile := NewFakeRoot(t)
	rootWithFile.AddDir(RootPath, []string{"a"})
	rootWithFile.AddDir("a", []string{"a.txt"})
	rootWithFile.AddFile("a/a.txt", "aaa\n", 0)

	_, err := GetPathUnifiedDiff("non-existing-file", rootWithFile, emptyRoot)
	if err != ErrTreeNotFound {
		t.Fatalf("expected tree not found got %s", err)
	}
	_, err = GetPathUnifiedDiff("a", rootWithFile, emptyRoot)
	if err != ErrTreeNotFound {
		t.Fatalf("expected tree not found got %s", err)
	}
	_, err = GetPathUnifiedDiff("a/a.txt", rootWithFile, emptyRoot)
	if err != nil {
		t.Fatalf("got err for existing path %q: %s", "a/a.txt", err)
	}
}

func TestGetPathUnifiedDiffForSingleFileCreated(t *testing.T) {
	emptyRoot := NewFakeRoot(t)
	emptyRoot.AddDir(RootPath, nil)

	rootWithFile := NewFakeRoot(t)
	rootWithFile.AddDir(RootPath, []string{"a"})
	rootWithFile.AddDir("a", []string{"a.txt"})
	rootWithFile.AddFile("a/a.txt", "aaa\n", 0)

	diffBytes, err := GetPathUnifiedDiff("a/a.txt", rootWithFile, emptyRoot)
	check(err, t)
	checkDiffBytesContain("+aaa", diffBytes, t)

	diffBytes, err = GetPathUnifiedDiff("a/a.txt", emptyRoot, rootWithFile)
	check(err, t)
	checkDiffBytesContain("-aaa", diffBytes, t)
}

func TestGetTextDiffForSingleFileCreated(t *testing.T) {
	emptyRoot := NewFakeRoot(t)
	emptyRoot.AddDir(RootPath, nil)

	rootWithFile := NewFakeRoot(t)
	rootWithFile.AddDir(RootPath, []string{"a"})
	rootWithFile.AddDir("a", []string{"a.txt"})
	rootWithFile.AddFile("a/a.txt", "aaa\n", 0)

	buff := bytes.NewBuffer(nil)
	err := WriteUnifiedDiff(rootWithFile, emptyRoot, buff)
	check(err, t)

	checkDiffContains("+aaa", buff, t)
}

func TestGetPathUnifiedDiffForCreatedAndModifiedFiles(t *testing.T) {
	lowecaseRoot := NewFakeRoot(t)
	lowecaseRoot.AddDir(RootPath, []string{"a", "b.txt"})
	lowecaseRoot.AddDir("a", []string{"a.txt"})
	lowecaseRoot.AddFile("a/a.txt", "aaa", 0)
	lowecaseRoot.AddFile("b.txt", "bbb", 0)

	uppercaseRoot := NewFakeRoot(t)
	uppercaseRoot.AddDir(RootPath, []string{"a", "b.txt", "c"})
	uppercaseRoot.AddDir("a", []string{"a.txt"})
	uppercaseRoot.AddFile("a/a.txt", "AAA", 0)
	uppercaseRoot.AddFile("b.txt", "bbb", 0)
	uppercaseRoot.AddDir("c", []string{"c.txt"})
	uppercaseRoot.AddFile("c/c.txt", "CCC", 0)

	diffBytes, err := GetPathUnifiedDiff("b.txt", uppercaseRoot, lowecaseRoot)
	check(err, t)
	checkDiffBytesContain(" bbb", diffBytes, t)

	diffBytes, err = GetPathUnifiedDiff("a/a.txt", uppercaseRoot, lowecaseRoot)
	check(err, t)
	checkDiffBytesContain("+AAA", diffBytes, t)
	checkDiffBytesContain("-aaa", diffBytes, t)

	diffBytes, err = GetPathUnifiedDiff("c/c.txt", uppercaseRoot, lowecaseRoot)
	check(err, t)
	checkDiffBytesContain("+CCC", diffBytes, t)
}

func TestGetPathUnifiedDiffForFileThatTurnedToDir(t *testing.T) {
	rootWithDir := NewFakeRoot(t)
	rootWithDir.AddDir(RootPath, []string{"a"})
	rootWithDir.AddDir("a", []string{"a.txt"})
	rootWithDir.AddFile("a/a.txt", "aaa", 0)

	rootWithFile := NewFakeRoot(t)
	rootWithFile.AddDir(RootPath, []string{"a"})
	rootWithFile.AddFile("a", "aaa", 0)

	diffBytes, err := GetPathUnifiedDiff("a", rootWithFile, rootWithDir)
	check(err, t)
	checkDiffBytesContain("+aaa", diffBytes, t)
}

func TestGetTextDiffUnchanged(t *testing.T) {
	rootWithFile := NewFakeRoot(t)
	rootWithFile.AddDir(RootPath, []string{"a.txt"})
	rootWithFile.AddFile("a.txt", "aaa\n", 0)

	buff := bytes.NewBuffer(nil)
	err := WriteUnifiedDiff(rootWithFile, rootWithFile, buff)
	check(err, t)
	if buff.String() != "" {
		t.Fatalf("expected empty diff got %q", buff.String())
	}
}

func TestGetTextDiffOnlyOneMoreFile(t *testing.T) {
	cAndD := NewFakeRoot(t)
	cAndD.AddDir(RootPath, []string{"c", "d.txt"})
	cAndD.AddDir("c", []string{"c.txt"})
	cAndD.AddFile("c/c.txt", "ccc", 0)
	cAndD.AddFile("d.txt", "ddd", 0)

	dOnly := NewFakeRoot(t)
	dOnly.AddDir(RootPath, []string{"d.txt"})
	dOnly.AddFile("d.txt", "ddd", 0)

	buff := bytes.NewBuffer(nil)
	err := WriteUnifiedDiff(dOnly, cAndD, buff)
	check(err, t)

	checkDiffDoesntContain("d.txt", buff, t)
	checkDiffContains("c.txt", buff, t)
}

func TestGetTextDiffForCreatedAndModifiedFiles(t *testing.T) {
	lowecaseRoot := NewFakeRoot(t)
	lowecaseRoot.AddDir(RootPath, []string{"a", "b", "c", "d.txt"})
	lowecaseRoot.AddFile("d.txt", "ddd", 0)
	lowecaseRoot.AddDir("a", []string{"a.txt"})
	lowecaseRoot.AddDir("b", []string{"b.txt"})
	lowecaseRoot.AddDir("c", []string{"c.txt"})
	lowecaseRoot.AddFile("a/a.txt", "aaa", 0)
	lowecaseRoot.AddFile("b/b.txt", "bbb", 0)
	lowecaseRoot.AddFile("c/c.txt", "ccc", 0)

	uppercaseRoot := NewFakeRoot(t)
	uppercaseRoot.AddDir(RootPath, []string{"a", "b", "d.txt"})
	uppercaseRoot.AddFile("d.txt", "ddd", 0)
	uppercaseRoot.AddDir("a", []string{"a.txt"})
	uppercaseRoot.AddDir("b", []string{"b.txt"})
	uppercaseRoot.AddFile("a/a.txt", "AAA", 0)
	uppercaseRoot.AddFile("b/b.txt", "BBB", 0)

	buff := bytes.NewBuffer(nil)
	err := WriteUnifiedDiff(uppercaseRoot, lowecaseRoot, buff)
	check(err, t)
	fmt.Println(buff.String())

	checkDiffDoesntContain("d.txt", buff, t)
	checkDiffContains("-aaa", buff, t)
	checkDiffContains("+AAA", buff, t)
	checkDiffContains("-bbb", buff, t)
	checkDiffContains("+BBB", buff, t)
	checkDiffContains("-ccc", buff, t)
}

func TestGetTextDiffForFileThatTurnedToDir(t *testing.T) {
	rootWithDir := NewFakeRoot(t)
	rootWithDir.AddDir(RootPath, []string{"a"})
	rootWithDir.AddDir("a", []string{"a.txt"})
	rootWithDir.AddFile("a/a.txt", "aaa", 0)

	rootWithFile := NewFakeRoot(t)
	rootWithFile.AddDir(RootPath, []string{"a"})
	rootWithFile.AddFile("a", "aaa", 0)

	buff := bytes.NewBuffer(nil)
	err := WriteUnifiedDiff(rootWithFile, rootWithDir, buff)
	check(err, t)

	checkDiffContains("+aaa", buff, t)
}

func TestGetTextDiffForFileDirSwapIsWrittenOnlyOnce(t *testing.T) {
	// A file<->dir swap is reached twice by the walk (the dir side is
	// visited before and after its children), but its diff must be
	// written only once
	rootWithDir := NewFakeRoot(t)
	rootWithDir.AddDir(RootPath, []string{"a"})
	rootWithDir.AddDir("a", []string{"a.txt"})
	rootWithDir.AddFile("a/a.txt", "inner content", 0)

	rootWithFile := NewFakeRoot(t)
	rootWithFile.AddDir(RootPath, []string{"a"})
	rootWithFile.AddFile("a", "outer content", 0)

	// dir -> file
	buff := bytes.NewBuffer(nil)
	check(WriteUnifiedDiff(rootWithFile, rootWithDir, buff), t)
	if n := strings.Count(buff.String(), "+outer content"); n != 1 {
		t.Fatalf("expected %q once in diff, got %d times:\n%s",
			"+outer content", n, buff.String())
	}

	// file -> dir
	buff = bytes.NewBuffer(nil)
	check(WriteUnifiedDiff(rootWithDir, rootWithFile, buff), t)
	if n := strings.Count(buff.String(), "-outer content"); n != 1 {
		t.Fatalf("expected %q once in diff, got %d times:\n%s",
			"-outer content", n, buff.String())
	}
}

func TestSimpleWalk4(t *testing.T) {

	// A creates a.txt
	ABase := NewFakeRoot(t)
	ABase.AddDir(RootPath, nil)
	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"a.txt"})
	A.AddFile("a.txt", "a", 0)
	// B creates b.txt
	BBase := NewFakeRoot(t)
	BBase.AddDir(RootPath, nil)
	B := NewFakeRoot(t)
	B.AddDir(RootPath, []string{"b.txt"})
	B.AddFile("b.txt", "b", 0)

	di, err := Walk4(A, MakeComplete(ABase, t), B, MakeComplete(BBase, t))
	check(err, t)
	checkDiff(di,
		DiffTypeUndefined,
		RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"a.txt"},
		/*isFullyComputed*/ false,
		/*size*/ 0,
		t)
	check(di.Next(), t)
	checkDiff(di,
		DiffTypeCreated,
		"a.txt",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*isExecutableFile=*/ false,
		/*children*/ []string{},
		/*isFullyComputed*/ true,
		/*size*/ 1,
		t)
	check(di.Next(), t)
	checkDiff(di,
		DiffTypeDeleted,
		"b.txt",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*isExecutableFile=*/ false,
		/*children*/ []string{},
		/*isFullyComputed*/ true,
		/*size*/ 1,
		t)
	check(di.Next(), t)
	checkDiff(di,
		DiffTypeAnyModified,
		RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"a.txt"},
		/*isFullyComputed*/ true,
		/*size*/ 1,
		t)
}

func TestSimpleWalk4WithDirectories(t *testing.T) {

	// A creates a/b/file1.txt
	ABase := NewFakeRoot(t)
	ABase.AddDir(RootPath, nil)
	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"a"})
	A.AddDir("a", []string{"b"})
	A.AddDir("a/b", []string{"file1.txt"})
	A.AddFile("a/b/file1.txt", "a", 0)
	// B creates a/b/file2.txt
	BBase := NewFakeRoot(t)
	BBase.AddDir(RootPath, nil)
	B := NewFakeRoot(t)
	B.AddDir(RootPath, []string{"a"})
	B.AddDir("a", []string{"b"})
	B.AddDir("a/b", []string{"file2.txt"})
	B.AddFile("a/b/file2.txt", "b", 0)

	di, err := Walk4(A, MakeComplete(ABase, t), B, MakeComplete(BBase, t))
	check(err, t)
	checkDiff(di,
		DiffTypeUndefined,
		RootPath,
		/*depth*/ 0,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"a"},
		/*isFullyComputed*/ false,
		/*size*/ 0,
		t)
	check(di.Next(), t)
	checkDiff(di,
		DiffTypeUndefined,
		"a",
		/*depth*/ 1,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"b"},
		/*isFullyComputed*/ false,
		/*size*/ 0,
		t)
	check(di.Next(), t)
	checkDiff(di,
		DiffTypeUndefined,
		"a/b",
		/*depth*/ 2,
		/*isRemovedChild*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"file1.txt"},
		/*isFullyComputed*/ false,
		/*size*/ 1,
		t)
	check(di.Next(), t)
	checkDiff(di,
		DiffTypeCreated,
		"a/b/file1.txt",
		/*depth*/ 3,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*isExecutableFile=*/ false,
		/*children*/ []string{},
		/*isFullyComputed*/ true,
		/*size*/ 1,
		t)
	check(di.Next(), t)
	checkDiff(di,
		DiffTypeDeleted,
		"a/b/file2.txt",
		/*depth*/ 3,
		/*isRemovedChild*/ false,
		/*isDir*/ false,
		/*isExecutableFile=*/ false,
		/*children*/ []string{},
		/*isFullyComputed*/ true,
		/*size*/ 1,
		t)
}

func TestLargeWalk4(t *testing.T) {

	ABase := NewFakeRoot(t)
	ABase.AddDir(RootPath, []string{"a1", "a2", "a3.txt", "a4.txt"})
	ABase.AddDir("a1", []string{"a11.txt"})
	ABase.AddFile("a1/a11.txt", "1", 0)
	ABase.AddDir("a2", []string{"a21.txt", "a22.txt"})
	ABase.AddFile("a2/a21.txt", "1", 0)
	ABase.AddFile("a2/a22.txt", "2", 0)
	ABase.AddFile("a3.txt", "3", 0)
	ABase.AddFile("a4.txt", "4", 0)

	// A:
	// edit a4.txt
	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"a1", "a2", "a3.txt", "a4.txt"})
	A.AddDir("a1", []string{"a11.txt"})
	A.AddFile("a1/a11.txt", "1", 0)
	A.AddDir("a2", []string{"a21.txt", "a22.txt"})
	A.AddFile("a2/a21.txt", "1", 0)
	A.AddFile("a2/a22.txt", "2", 0)
	A.AddFile("a3.txt", "3", 0)
	A.AddFile("a4.txt", "9", 0)

	BBase := NewFakeRoot(t)
	BBase.AddDir(RootPath, []string{"b1", "b2", "b3.txt"})
	BBase.AddDir("b1", []string{"b11.txt", "b12.txt"})
	BBase.AddFile("b1/b11.txt", "1", 0)
	BBase.AddFile("b1/b12.txt", "2", 0)
	BBase.AddDir("b2", []string{"b21.txt", "b22.txt"})
	BBase.AddFile("b2/b21.txt", "1", 0)
	BBase.AddFile("b2/b22.txt", "2", 0)
	BBase.AddFile("b3.txt", "3", 0)
	// B:
	// edit b2/b21.txt
	B := NewFakeRoot(t)
	B.AddDir(RootPath, []string{"b1", "b2", "b3.txt"})
	B.AddDir("b1", []string{"b11.txt", "b12.txt"})
	B.AddFile("b1/b11.txt", "1", 0)
	B.AddFile("b1/b12.txt", "2", 0)
	B.AddDir("b2", []string{"b21.txt", "b22.txt"})
	B.AddFile("b2/b21.txt", "9", 0)
	B.AddFile("b2/b22.txt", "2", 0)
	B.AddFile("b3.txt", "3", 0)

	di, err := Walk4(A, MakeComplete(ABase, t), B, MakeComplete(BBase, t))
	check(err, t)

	nDiffs := 0
	for {
		path, depth, _, tr := di.Get()
		if tr.IsRemovedChild() {
			err := di.Next()
			check(err, t)
			continue
		}
		if !tr.DataIsComplete() {
			err := di.Next()
			check(err, t)
			continue
		}
		if depth == 0 && tr.DataIsComplete() {
			break
		}

		d := di.GetDiff()
		if d.Type != DiffTypeNoChange {
			nDiffs++
		}

		if path == "a4.txt" {
			if d.Type != DiffTypeCreated {
				t.Fatal("a4.txt should appear as created")
			}
		}
		if path == "b2" {
			if d.Type != DiffTypeDeleted {
				t.Fatal("b2 should appear as deleted")
			}
		}
		if path == "b2/b21.txt" {
			if d.Type != DiffTypeDeleted {
				t.Fatal("b2/b21.txt should appear as deleted")
			}
		}

		err = di.Next()
		check(err, t)
	}
	const expectedDiffs = 3
	if nDiffs != expectedDiffs {
		t.Fatalf("expected %d diffs got %d", expectedDiffs, nDiffs)
	}
}

func TestGetTextDiffRowCounts(t *testing.T) {
	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"added.txt", "changed.txt", "same.txt"})
	A.AddFile("added.txt", "a1\na2\n", 0)
	A.AddFile("changed.txt", "same\nnew\n", 0)
	A.AddFile("same.txt", "a\na\n\an", 0)

	B := NewFakeRoot(t)
	B.AddDir(RootPath, []string{"changed.txt", "removed.txt", "same.txt"})
	B.AddFile("changed.txt", "same\nold\n", 0)
	B.AddFile("removed.txt", "r\n", 0)
	B.AddFile("same.txt", "a\na\n\an", 0)

	di, err := Walk2(A, B)
	check(err, t)

	counts := collectTextDiffRowCounts(di, t)
	checkTextDiffRowCounts(counts, "added.txt",
		/*nAdd*/ 2 /*nRemoved*/, 0 /*nChanged*/, 0, t)
	checkTextDiffRowCounts(counts, "removed.txt",
		/*nAdd*/ 0 /*nRemoved*/, 1 /*nChanged*/, 0, t)
	checkTextDiffRowCounts(counts, "changed.txt",
		/*nAdd*/ 0 /*nRemoved*/, 0 /*nChanged*/, 1, t)
	checkTextDiffRowCounts(counts, "same.txt",
		/*nAdd*/ 0 /*nRemoved*/, 0 /*nChanged*/, 0, t)

	diffData, err := CountDiffs(A, B)
	check(err, t)
	expectedDiffData := TotalDiffCounts{
		LinesCreated:  2,
		LinesDeleted:  1,
		LinesModified: 1,
		FilesCreated:  1,
		FilesDeleted:  1,
		FilesModified: 1,
	}
	if diffData != expectedDiffData {
		t.Fatalf("wrong diffData. expected %#v got %#v", expectedDiffData, diffData)
	}
}

func TestGetTextDiffRowCountsForLargeFiles(t *testing.T) {
	largeString := strings.Repeat("x", MaxFileSizeToDiff+1)

	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{"added.txt", "changed.txt", "same.txt"})
	A.AddFile("added.txt", largeString, 0)
	A.AddFile("changed.txt", largeString+"A", 0)
	A.AddFile("same.txt", largeString, 0)

	B := NewFakeRoot(t)
	B.AddDir(RootPath, []string{"changed.txt", "removed.txt", "same.txt"})
	B.AddFile("changed.txt", largeString, 0)
	B.AddFile("removed.txt", largeString, 0)
	B.AddFile("same.txt", largeString, 0)

	di, err := Walk2(A, B)
	check(err, t)

	counts := collectTextDiffRowCounts(di, t)
	// Large files are not diffed; the whole file counts as one changed row
	checkTextDiffRowCounts(counts, "added.txt",
		/*nAdd*/ 1 /*nRemoved*/, 0 /*nChanged*/, 0, t)
	checkTextDiffRowCounts(counts, "removed.txt",
		/*nAdd*/ 0 /*nRemoved*/, 1 /*nChanged*/, 0, t)
	checkTextDiffRowCounts(counts, "changed.txt",
		/*nAdd*/ 0 /*nRemoved*/, 0 /*nChanged*/, 1, t)
	checkTextDiffRowCounts(counts, "same.txt",
		/*nAdd*/ 0 /*nRemoved*/, 0 /*nChanged*/, 0, t)

	diffData, err := CountDiffs(A, B)
	check(err, t)
	expectedDiffData := TotalDiffCounts{
		LinesCreated:  1,
		LinesDeleted:  1,
		LinesModified: 1,
		FilesCreated:  1,
		FilesDeleted:  1,
		FilesModified: 1,
	}
	if diffData != expectedDiffData {
		t.Fatalf("wrong diffData. expected %#v got %#v", expectedDiffData, diffData)
	}
}

func TestCountDiffsForMixedChangeTypes(t *testing.T) {
	// B is the old state, A is the new state.
	B := NewFakeRoot(t)
	B.AddDir(RootPath, []string{
		"dir_becomes_file",
		"file_becomes_dir",
		"file_becomes_executable.sh",
		"grows_too_large.txt",
		"link_changes_target",
		"unchanged.txt",
	})
	B.AddDir("dir_becomes_file", []string{"old_inner.txt"})
	B.AddFile("dir_becomes_file/old_inner.txt", "old inner line\n", 0)
	B.AddFile("file_becomes_dir", "old file line\n", 0)
	B.AddFile("file_becomes_executable.sh", "same content\n", 0)
	B.AddFile("grows_too_large.txt", "small\n", 0)
	B.AddSymlink("link_changes_target", "old-target")
	B.AddFile("unchanged.txt", "unchanged\n", 0)

	A := NewFakeRoot(t)
	A.AddDir(RootPath, []string{
		"dir_becomes_file",
		"file_becomes_dir",
		"file_becomes_executable.sh",
		"grows_too_large.txt",
		"link_changes_target",
		"unchanged.txt",
	})
	A.AddFile("dir_becomes_file", "new file line\n", 0)
	A.AddDir("file_becomes_dir", []string{"new_inner.txt"})
	A.AddFile("file_becomes_dir/new_inner.txt", "new inner line\n", 0)
	A.AddExecutableFile("file_becomes_executable.sh", "same content\n", 0)
	A.AddFile("grows_too_large.txt", strings.Repeat("x", MaxFileSizeToDiff+1), 0)
	A.AddSymlink("link_changes_target", "new-target")
	A.AddFile("unchanged.txt", "unchanged\n", 0)

	diffData, err := CountDiffs(A, B)
	check(err, t)
	expectedDiffData := TotalDiffCounts{
		// "dir_becomes_file" (now a file) and "file_becomes_dir/new_inner.txt",
		// one line each
		LinesCreated: 2,
		FilesCreated: 2,
		// "file_becomes_dir" (was a file) and "dir_becomes_file/old_inner.txt",
		// one line each
		LinesDeleted: 2,
		FilesDeleted: 2,
		// "file_becomes_executable.sh" (same content, so no line changes),
		// "grows_too_large.txt" and "link_changes_target"
		FilesModified: 3,
		// "link_changes_target" counts its target line as changed;
		// "grows_too_large.txt" is beyond MaxFileSizeToDiff, so the whole
		// file counts as one changed line
		LinesModified: 2,
	}
	if diffData != expectedDiffData {
		t.Fatalf("wrong diffData. expected %#v got %#v", expectedDiffData, diffData)
	}
}

// Walks the whole iterator and returns the [nAdd, nRemoved, nChanged] of
// every path with a defined text diff
func collectTextDiffRowCounts(di ParallelIterator, t *testing.T) map[string][3]int64 {
	t.Helper()
	counts := map[string][3]int64{}
	for di.CanGet() {
		path, _, _, _ := di.Get()
		_, nAdd, nRemoved, nChanged, ok, err := di.GetTextDiff()
		check(err, t)
		if ok {
			counts[path] = [3]int64{nAdd, nRemoved, nChanged}
		}
		check(di.Next(), t)
	}
	return counts
}

func checkTextDiffRowCounts(counts map[string][3]int64, path string,
	nAdd, nRemoved, nChanged int64, t *testing.T) {
	t.Helper()
	got, ok := counts[path]
	if !ok {
		t.Fatalf("no text diff computed for %s", path)
	}
	want := [3]int64{nAdd, nRemoved, nChanged}
	if got != want {
		t.Fatalf("wrong row counts for %s: expected %v got %v", path, want, got)
	}
}

func checkDiffBytesContain(substring string, diffBytes []byte, t *testing.T) {
	if !strings.Contains(string(diffBytes), substring) {
		t.Fatalf("%s not found in diff: %s", substring, string(diffBytes))
	}
}
func checkDiffBytesDontContain(substring string, diffBytes []byte, t *testing.T) {
	if strings.Contains(string(diffBytes), substring) {
		t.Fatalf("%s found in diff: %s", substring, string(diffBytes))
	}
}
func checkDiffContains(substring string, diff *bytes.Buffer, t *testing.T) {
	checkDiffBytesContain(substring, diff.Bytes(), t)
}
func checkDiffDoesntContain(substring string, diff *bytes.Buffer, t *testing.T) {
	checkDiffBytesDontContain(substring, diff.Bytes(), t)
}
