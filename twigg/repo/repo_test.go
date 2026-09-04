package repo

import (
	"bytes"
	"errors"
	"monorepo/twigg/cli/clidb"
	"monorepo/twigg/tree"
	"monorepo/twigg/workdir"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"
)

func setup(t *testing.T) (TreeVersion, workdir.TestWorkdir, Write, Repo) {
	wd := workdir.NewTest("test", t)
	db, closeDb, err := clidb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDb)
	l, ul, _, err := db.BeginWrite()
	t.Cleanup(ul)
	if err != nil {
		t.Fatal(err)
	}
	w := db.Bind(l)

	r := New("owner", 1)
	v0, _, err := r.Init(w)
	if err != nil {
		t.Fatal(err)
	}
	if v0 != RootTreeVersion {
		t.Fatal("first version must be zero")
	}
	return v0, wd, w, r
}

func TestSaveSingleFile(t *testing.T) {
	v0, wd, l, r := setup(t)

	wd.WriteFile("a.txt", "a")
	v1, v1Hash, err := r.Save(wd, v0, l)
	if err != nil {
		t.Fatal(err)
	}
	root0 := r.Root(v1, l)

	tr, err := root0.Tree(tree.RootPath)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Data().Depth != 0 {
		t.Fatal("root should have depth 0")
	}
	if !tr.Data().HasChildrenData {
		t.Fatal("expected to have children data")
	}
	checkTreeHash(tr, v1Hash, t)
	tree.CheckTree(tr,
		/*name=*/ tree.RootPath,
		/*depth=*/ 0,
		/*isRemovedChild=*/ false,
		/*isDir=*/ true,
		/*isExecutableFile=*/ false,
		/*childNames=*/ []string{"a.txt"},
		/*isFullyComputed=*/ true,
		/*size=*/ 1, t)

	aTree, err := root0.Tree("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if aTree.Data().Depth != 1 {
		t.Fatal("a.txt should have depth 1")
	}
	tree.CheckTree(aTree,
		/*name=*/ "a.txt",
		/*depth=*/ 1,
		/*isRemovedChild=*/ false,
		/*isDir=*/ false,
		/*isExecutableFile=*/ false,
		/*childNames=*/ []string{},
		/*isFullyComputed=*/ true,
		/*size=*/ 1, t)
	if !reflect.DeepEqual(aTree.Data(), tr.Data().ChildrenData[0]) {
		t.Fatal("wrong children data")
	}
}

func TestSaveExecutableSingleFile(t *testing.T) {
	v0, wd, l, r := setup(t)

	wd.WriteExecutableFile("a.bin", "a")
	v1, _, err := r.Save(wd, v0, l)
	if err != nil {
		t.Fatal(err)
	}
	root0 := r.Root(v1, l)

	aTree, err := root0.Tree("a.bin")
	if err != nil {
		t.Fatal(err)
	}
	if aTree.Data().Depth != 1 {
		t.Fatal("a.bin should have depth 1")
	}
	tree.CheckTree(aTree,
		/*name=*/ "a.bin",
		/*depth=*/ 1,
		/*isRemovedChild=*/ false,
		/*isDir=*/ false,
		/*isExecutableFile=*/ true,
		/*childNames=*/ []string{},
		/*isFullyComputed=*/ true,
		/*size=*/ 1, t)

}

func TestSaveFileAndDirWithSamePrefix(t *testing.T) {
	v0, wd, l, r := setup(t)

	wd.WriteFile("a.txt", "a")
	wd.WriteFile("a/a.txt", "aa")
	v1, v1RootHash, err := r.Save(wd, v0, l)
	if err != nil {
		t.Fatal(err)
	}
	if v1RootHash != wd.RootDirHash() {
		t.Fatal("wrong root Hash")
	}
	iter, err := tree.Walk(r.Root(v1, l))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, tr := iter.Get()
	checkTreeHash(tr, v1RootHash, t)
	tree.CheckIterator(iter, tree.RootPath,
		/*depth*/ 0,
		/*isRemovedChild=*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a", "a.txt"},
		/*isFullyKnown*/ true,
		/*size*/ 3,
		t)
	err = iter.Next()
	if err != nil {
		t.Fatal(err)
	}
	tree.CheckIterator(iter, "a",
		/*depth*/ 1,
		/*isRemovedChild=*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a.txt"},
		/*isFullyKnown*/ true,
		/*size*/ 2,
		t)
	err = iter.Next()
	if err != nil {
		t.Fatal(err)
	}
	tree.CheckIterator(iter, "a/a.txt",
		/*depth*/ 2,
		/*isRemovedChild=*/ false,
		/*isDir*/ false,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 2,
		t)

}

func TestSaveV1(t *testing.T) {
	v0, wd, l, r := setup(t)

	// v1:
	// a.txt = "hello"
	// b/b.txt = "hi"
	wd.WriteFile("a.txt", "hello")
	wd.WriteFile("b/b.txt", "hi")
	v1, v1RootHash, err := r.Save(wd, v0, l)
	if err != nil {
		t.Fatal(err)
	}
	if v1 != 1 {
		t.Fatal("second version should have version 1")
	}
	if v1RootHash != wd.RootDirHash() {
		t.Fatal("wrong root Hash")
	}

	rt := r.Root(1, l)
	// Check all trees by reading them from the repo
	tr, err := rt.Tree(tree.RootPath)
	if err != nil {
		t.Fatal(err)
	}
	checkTreeHash(tr, v1RootHash, t)
	tree.CheckTree(tr,
		/*name=*/ tree.RootPath,
		/*depth=*/ 0,
		/*isRemovedChild=*/ false,
		/*isDir=*/ true,
		/*isExecutableFile=*/ false,
		/*childNames=*/ []string{"a.txt", "b"},
		/*isFullyComputed=*/ true,
		/*size=*/ 7, t)

	tr, err = rt.Tree("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	tree.CheckTree(tr,
		/*name=*/ "a.txt",
		/*depth=*/ 1,
		/*isRemovedChild=*/ false,
		/*isDir=*/ false,
		/*isExecutableFile=*/ false,
		/*childNames=*/ []string{},
		/*isFullyComputed=*/ true,
		/*size=*/ 5, t)

	tr, err = rt.Tree("b")
	if err != nil {
		t.Fatal(err)
	}
	tree.CheckTree(tr,
		/*name=*/ "b",
		/*depth=*/ 1,
		/*isRemovedChild=*/ false,
		/*isDir=*/ true,
		/*isExecutableFile=*/ false,
		/*childNames=*/ []string{"b.txt"},
		/*isFullyComputed=*/ true,
		/*size=*/ 2, t)

	tr, err = rt.Tree("b/b.txt")
	if err != nil {
		t.Fatal(err)
	}
	tree.CheckTree(tr,
		/*name=*/ "b.txt",
		/*depth=*/ 2,
		/*isRemovedChild=*/ false,
		/*isDir=*/ false,
		/*isExecutableFile=*/ false,
		/*childNames=*/ []string{},
		/*isFullyComputed=*/ true,
		/*size=*/ 2, t)
}

