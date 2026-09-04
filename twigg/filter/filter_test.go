package filter

import (
	"monorepo/twigg/tree"
	"monorepo/twigg/workdir"
	"strings"
	"testing"
)

func TestTwoEmptyDirs(t *testing.T) {
	wd1 := workdir.NewTest("wd1", t)
	wd2 := workdir.NewTest("wd2", t)
	f := Filter(wd1, []string{tree.RootPath}, wd2)
	checkTreeSize(tree.RootPath, f, 0, t)
}

func TestFilterNonExisting(t *testing.T) {
	wd1 := workdir.NewTest("wd1", t)
	wd1.WriteFile("a.txt", "aaaa")

	wd2 := workdir.NewTest("wd2", t)

	f := Filter(wd1, []string{"non exsisting path"}, wd2)
	checkTreeSize(tree.RootPath, f, 0, t)
}

func TestFilterRootIncludesEverything(t *testing.T) {
	wd1 := workdir.NewTest("wd1", t)
	wd1.WriteFile("a/a.txt", "aa")
	wd1.WriteFile("b/a.txt", "aaa")
	wd1.WriteFile("b/b.txt", "bbbb")
	wd2 := workdir.NewTest("wd2", t)

	f := Filter(wd1, []string{tree.RootPath}, wd2)
	checkTreeSize(tree.RootPath, f, 9, t)
}

func TestFilterOneFile(t *testing.T) {
	wd1 := workdir.NewTest("wd1", t)
	wd1.WriteFile("a.txt", "aa")

	wd2 := workdir.NewTest("wd2", t)

	// The filter should basically be wd1, since its only file is included
	f := Filter(wd1, []string{"a.txt"}, wd2)
	checkTree(tree.RootPath, f, wd1, t)
}

func TestNilFilter(t *testing.T) {
	wd1 := workdir.NewTest("wd1", t)
	wd1.WriteFile("a/b.txt", "bb")
	wd1.WriteFile("b.txt", "bbbbbbb")

	wd2 := workdir.NewTest("wd2", t)
	wd2.WriteFile("b.txt", "bb")

	// Filter nil means it'll be a copy of wd2
	f := Filter(wd1, nil, wd2)
	checkTree("b.txt", f, wd2, t)
	checkTree(tree.RootPath, f, wd2, t)
}

func TestSimpleTwoFiles(t *testing.T) {
	wd1 := workdir.NewTest("wd1", t)
	wd1.WriteFile("a.txt", "aa")
	wd1.WriteFile("b.txt", "bb")

	wd2 := workdir.NewTest("wd2", t)
	wd2.WriteFile("a.txt", "AAA")
	wd2.WriteFile("b.txt", "BBB")

	// filtered will be like wd1 for a.txt, and like wd2 for b.txt
	f := Filter(wd1, []string{"a.txt"}, wd2)
	checkTree("a.txt", f, wd1, t)
	checkTree("b.txt", f, wd2, t)

	// The root folder contains "a.txt=aa" and "b.txt=BBB" -> size = 5
	checkTreeSize(tree.RootPath, f, 5, t)
}

func TestManyFolders(t *testing.T) {
	wd1 := workdir.NewTest("wd1", t)
	wd1.WriteFile("a/a.txt", "a")                           // size 1
	wd1.WriteFile("a/b.txt", "bb")                          // size 2
	wd1.WriteFile("x/a.txt", "aaa")                         // size 3
	wd1.WriteFile("x/b.txt", "bbbbb")                       // size 5
	wd1.WriteFile("s/s/s/s/s.txt", strings.Repeat("a", 55)) // size 55

	wd2 := workdir.NewTest("wd2", t)
	wd2.WriteFile("a/a.txt", strings.Repeat("a", 8))  // size 8
	wd2.WriteFile("a/b.txt", strings.Repeat("b", 13)) // size 13
	wd2.WriteFile("c/a.txt", strings.Repeat("a", 21)) // size 21
	wd2.WriteFile("c/b.txt", strings.Repeat("b", 34)) // size 34

	// Filter with root path.
	// We expect everything from w1 and everything extra that wd2 has.
	f := Filter(wd1, []string{tree.RootPath}, wd2)
	checkTreeSize(tree.RootPath, f, 1+2+3+5+55+21+34, t)

	// Filter only a/
	f = Filter(wd1, []string{"a/"}, wd2)
	checkTreeSize(tree.RootPath, f, 1+2+21+34, t)

	// Filter only a/a.txt
	// We expect:
	// a/a.txt from w1
	// Everything else from w2
	f = Filter(wd1, []string{"a/a.txt"}, wd2)
	checkTreeSize(tree.RootPath, f, 1+13+21+34, t)

	// Filter only a/a.txt and s/s/s
	// We expect:
	// a/a.txt from w1 and s/s/s/s/s.txt
	// Everything else from w2
	f = Filter(wd1, []string{"a/a.txt", "s/s/s"}, wd2)
	checkTreeSize(tree.RootPath, f, 1+55+13+21+34, t)

}

// Checks that the tree of the given path is the same in the provided roots
func checkTree(treePath string, r1, r2 tree.Root, t *testing.T) {
	w1, err := tree.Walk(r1)
	if err != nil {
		t.Fatal(err)
	}
	w2, err := tree.Walk(r2)
	if err != nil {
		t.Fatal(err)
	}
	foundT1 := false
	var t1 tree.Tree
	var t1Path string
	for w1.CanGet() {
		t1Path, _, _, t1 = w1.Get()
		if t1Path == treePath && t1.DataIsComplete() {
			foundT1 = true
			break
		}
		err = w1.Next()
		if err != nil {
			t.Fatal(err)
		}
	}
	if !foundT1 {
		t.Fatalf("could not find tree in root 1")
	}
	foundT2 := false
	var t2 tree.Tree
	var t2Path string
	for w2.CanGet() {
		t2Path, _, _, t2 = w2.Get()
		if t2Path == treePath && t2.DataIsComplete() {
			foundT2 = true
			break
		}
		err = w2.Next()
		if err != nil {
			t.Fatal(err)
		}
	}
	if !foundT2 {
		t.Fatalf("could not find tree in root 2")
	}
	if !tree.IsEqual(t1, t2) {
		t.Fatal("trees are different")
	}
}

func checkTreeSize(treePath string, r1 tree.Root, expectedSize int64, t *testing.T) {
	w1, err := tree.Walk(r1)
	if err != nil {
		t.Fatal(err)
	}
	foundT1 := false
	var t1 tree.Tree
	var t1Path string
	for w1.CanGet() {
		t1Path, _, _, t1 = w1.Get()
		if t1Path == treePath && t1.DataIsComplete() {
			foundT1 = true
			break
		}
		err = w1.Next()
		if err != nil {
			t.Fatal(err)
		}
	}
	if !foundT1 {
		t.Fatalf("could not find tree in root 1")
	}
	if t1.Data().Size != int64(expectedSize) {
		t.Fatalf("expected size %d got %d", expectedSize, t1.Data().Size)
	}
}