func TestSaveV2(t *testing.T) {

	testOnce := func(useV1AsParentOfV2 bool) {
		v0, wd, l, r := setup(t)

		// v1:
		// a.txt = "hello"
		// b/b.txt = "hi"
		wd.WriteFile("a.txt", "hello")
		wd.WriteFile("b/b.txt", "hi")
		v1, v1RootHash, err := r.Save(wd, v0, l)
		if err != nil {
			t.Fatal(err)
		}
		if v1RootHash != wd.RootDirHash() {
			t.Fatal("wrong root Hash")
		}

		// v2:
		// b/b.txt = "hi"
		// b/c.txt = "ccc"
		wd.Delete("a.txt")
		wd.WriteFile("b/c.txt", "ccc")

		// The parent used isnt really all that relevant.
		// It just otimizes storage if we pass a parent version that is
		// very similar to thw new one, as less trees will be stored.
		var v2 TreeVersion
		var v2RootHash [32]byte
		if useV1AsParentOfV2 {
			v2, v2RootHash, err = r.Save(wd, v1, l)
			if err != nil {
				t.Fatal(err)
			}
		} else {
			v2, v2RootHash, err = r.Save(wd, v0, l)
			if err != nil {
				t.Fatal(err)
			}
		}
		if v2 != 2 {
			t.Fatal("v2 should be 2")
		}
		if v2RootHash != wd.RootDirHash() {
			t.Fatal("wrong root Hash")
		}

		rt := r.Root(v2, l)
		tr, err := rt.Tree("")
		if err != nil {
			t.Fatal(err)
		}
		checkTreeHash(tr, v2RootHash, t)
		tree.CheckTree(tr,
			/*name=*/ tree.RootPath,
			/*depth=*/ 0,
			/*isRemovedChild=*/ false,
			/*isDir=*/ true,
			/*isExecutableFile=*/ false,
			/*childNames=*/ []string{"b"},
			/*isFullyComputed=*/ true,
			/*size=*/ 5, t)
		tr, err = rt.Tree("b")
		if err != nil {
			t.Fatal(err)
		}
		tree.CheckTree(tr,
			/*name=*/ "b",
			/*depth=*/ 1,
			/*isRemovedChild=*/ false,
			/*isDir=*/ true,
			/*isExecutableFile=*/ false,
			/*childNames=*/ []string{"b.txt", "c.txt"},
			/*isFullyComputed=*/ true,
			/*size=*/ 5, t)
	}

	testOnce(true)
	testOnce(false)
}

func TestManyChanges(t *testing.T) {
	v0, wd, l, r := setup(t)

	// Repo version i will contain a.txt="a"*i
	versionToRootHash := make(map[uint64][32]byte)
	var rootHash [32]byte
	lastRepoVersion := v0
	for i := 0; i < 20; i++ {
		newRepoVersion := lastRepoVersion + 1
		wd.WriteFile("a.txt", strings.Repeat("a", int(newRepoVersion)))
		lastRepoVersion, rootHash, _ = r.Save(wd, lastRepoVersion, l)
		if rootHash != wd.RootDirHash() {
			t.Fatal("wrong root dir hash")
		}
		versionToRootHash[lastRepoVersion] = rootHash
	}

	for repoVersion := uint64(1); repoVersion < 20; repoVersion++ {
		rt := r.Root(repoVersion, l)
		tr, err := rt.Tree(tree.RootPath)
		if err != nil {
			t.Fatal(err)
		}
		checkTreeHash(tr, versionToRootHash[repoVersion], t)
		tree.CheckTree(tr,
			/*name=*/ tree.RootPath,
			/*depth=*/ 0,
			/*isRemovedChild=*/ false,
			/*isDir=*/ true,
			/*isExecutableFile=*/ false,
			/*childNames=*/ []string{"a.txt"},
			/*isFullyComputed=*/ true,
			/*size=*/ int64(repoVersion), t)
	}
}

func TestReadFile(t *testing.T) {
	v0, wd, l, r := setup(t)

	wd.WriteFile("1_keep.txt", "1_keep")
	wd.WriteFile("2_edit.txt", "2_edit")
	r.Save(wd, v0, l)

	wd.WriteFile("2_edit.txt", "2_EDIT")
	wd.WriteFile("3_create.txt", "3_create")
	r.Save(wd, 1, l)

	checkFile := func(repoVersion TreeVersion, filename string, content string) {
		rt := r.Root(repoVersion, l)
		tr, err := rt.Tree(filename)
		if err != nil {
			t.Fatal(err)
		}
		md := tr.Data()
		if md.IsDir {
			t.Fatal("tree is directory")
		}
		if md.Size != int64(len(content)) {
			t.Fatalf("expected size %d got %d",
				len(content), md.Size)
		}
		wt, err := tr.GetFile()
		if err != nil {
			t.Fatal(err)
		}
		buff := bytes.NewBuffer(nil)
		wt.WriteTo(buff)
		s := buff.String()
		if s != content {
			t.Fatalf("expected %s, got %s", content, s)
		}
	}

	checkFile(1, "1_keep.txt", "1_keep")
	checkFile(1, "2_edit.txt", "2_edit")

	checkFile(2, "1_keep.txt", "1_keep")
	checkFile(2, "2_edit.txt", "2_EDIT")
	checkFile(2, "3_create.txt", "3_create")
}

func TestErrNoChange(t *testing.T) {
	v0, wd, l, r := setup(t)

	// v1: a.txt
	wd.WriteFile("a.txt", "hello")
	v1, v1RootHash, err := r.Save(wd, v0, l)
	if err != nil {
		t.Fatal(err)
	}
	if v1 != 1 {
		t.Fatal("second version should have version 1")
	}

	// Trying to save unchanged repo returns an err and the last version
	v, vRootHash, err := r.Save(wd, v1, l)
	if !errors.Is(err, ErrNoChange) {
		t.Fatalf("Expected ErrNoChange, got %v", err)
	}
	if v != v1 {
		t.Fatal("should show last saved version")
	}
	if v1RootHash != vRootHash {
		t.Fatal("hashes should match")
	}
}

func TestDiff(t *testing.T) {
	v0, wd, l, r := setup(t)

	wd.WriteFile("1_keep.txt", "keep")
	wd.WriteFile("2_edit.txt", "edit")
	wd.WriteFile("3_delete.txt", "delete")
	wd.WriteFile("a/b/c/d/3_keep.txt", "keep")
	wd.WriteFile("a/b/c/d/1_edit.txt", "edit")
	wd.WriteFile("a/b/c/d/2_delete.txt", "delete")
	v1, _, err := r.Save(wd, v0, l)
	if err != nil {
		t.Fatal(err)
	}

	// wd.WriteFile("1_keep.txt", "keep")
	wd.WriteFile("2_edit.txt", "EDIT")
	wd.Delete("3_delete.txt")
	wd.WriteFile("4_create.txt", "create")
	// wd.WriteFile("a/b/c/d/3_keep.txt", "keep")
	wd.WriteFile("a/b/c/d/0_create.txt", "create")
	wd.WriteFile("a/b/c/d/1_edit.txt", "EDIT")
	wd.Delete("a/b/c/d/2_delete.txt")
	v2, _, err := r.Save(wd, v1, l)
	if err != nil {
		t.Fatal(err)
	}

	di, err := tree.Walk2(r.Root(v2, l), r.Root(v1, l))
	if err != nil {
		t.Fatal(err)
	}

	skipDirAndNoChange := func() {
		if !di.CanGet() {
			return
		}
		_, _, _, tr := di.Get()
		d := di.GetDiff()
		for tr.Data().IsDir || d.Type == tree.DiffTypeNoChange {
			err = di.Next()
			if err != nil {
				t.Fatal(err)
			}
			if !di.CanGet() {
				return
			}
			_, _, _, tr = di.Get()
			d = di.GetDiff()
		}
	}

	skipDirAndNoChange()
	checkFileDiff(di, "2_edit.txt", tree.DiffTypeAnyModified, "EDIT", t)

	err = di.Next()
	if err != nil {
		t.Fatal(err)
	}
	skipDirAndNoChange()

	checkFileDiff(di, "3_delete.txt", tree.DiffTypeDeleted, "delete", t)
	err = di.Next()
	if err != nil {
		t.Fatal(err)
	}
	skipDirAndNoChange()

	checkFileDiff(di, "4_create.txt", tree.DiffTypeCreated, "create", t)
	err = di.Next()
	if err != nil {
		t.Fatal(err)
	}
	skipDirAndNoChange()

	checkFileDiff(di, "a/b/c/d/0_create.txt", tree.DiffTypeCreated, "create", t)
	err = di.Next()
	if err != nil {
		t.Fatal(err)
	}
	skipDirAndNoChange()

	checkFileDiff(di, "a/b/c/d/1_edit.txt", tree.DiffTypeAnyModified, "EDIT", t)
	err = di.Next()
	if err != nil {
		t.Fatal(err)
	}
	skipDirAndNoChange()

	checkFileDiff(di, "a/b/c/d/2_delete.txt", tree.DiffTypeDeleted, "delete", t)
	err = di.Next()
	if err != nil {
		t.Fatal(err)
	}
	skipDirAndNoChange()
	if di.CanGet() {
		t.Fatal("should be done")
	}
}

func TestDiffOtherParents(t *testing.T) {
	v0, wd, l, r := setup(t)

	// v1: creates a.txt=aaa
	// c2 modifies a.txt
	wd.WriteFile("a.txt", "aaa")
	v1, _, err := r.Save(wd, v0, l)
	if err != nil {
		t.Fatal(err)
	}

	// v2: modify a.txt=aaaAAAA
	wd.WriteFile("a.txt", "aaaAAAA")
	v2, _, err := r.Save(wd, v1, l)
	if err != nil {
		t.Fatal(err)
	}

	// v3: child of v1
	// create b.txt=bbb
	wd.WriteFile("a.txt", "aaa")
	wd.WriteFile("b.txt", "bbb")
	v3, _, err := r.Save(wd, v1, l)
	if err != nil {
		t.Fatal(err)
	}

	// Tree:
	//
	// v2(a.txt=aaaAAAA)
	// |                    v3(a.txt=aaa, b.txt=bbb)
	// v1(a.txt=aaa)--------/
	// |
	// v0

	// Diff v3 and v2
	di, err := tree.Walk2(r.Root(v3, l), r.Root(v2, l))
	if err != nil {
		t.Fatal(err)
	}

	tree.CheckDiff(di,
		tree.DiffTypeAnyModified,
		tree.RootPath,
		/*depth*/ 0,
		/*isRemovedChild=*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"a.txt", "b.txt"},
		/*isFullyComputed*/ true,
		/*size*/ 6,
		t)
	di.Next()
	tree.CheckDiff(di,
		tree.DiffTypeAnyModified,
		"a.txt",
		/*depth*/ 1,
		/*isRemovedChild=*/ false,
		/*isDir*/ false,
		/*isExecutableFile=*/ false,
		/*children*/ []string{},
		/*isFullyComputed*/ true,
		/*size*/ 3,
		t)
	di.Next()
	tree.CheckDiff(di,
		tree.DiffTypeCreated,
		"b.txt",
		/*depth*/ 1,
		/*isRemovedChild=*/ false,
		/*isDir*/ false,
		/*isExecutableFile=*/ false,
		/*children*/ []string{},
		/*isFullyComputed*/ true,
		/*size*/ 3,
		t)
	di.Next()
	tree.CheckDiff(di,
		tree.DiffTypeAnyModified,
		tree.RootPath,
		/*depth*/ 0,
		/*isRemovedChild=*/ false,
		/*isDir*/ true,
		/*isExecutableFile=*/ false,
		/*children*/ []string{"a.txt", "b.txt"},
		/*isFullyComputed*/ true,
		/*size*/ 6,
		t)
	di.Next()
	if di.CanGet() {
		t.Fatal("should be done")
	}

}

func TestTextDiff(t *testing.T) {
	v0, wd, l, r := setup(t)

	// v1
	wd.WriteFile("a.txt", "000\n111\n222\n333\n444\n")
	v1, _, _ := r.Save(wd, v0, l)
	// v2
	wd.WriteFile("a.txt", "000\n111\naaa\n333\n444\n")
	v2, _, _ := r.Save(wd, v1, l)

	diffBytes, err := tree.GetPathUnifiedDiff("a.txt", r.Root(v2, l), r.Root(v1, l))
	if err != nil {
		t.Fatal(err)
	}
	diffString := string(diffBytes)

	if !strings.Contains(diffString, " 000") {
		t.Fatalf("wrong diff string: %s", diffString)
	}
	if !strings.Contains(diffString, " 111") {
		t.Fatalf("wrong diff string: %s", diffString)
	}
	if !strings.Contains(diffString, "-222") {
		t.Fatalf("wrong diff string: %s", diffString)
	}
	if !strings.Contains(diffString, "+aaa") {
		t.Fatalf("wrong diff string: %s", diffString)
	}
	if !strings.Contains(diffString, " 333") {
		t.Fatalf("wrong diff string: %s", diffString)
	}
	if !strings.Contains(diffString, " 444") {
		t.Fatalf("wrong diff string: %s", diffString)
	}
}

func TestLoad(t *testing.T) {
	v0, wd, l, r := setup(t)

	// v1
	// a/a/a1.txt = "111"
	// a/a/a2.txt = "222"
	// b.txt = "bbb"
	// c.txt = "ccc"
	// d.txt = "ddd"
	wd.WriteFile("a/a/a1.txt", "111")
	wd.WriteFile("a/a/a2.txt", "222")
	wd.WriteFile("b.txt", "bbb")
	wd.WriteFile("c.txt", "ccc")
	wd.WriteFile("d.txt", "ddd")
	v1, _, _ := r.Save(wd, v0, l)

	// Working directory:
	// a/a/a1.txt (deleted)
	// a/a/a2.txt = "222"
	// b.txt = "BBB"
	// c.txt = ""
	// d.txt = "ddd"
	wd.Delete("a/a/a1.txt")
	wd.WriteFile("b.txt", "BBB")
	wd.WriteFile("c.txt", "")

	// Load back v1 and read back
	err := r.Load(v1, wd, l)
	if err != nil {
		t.Fatal(err)
	}
	if wd.ReadFile("a/a/a1.txt") != "111" {
		t.Fatal("wrong contents of a1")
	}
	if wd.ReadFile("a/a/a2.txt") != "222" {
		t.Fatal("wrong contents of a2")
	}
	if wd.ReadFile("b.txt") != "bbb" {
		t.Fatal("wrong contents of b")
	}
	if wd.ReadFile("c.txt") != "ccc" {
		t.Fatal("wrong contents of c")
	}
	if wd.ReadFile("d.txt") != "ddd" {
		t.Fatal("wrong contents of d")
	}
}

func TestLoadFileWhereWorkdirHasDir(t *testing.T) {
	v0, wd, l, r := setup(t)

	// v1
	// a.txt = "aaa"
	// swap = "vvv"
	// z.txt = "zzz"
	wd.WriteFile("a.txt", "aaa")
	wd.WriteFile("swap", "vvv")
	wd.WriteFile("z.txt", "zzz")
	v1, _, err := r.Save(wd, v0, l)
	if err != nil {
		t.Fatal(err)
	}

	// Working directory:
	// a.txt = "aaa"
	// swap/child.txt = "ccc"
	// z.txt = "zzz"
	err = wd.Delete("swap")
	if err != nil {
		t.Fatal(err)
	}
	wd.WriteFile("swap/child.txt", "ccc")

	// Load back v1: the swap dir must become the v1 file again
	err = r.Load(v1, wd, l)
	if err != nil {
		t.Fatal(err)
	}
	if wd.HasFolder("swap") {
		t.Fatal("swap should no longer be a directory")
	}
	if wd.ReadFile("swap") != "vvv" {
		t.Fatal("wrong contents of swap")
	}
	if wd.ReadFile("a.txt") != "aaa" {
		t.Fatal("wrong contents of a")
	}
	if wd.ReadFile("z.txt") != "zzz" {
		t.Fatal("wrong contents of z")
	}
}

func TestLoadDirWhereWorkdirHasFile(t *testing.T) {
	v0, wd, l, r := setup(t)

	// v1
	// a.txt = "aaa"
	// swap/child.txt = "ccc"
	// z.txt = "zzz"
	wd.WriteFile("a.txt", "aaa")
	wd.WriteFile("swap/child.txt", "ccc")
	wd.WriteFile("z.txt", "zzz")
	v1, _, err := r.Save(wd, v0, l)
	if err != nil {
		t.Fatal(err)
	}

	// Working directory:
	// a.txt = "aaa"
	// swap = "vvv"
	// z.txt = "zzz"
	err = wd.Delete("swap")
	if err != nil {
		t.Fatal(err)
	}
	wd.WriteFile("swap", "vvv")

	// Load back v1: the swap file must become the v1 dir again
	err = r.Load(v1, wd, l)
	if err != nil {
		t.Fatal(err)
	}
	if !wd.HasFolder("swap") {
		t.Fatal("swap should be a directory again")
	}
	if wd.ReadFile("swap/child.txt") != "ccc" {
		t.Fatal("wrong contents of swap/child.txt")
	}
	if wd.ReadFile("a.txt") != "aaa" {
		t.Fatal("wrong contents of a")
	}
	if wd.ReadFile("z.txt") != "zzz" {
		t.Fatal("wrong contents of z")
	}
}

func TestLoadModtimeUpdatedCorrectly(t *testing.T) {
	v0, wd, l, r := setup(t)

	// v1
	// a.txt = "hello"
	wd.WriteFile("a.txt", "hello, world!")
	v1, _, _ := r.Save(wd, v0, l)
	tr, err := wd.Tree("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	md := tr.Data()
	v1ModTimeUnixMillis := md.LastModifiedUnixMillis

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// v2
	// a.txt = "bye"
	wd.WriteFile("a.txt", "bye")
	r.Save(wd, v1, l)
	tr, err = wd.Tree("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	md = tr.Data()
	v2ModTimeUnixMicros := md.LastModifiedUnixMillis

	if v2ModTimeUnixMicros < v1ModTimeUnixMillis {
		t.Fatal("v1 modtime should be larger")
	}

	// load v1 and check the modtime
	r.Load(v1, wd, l)
	tr, err = wd.Tree("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	md = tr.Data()
	if md.LastModifiedUnixMillis != v1ModTimeUnixMillis {
		t.Fatal("modtime was not updated to the v1 value")
	}
}

func TestSimpleRebase(t *testing.T) {
	v0, wd, l, r := setup(t)

	// v1: a.txt=hello
	wd.WriteFile("a.txt", "hello")
	v1, _, _ := r.Save(wd, v0, l)

	// v2: b.txt=bye
	wd.Delete("a.txt")
	wd.WriteFile("b.txt", "bye")
	// Note that the parent (v1) for saving doesnt really matter.
	// we just benefit from passing a parent similar to the new one as that
	// reduces the number of files written.
	v2, _, _ := r.Save(wd, v1, l)

	// Rebase v2 into v1 using v0 as parent:
	//
	// v1(+ a.txt) v2(+b.txt)
	// |           |
	// v0----------/
	//
	// Since v2 adds b.txt, we just add b.txt to v1.
	// Expect:
	// a.txt = "hello"
	// b.txt = "bye"
	v, vHash, c, err := r.Rebase(v2, "v2", v1, "v1", v0, l)
	if err != nil {
		t.Fatal(err)
	}
	if c {
		t.Fatal("should not conflict")
	}

	rootTree, err := r.Root(v, l).Tree(tree.RootPath)
	if err != nil {
		t.Fatal(err)
	}
	checkTreeHash(rootTree, vHash, t)

	err = r.Load(v, wd, l)
	if err != nil {
		t.Fatal(err)
	}

	if wd.ReadFile("a.txt") != "hello" {
		t.Fatal("wrong contents of a")
	}
	if wd.ReadFile("b.txt") != "bye" {
		t.Fatal("wrong contents of b")
	}

	canRebase, err := r.CanRebaseWithoutConflict(v2, v1, v0, l)
	if err != nil {
		t.Fatal(err)
	}
	if !canRebase {
		t.Fatal("got unable to rebase without conflicts")
	}
}

func TestSimpleRebaseOfDeletion(t *testing.T) {
	v0, wd, l, r := setup(t)

	// v1: a.txt=aa
	wd.WriteFile("a.txt", "aa")
	v1, _, _ := r.Save(wd, v0, l)
	// v2: b.txt=bb
	wd.Delete("a.txt")
	wd.WriteFile("b.txt", "bb")
	v2, _, _ := r.Save(wd, v0, l)
	// v3: a.txt=aa, c.txt="cc"
	wd.WriteFile("a.txt", "aa")
	wd.WriteFile("c.txt", "cc")
	v3, _, _ := r.Save(wd, v0, l)

	// Rebase v2 into v3 using v1 as parent:
	//
	// v2(-a.txt, +b.txt)
	// |
	// v1(a.txt) v3(a.txt, c.txt)
	// |           |
	// v0----------/
	//
	// v2 deletes a.txt and creates b.txt.
	// So we expect to end up with b.txt and c.txt
	v, vHash, c, err := r.Rebase(v2, "v2", v3, "v3", v1, l)
	if err != nil {
		t.Fatal(err)
	}
	if c {
		t.Fatal("should not conflict")
	}
	rootTree, err := r.Root(v, l).Tree(tree.RootPath)
	if err != nil {
		t.Fatal(err)
	}
	checkTreeHash(rootTree, vHash, t)
	err = r.Load(v, wd, l)
	if err != nil {
		t.Fatal(err)
	}
	if wd.HasFile("a.txt") {
		t.Fatal("a.txt should be deleted")
	}
	if wd.ReadFile("b.txt") != "bb" {
		t.Fatal("wrong contents of b")
	}
	if wd.ReadFile("c.txt") != "cc" {
		t.Fatal("wrong contents of c")
	}

	canRebase, err := r.CanRebaseWithoutConflict(v2, v3, v1, l)
	if err != nil {
		t.Fatal(err)
	}
	if !canRebase {
		t.Fatal("got unable to rebase without conflicts")
	}
}

func TestSimpleConflictRebase(t *testing.T) {
	v0, wd, l, r := setup(t)

	// v1
	// a.txt = "aaa"
	wd.WriteFile("a.txt", "hello")
	v1, _, _ := r.Save(wd, v0, l)

	// v2
	// a.txt = "AAA"
	wd.WriteFile("a.txt", "AAA")
	v2, _, _ := r.Save(wd, v0, l)

	// Rebase v2 into v1 using v0 as parent
	// Expect a conflict
	v, vHash, c, err := r.Rebase(v2, "v2", v1, "v1", v0, l)
	if err != nil {
		t.Fatal(err)
	}
	if !c {
		t.Fatal("should get conflict")
	}
	rootTree, err := r.Root(v, l).Tree(tree.RootPath)
	if err != nil {
		t.Fatal(err)
	}
	checkTreeHash(rootTree, vHash, t)

	err = r.Load(v, wd, l)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(wd.ReadFile("a.txt"), "<<") {
		t.Fatal("a.txt should contain conflict markers")
	}

	canRebase, err := r.CanRebaseWithoutConflict(v2, v1, v0, l)
	if err != nil {
		t.Fatal(err)
	}
	if canRebase {
		t.Fatal("got ok to rebase without conflicts")
	}
}

func TestEmptyDelta(t *testing.T) {
	v0, _, l, r := setup(t)
	d, err := r.GetDelta(v0, v0, l)
	if err != nil {
		t.Fatal(err)
	}
	checkDelta(d, tree.RootPath, nil, t)
	check(d.Pop(), t)
	if d.CanGet() {
		t.Fatal("should be done")
	}
}

func TestSingleFileDelta(t *testing.T) {
	v0, wd, l, r := setup(t)
	// v1 adds a.txt
	wd.WriteFile("a.txt", "hello")
	v1, _, _ := r.Save(wd, v0, l)

	d, err := r.GetDelta(v1, v0, l)
	check(err, t)

	checkDelta(d, "a.txt", nil, t)

	check(d.Pop(), t)
	checkDelta(d, tree.RootPath, []string{"a.txt"}, t)

	check(d.Pop(), t)
	if d.CanGet() {
		t.Fatal("should be done")
	}
}

func TestSingleFileChangeDelta(t *testing.T) {
	v0, wd, l, r := setup(t)
	// v1 adds a.txt
	wd.WriteFile("a.txt", "hello")
	v1, _, _ := r.Save(wd, v0, l)
	// v2 modifies a.txt and creates b.txt
	wd.WriteFile("a.txt", "HELLO")
	wd.WriteFile("b.txt", "bbb")
	v2, _, _ := r.Save(wd, v0, l)

	d, err := r.GetDelta(v2, v1, l)
	check(err, t)

	checkDelta(d, "a.txt", nil, t)
	check(d.Pop(), t)
	checkDelta(d, "b.txt", nil, t)
	check(d.Pop(), t)
	checkDelta(d, tree.RootPath, []string{"a.txt", "b.txt"}, t)

	check(d.Pop(), t)
	if d.CanGet() {
		t.Fatal("should be done")
	}
}

func TestLargerDelta(t *testing.T) {
	v0, wd, l, r := setup(t)
	// v1
	// a/a/a1.txt = "111"
	// a/a/a2.txt = "222"
	// b.txt = "bbb"
	// c.txt = "ccc"
	// d/d.txt = "ddd"
	wd.WriteFile("a/a/a1.txt", "111")
	wd.WriteFile("a/a/a2.txt", "222")
	wd.WriteFile("b.txt", "bbb")
	wd.WriteFile("c.txt", "ccc")
	wd.WriteFile("d/d.txt", "ddd")
	v1, _, _ := r.Save(wd, v0, l)

	// v2
	// a/a/a1.txt (deleted)
	// a/a/a2.txt = "222" (unchanged)
	// b.txt = "BBB" (modified)
	// c.txt = "" (modified)
	// d/d.txt = "ddd" (unchanged)
	wd.Delete("a/a/a1.txt")
	wd.WriteFile("b.txt", "BBB")
	wd.WriteFile("c.txt", "")
	v2, _, _ := r.Save(wd, v1, l)

	d, err := r.GetDelta(v2, v1, l)
	check(err, t)

	checkDelta(d, "a/a/a2.txt", nil, t)
	check(d.Pop(), t)
	checkDelta(d, "a/a", []string{"a2.txt"}, t)
	check(d.Pop(), t)
	checkDelta(d, "a", []string{"a"}, t)
	check(d.Pop(), t)
	checkDelta(d, "b.txt", nil, t)
	check(d.Pop(), t)
	checkDelta(d, "c.txt", nil, t)
	check(d.Pop(), t)
	checkDelta(d, "d", []string{"d.txt"}, t) // note d/d.txt is not present
	check(d.Pop(), t)
	checkDelta(d, tree.RootPath,
		[]string{"a", "b.txt", "c.txt", "d"}, t)
}

func TestSaveEmptyDelta(t *testing.T) {
	v0, _, l, r := setup(t)
	d, err := r.GetDelta(v0, v0, l)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = r.SaveDelta(d, v0, l)
	if !errors.Is(err, ErrNoChange) {
		t.Fatal("expected no change err")
	}
}

func TestSaveSingleFileDelta(t *testing.T) {
	v0, wd, l, r := setup(t)
	// v1 adds a.txt
	wd.WriteFile("a.txt", "hello")
	v1, v1Hash, _ := r.Save(wd, v0, l)

	d, err := r.GetDelta(v1, v0, l)
	check(err, t)

	// Naturally, v2 will be the same as v1.
	// But this shows that saving the delta works
	v2, v2Hash, err := r.SaveDelta(d, v0, l)
	check(err, t)
	if v2 != 2 {
		t.Fatal("expected v2")
	}
	if v1Hash != v2Hash {
		t.Fatal("expected same hash")
	}

	// Walk the tree to verify it's what we expect
	iter, err := tree.Walk(r.Root(v2, l))
	check(err, t)

	tree.CheckIterator(iter,
		tree.RootPath,
		/*depth*/ 0,
		/*isRemovedChild=*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a.txt"},
		/*isFullyKnown*/ true,
		/*size*/ 5,
		t)
	check(iter.Next(), t)
	tree.CheckIterator(iter,
		"a.txt",
		/*depth*/ 1,
		/*isRemovedChild=*/ false,
		/*isDir*/ false,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 5,
		t)
	check(iter.Next(), t)
	tree.CheckIterator(iter,
		tree.RootPath,
		/*depth*/ 0,
		/*isRemovedChild=*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a.txt"},
		/*isFullyKnown*/ true,
		/*size*/ 5,
		t)
	check(iter.Next(), t)
	if d.CanGet() {
		t.Fatal("should be done")
	}
}

func TestSaveLargerDelta(t *testing.T) {
	v0, wd, l, r := setup(t)
	// v1
	// a/a/a1.txt = "111"
	// a/a/a2.txt = "222"
	// b.txt = "bbb"
	// c.txt = "ccc"
	// d/d.txt = "ddd"
	wd.WriteFile("a/a/a1.txt", "111")
	wd.WriteFile("a/a/a2.txt", "222")
	wd.WriteFile("b.txt", "bbb")
	wd.WriteFile("c.txt", "ccc")
	wd.WriteFile("d/d.txt", "ddd")
	v1, _, _ := r.Save(wd, v0, l)

	// v2
	// a/a/a2.txt = "222"
	// b.txt = "BBB"
	// c.txt = ""
	// d/d.txt = "ddd"
	wd.Delete("a/a/a1.txt")
	wd.WriteFile("b.txt", "BBB")
	wd.WriteFile("c.txt", "")
	v2, _, _ := r.Save(wd, v1, l)

	d, err := r.GetDelta(v2, v1, l)
	check(err, t)

	v3, v3Hash, err := r.SaveDelta(d, v1, l)
	check(err, t)

	v3RootTree, err := r.Root(v3, l).Tree(tree.RootPath)
	check(err, t)
	checkTreeHash(v3RootTree, v3Hash, t)

	// Walk the tree to verify it's what we expect
	iter, err := tree.Walk(r.Root(v3, l))
	check(err, t)

	tree.CheckIterator(iter,
		tree.RootPath,
		/*depth*/ 0,
		/*isRemovedChild=*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a", "b.txt", "c.txt", "d"},
		/*isFullyKnown*/ true,
		/*size*/ 9,
		t)
	check(iter.Next(), t)
	tree.CheckIterator(iter,
		"a",
		/*depth*/ 1,
		/*isRemovedChild=*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a"},
		/*isFullyKnown*/ true,
		/*size*/ 3,
		t)
	iter.SkipChildrenOnNext()
	check(iter.Next(), t)
	tree.CheckIterator(iter,
		"b.txt",
		/*depth*/ 1,
		/*isRemovedChild=*/ false,
		/*isDir*/ false,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 3,
		t)
	check(iter.Next(), t)
	tree.CheckIterator(iter,
		"c.txt",
		/*depth*/ 1,
		/*isRemovedChild=*/ false,
		/*isDir*/ false,
		/*children*/ []string{},
		/*isFullyKnown*/ true,
		/*size*/ 0,
		t)
	check(iter.Next(), t)
	tree.CheckIterator(iter,
		"d",
		/*depth*/ 1,
		/*isRemovedChild=*/ false,
		/*isDir*/ true,
		/*children*/ []string{"d.txt"},
		/*isFullyKnown*/ true,
		/*size*/ 3,
		t)
	iter.SkipChildrenOnNext()
	check(iter.Next(), t)
	tree.CheckIterator(iter,
		tree.RootPath,
		/*depth*/ 0,
		/*isRemovedChild=*/ false,
		/*isDir*/ true,
		/*children*/ []string{"a", "b.txt", "c.txt", "d"},
		/*isFullyKnown*/ true,
		/*size*/ 9,
		t)
	check(iter.Next(), t)
	if d.CanGet() {
		t.Fatal("should be done")
	}
}

func TestSearchFileInChangedDirs(t *testing.T) {
	// v0: empty
	v0, wd, l, r := setup(t)

	// v1:
	// a/a/FILE.txt = "a111"
	// a/b/FILE.txt = "b111"
	// b.txt = "bbb"
	wd.WriteFile("a/a/FILE.txt", "a111")
	wd.WriteFile("a/b/FILE.txt", "b111")
	wd.WriteFile("b.txt", "bbb")
	v1, _, _ := r.Save(wd, v0, l)

	// v2:
	//
	// a/b/FILE.txt = "b111"
	// b.txt = "bbb"
	wd.Delete("a/a/FILE.txt")
	v2, _, _ := r.Save(wd, v1, l)

	// v3:
	//
	// a/b/FILE.txt/a.txt = "aaa"
	// a/b/FILE.txt/FILE.txt = "FILE.txt is now a folder"
	// b.txt = "bbb"
	wd.Delete("a/b/FILE.txt")
	wd.WriteFile("a/b/FILE.txt/a.txt", "aaa")
	wd.WriteFile("a/b/FILE.txt/FILE.txt", "FILE.txt is now a folder")
	v3, _, _ := r.Save(wd, v2, l)

	// v4:
	//
	// FILE.txt/a.txt
	// b.txt = "bbb"
	wd.Delete("a/b/FILE.txt/a.txt")
	wd.Delete("a/b/FILE.txt/FILE.txt")
	wd.WriteFile("FILE.txt/a.txt", "aaa")
	v4, _, _ := r.Save(wd, v3, l)

	// v0-v0, v1-v1, etc: no change
	iter, err := r.SearchFileInChangedDirs(v0, v0, l, "FILE.txt")
	check(err, t)
	checkFileInChangedDirsIter(iter, nil, t)
	iter, err = r.SearchFileInChangedDirs(v1, v1, l, "FILE.txt")
	check(err, t)
	checkFileInChangedDirsIter(iter, nil, t)

	// v1 - v0: Creates `"a/a/FILE.txt"` and `"a/b/FILE.txt"`
	iter, err = r.SearchFileInChangedDirs(v1, v0, l, "FILE.txt")
	check(err, t)
	checkFileInChangedDirsIter(iter, []fileInChangedDirsIterEntry{
		{
			IsCreated:   true,
			Path:        "a/a/FILE.txt",
			PathDepth:   3,
			FileContent: "a111",
		},
		{
			IsCreated:   true,
			Path:        "a/b/FILE.txt",
			PathDepth:   3,
			FileContent: "b111",
		},
	}, t)

	// v2 - v1: Deletes `"a/a/FILE.txt"`
	iter, err = r.SearchFileInChangedDirs(v2, v1, l, "FILE.txt")
	check(err, t)
	checkFileInChangedDirsIter(iter, []fileInChangedDirsIterEntry{
		{
			IsDeleted:   true,
			Path:        "a/a/FILE.txt",
			PathDepth:   3,
			FileContent: "a111",
		},
	}, t)

	// v2 - v0: Creates `"a/b/FILE.txt"`
	iter, err = r.SearchFileInChangedDirs(v2, v0, l, "FILE.txt")
	check(err, t)
	checkFileInChangedDirsIter(iter, []fileInChangedDirsIterEntry{
		{
			IsCreated:   true,
			Path:        "a/b/FILE.txt",
			PathDepth:   3,
			FileContent: "b111",
		},
	}, t)

	// v3 - v2: `"a/b/FILE.txt"` becomes a directory
	iter, err = r.SearchFileInChangedDirs(v3, v2, l, "FILE.txt")
	check(err, t)
	checkFileInChangedDirsIter(iter, []fileInChangedDirsIterEntry{
		{
			IsDeleted:   true,
			Path:        "a/b/FILE.txt",
			PathDepth:   3,
			FileContent: "b111",
		},
		{
			IsCreated:   true,
			Path:        "a/b/FILE.txt/FILE.txt",
			PathDepth:   4,
			FileContent: "FILE.txt is now a folder",
		},
	}, t)

	// v4 - v0: no FILE.txt created
	iter, err = r.SearchFileInChangedDirs(v4, v0, l, "FILE.txt")
	check(err, t)
	checkFileInChangedDirsIter(iter, nil, t)

	// v4 - v3: a/b/FILE.txt/FILE.txt deleted
	iter, err = r.SearchFileInChangedDirs(v4, v3, l, "FILE.txt")
	check(err, t)
	checkFileInChangedDirsIter(iter, []fileInChangedDirsIterEntry{
		{
			IsDeleted:   true,
			Path:        "a/b/FILE.txt/FILE.txt",
			PathDepth:   4,
			FileContent: "FILE.txt is now a folder",
		},
	}, t)

}

func check(err error, t testing.TB) {
	if err != nil {
		t.Fatal("check failed:", err)
	}
}

func checkFileDiff(
	fileDiff tree.ParallelIterator,
	expectedPath string,
	expectedDiffType tree.DiffType,
	expectedFileContent string, t *testing.T) {
	gotPath, gotDepth, _, tr := fileDiff.Get()
	if gotPath != expectedPath {
		t.Fatalf("expected path %s got %s", expectedPath, gotPath)
	}
	if tr.Data().BaseName != path.Base(expectedPath) {
		t.Fatalf("expected file %s got %s", path.Base(expectedPath), tr.Data().BaseName)
	}
	if gotPath == tree.RootPath {
		if gotDepth != 0 {
			t.Fatalf("expected depth %d got %d", 0, gotDepth)
		}
	} else {
		expectedDepth := uint32(strings.Count(gotPath, "/") + 1)
		if gotDepth != expectedDepth {
			t.Fatalf("expected depth %d got %d", expectedDepth, gotDepth)
		}
	}

	diff := fileDiff.GetDiff()
	if diff.Type != expectedDiffType {
		t.Fatalf("expected diff type %d got %d", expectedDiffType, diff.Type)
	}
	wt, err := tr.GetFile()
	if err != nil {
		t.Fatal(err)
	}
	buff := bytes.NewBuffer(nil)
	wt.WriteTo(buff)
	s := buff.String()
	if s != expectedFileContent {
		t.Fatalf("expected content %s got %s", expectedFileContent, s)
	}
}

func checkDelta(d DeltaIter, expectedPath string, children []string, t *testing.T) {
	if !d.CanGet() {
		t.Fatalf("delta iter is empty")
	}
	gotPath, gotDepth, tr := d.Get()
	if gotPath != expectedPath {
		t.Fatalf("expected path %s got %s", expectedPath, gotPath)
	}
	if tr.Data().BaseName != path.Base(expectedPath) {
		t.Fatalf("expected name %s got %s", path.Base(expectedPath), tr.Data().BaseName)
	}
	if gotPath == tree.RootPath {
		if gotDepth != 0 {
			t.Fatalf("expected depth %d got %d", 0, gotDepth)
		}
	} else {
		expectedDepth := uint32(strings.Count(gotPath, "/") + 1)
		if gotDepth != expectedDepth {
			t.Fatalf("expected depth %d got %d", expectedDepth, gotDepth)
		}
	}

	if len(tr.Data().ChildrenBaseNames) != 0 || len(children) != 0 {
		if !reflect.DeepEqual(tr.Data().ChildrenBaseNames, children) {
			t.Fatalf("expected children %v\n got %v\n",
				children, tr.Data().ChildrenBaseNames)
		}
	}
}

func checkTreeHash(tr tree.Tree, hash [32]byte, t *testing.T) {
	if !tr.DataIsComplete() {
		t.Fatalf("expected tree data to be complete")
	}
	if tr.Data().ContentHash != hash {
		t.Fatalf("expected tree hash %s, got %s", tr.Data().ContentHash, hash)
	}
}

type fileInChangedDirsIterEntry struct {
	IsCreated   bool
	IsModified  bool
	IsDeleted   bool
	Path        string
	PathDepth   uint32
	FileContent string
}

func checkFileInChangedDirsIter(
	it FileInChangedDirsIter, expectedEntries []fileInChangedDirsIterEntry,
	t *testing.T) {
	t.Helper()
	if expectedEntries == nil {
		expectedEntries = []fileInChangedDirsIterEntry{}
	}

	getTreeString := func(tr tree.Tree) string {
		f, err := tr.GetFile()
		if err != nil {
			t.Fatalf("failed to GetFile FileInChangedDirsIter: %s", err)
		}
		buff := bytes.NewBuffer(nil)
		_, err = f.WriteTo(buff)
		if err != nil {
			t.Fatalf("failed to read file in FileInChangedDirsIter: %s", err)
		}
		return buff.String()
	}

	gotEntries := []fileInChangedDirsIterEntry{}
	for it.CanGet() {
		isCreated, isModified, isDeleted, path, pathDepth, tr, aTr, bTr := it.GetFile()
		if isModified {
			if getTreeString(aTr) == getTreeString(bTr) {
				t.Fatalf("got isModified but tree strings are the same: %s", getTreeString(aTr))
			}
		}
		if isCreated || isModified {
			// the A tree must match the first tree returned
			trStr := getTreeString(tr)
			aStr := getTreeString(aTr)
			if trStr != aStr {
				t.Fatalf("tr=%s, aTr=%s", trStr, aStr)
			}
		}
		if isDeleted {
			// the B tree must match the tree returned
			trStr := getTreeString(tr)
			bStr := getTreeString(bTr)
			if trStr != bStr {
				t.Fatalf("tr=%s, bTr=%s", trStr, bStr)
			}
		}
		gotEntries = append(gotEntries, fileInChangedDirsIterEntry{
			IsCreated:   isCreated,
			IsModified:  isModified,
			IsDeleted:   isDeleted,
			Path:        path,
			PathDepth:   pathDepth,
			FileContent: getTreeString(tr),
		})
		err := it.Next()
		if err != nil {
			t.Fatalf("failed to iterate FileInChangedDirsIter: %s", err)
		}
	}
	if !reflect.DeepEqual(expectedEntries, gotEntries) {
		t.Fatalf("FileInChangedDirsIter expected entries:\n%#v\ngot:\n%#v\n", expectedEntries, gotEntries)
	}
}
