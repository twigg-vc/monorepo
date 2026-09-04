package workdir

import (
	"bytes"
	"errors"
	diff3 "monorepo/twigg/diff/epiclabs-io"
	"monorepo/twigg/tree"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNotFound(t *testing.T) {
	wd := NewTest("test", t)
	_, err := wd.Tree("non existing")
	if !errors.Is(err, tree.ErrTreeNotFound) {
		t.Fatal("expected err not found")
	}
}

func TestEmptyRootTree(t *testing.T) {
	wd := NewTest("test", t)
	tree, err := wd.Tree("")
	if err != nil {
		t.Fatal(err)
	}
	n := len(tree.Data().ChildrenBaseNames)
	if n != 0 {
		t.Fatal("should have zero children")
	}
}

func TestRoot(t *testing.T) {
	wd := NewTest("test", t)
	wd.WriteFile("a.txt", "aaa")
	wd.WriteFile("sub/a.txt", "a")
	wd.WriteExecutableFile("sub/b.bin", "b")
	wd.WriteFile("a/b/c/d/x.txt", "xxx")

	tr, err := wd.Tree(tree.RootPath)
	if err != nil {
		t.Fatal(err)
	}
	tree.CheckTree(tr,
		/*name=*/ tree.RootPath,
		/*depth=*/ 0,
		/*isRemovedChild=*/ false,
		/*isDir=*/ true,
		/*isExecutableFile=*/ false,
		/*childNames=*/ []string{"a", "a.txt", "sub"},
		/*isFullyComputed=*/ false,
		/*size=*/ 0, t)

	tr, err = wd.Tree("a.txt")
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
		/*size=*/ 3, t)

	tr, err = wd.Tree("sub")
	if err != nil {
		t.Fatal(err)
	}
	tree.CheckTree(tr,
		/*name=*/ "sub",
		/*depth=*/ 1,
		/*isRemovedChild=*/ false,
		/*isDir=*/ true,
		/*isExecutableFile=*/ false,
		/*childNames=*/ []string{"a.txt", "b.bin"},
		/*isFullyComputed=*/ false,
		/*size=*/ 0, t)

	tr, err = wd.Tree("sub/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	tree.CheckTree(tr,
		/*name=*/ "a.txt",
		/*depth=*/ 2,
		/*isRemovedChild=*/ false,
		/*isDir=*/ false,
		/*isExecutableFile=*/ false,
		/*childNames=*/ []string{},
		/*isFullyComputed=*/ true,
		/*size=*/ 1, t)

	tr, err = wd.Tree("sub/b.bin")
	if err != nil {
		t.Fatal(err)
	}
	tree.CheckTree(tr,
		/*name=*/ "b.bin",
		/*depth=*/ 2,
		/*isRemovedChild=*/ false,
		/*isDir=*/ false,
		/*isExecutableFile=*/ true,
		/*childNames=*/ []string{},
		/*isFullyComputed=*/ true,
		/*size=*/ 1, t)

	tr, err = wd.Tree("a/b/c")
	if err != nil {
		t.Fatal(err)
	}
	tree.CheckTree(tr,
		/*name=*/ "c",
		/*depth=*/ 3,
		/*isRemovedChild=*/ false,
		/*isDir=*/ true,
		/*isExecutableFile=*/ false,
		/*childNames=*/ []string{"d"},
		/*isFullyComputed=*/ false,
		/*size=*/ 0, t)
}

func TestRootWalk(t *testing.T) {
	wd := NewTest("test", t)
	wd.WriteFile("a.txt", "aaa")
	wd.WriteFile("b.txt", "bbbb")
	wd.WriteFile("sub/c.txt", "cc")
	wd.WriteFile("sub/sub/d.txt", "d")
	nowMillis := time.Now().UnixMilli()
	wd.SetModTime("sub/sub/d.txt", nowMillis+500)

	dfs, err := tree.Walk(wd)
	if err != nil {
		t.Fatal(err)
	}
	var tr tree.Tree
	var depth uint32
	for {
		_, depth, _, tr = dfs.Get()
		if depth != tr.Data().Depth {
			t.Fatalf("inconsistent depth")
		}
		if depth == 0 && tr.DataIsComplete() {
			break
		}
		err = dfs.Next()
		if err != nil {
			t.Fatal(err)
		}
	}
	got := tr.Data()
	if got.Size != 10 {
		t.Fatal("wrong size")
	}
	if got.LastModifiedUnixMillis != nowMillis+500 {
		t.Fatal("wrong last update")
	}
	if got.Depth != 0 {
		t.Fatal("root has depth=0")
	}
}

func TestDepth(t *testing.T) {
	wd := NewTest("test", t)
	wd.WriteFile("a.txt", "aaa")
	wd.WriteFile("sub/c.txt", "cc")
	wd.WriteFile("sub/sub/d.txt", "d")

	checkDepth := func(path string, expectedDepth uint32) {
		tr, err := wd.Tree(path)
		if err != nil {
			t.Fatal(err)
		}
		if tr.Data().Depth != expectedDepth {
			t.Fatalf("expected depth %d got %d",
				expectedDepth, tr.Data().Depth)
		}
	}

	checkDepth(".", 0)
	checkDepth("a.txt", 1)
	checkDepth("sub", 1)
	checkDepth("sub/c.txt", 2)
	checkDepth("sub/sub", 2)
	checkDepth("sub/sub/d.txt", 3)
}

func TestSimpleIgnores(t *testing.T) {
	wd := NewTest("test", t)

	wd.Ignore("*.txt")
	wd.WriteFile("a.txt", "aaa")
	if wd.HasFile("a.txt") {
		t.Fatal("a.txt is ignored")
	}

	wd.Ignore("ignored_subfolder/")
	wd.WriteFile("ignored_subfolder/b.txt", "bbb")
	if wd.HasFile("ignored_subfolder/b.txt") {
		t.Fatal("b.txt is ignored")
	}

	wd.Ignore("**/*.bin")
	wd.WriteFile("c.bin", "ccc")
	if wd.HasFile("c.bin") {
		t.Fatal("c.bin is ignored")
	}
	wd.WriteFile("sub/c.bin", "ccc")
	if wd.HasFile("sub/c.bin") {
		t.Fatal("sub/c.bin is ignored")
	}
}

func TestSimpleIgnoresParsing(t *testing.T) {
	wd := NewTest("test", t)

	wd.WriteFile(IgnoreFileName, "*.txt\n**/*.bin")

	wd.WriteFile("a.txt", "aaa")
	if wd.HasFile("a.txt") {
		t.Fatal("a.txt is ignored")
	}

	wd.WriteFile("sub/b.bin", "bbb")
	if wd.HasFile("sub/b.bin") {
		t.Fatal("b.bin is ignored")
	}
}

func TestIgnore_file_folder_extension(t *testing.T) {
	wd := NewTest("test", t)

	wd.WriteFile("a.txt", "hello, world!")

	// Ignore file:
	// b.txt <- ignore any b.txt file
	// subfolder/ <- ignore any folder named "subfolder"
	// **/*.bin <- ignore any file that ends with .bin
	wd.WriteFile(IgnoreFileName, "b.txt\nsubfolder/\n**/*.bin")
	wd.WriteFile("b.txt", "this file is ignored")
	wd.WriteFile("subfolder/c.txt", "this file is in an ignored subfolder")
	wd.WriteFile("sub/sub/subfolder/c.txt", "this file is in an ignored subfolder")
	wd.WriteFile("sub/sub/sub/binary.bin", "all .bin files are ignored")

	wd.WriteFile("subfolder2/"+IgnoreFileName, "modules/")
	wd.WriteFile("subfolder2/modules/e.txt", "ignored")

	wd.Ignore("d.txt")
	wd.WriteFile("d.txt", "this was ignored at the workspace (not ignorefile)")

	if !wd.HasFile(IgnoreFileName) {
		t.Fatal("expected ignore file")
	}

	if !wd.HasFile("a.txt") {
		t.Fatal("expected a.txt")
	}
	if wd.HasFile("subfolder/c.txt") {
		t.Fatal("subfolder/c.txt is ignored")
	}
	if wd.HasFile("sub/sub/subfolder/c.txt") {
		t.Fatal("subfolder/ is ignored")
	}
	if wd.HasFile("subfolder2/modules/e.txt") {
		t.Fatal("subfolder2/modules/ is ignored")
	}
	if wd.HasFile("sub/sub/sub/binary.bin") {
		t.Fatal("**.bin are ignored")
	}

	content := wd.ReadFile("a.txt")
	if content != "hello, world!" {
		t.Fatalf("wrong a.txt content: %s", content)
	}
}

func TestIgnoreAnchor(t *testing.T) {
	wd := NewTest("test", t)

	// Leading slashes anchor the ignore to the directory of the ignore file:
	// "/dir/" ignores the ./dir directory, but not ./a/b/c/.../dir
	wd.WriteFile(IgnoreFileName, "/dir/\n/a.txt")
	wd.WriteFile("dir/ignored.txt", "ignored")
	wd.WriteFile("sub/dir/notIgnored.txt", "not ignored")
	wd.WriteFile("a.txt", "ignored")
	wd.WriteFile("sub/a.txt", "not ignored")

	if wd.HasFile("dir/ignored.txt") {
		t.Fatal("dir/ignored.txt should be ignored")
	}
	if !wd.HasFile("sub/dir/notIgnored.txt") {
		t.Fatal("sub directories named `dir` should not be ignored")
	}
	if wd.HasFile("a.txt") {
		t.Fatal("a.txt should be ignored")
	}
	if !wd.HasFile("sub/a.txt") {
		t.Fatal("sub/a.txt should not be ignored")
	}
}

func TestNewTestInSubfolder(t *testing.T) {
	wd := NewTest("subfolder/test", t)
	wd.WriteFile("b.txt", "I'm in ./subfolder/test")

	// cd ..
	wd.GoUp()

	// Now files are written to ./subfolder
	wd.WriteFile("a.txt", "I'm in ./subfolder")

	if !wd.HasFile("a.txt") {
		t.Fatal("expected a.txt")
	}
	if !wd.HasFile("test/b.txt") {
		t.Fatal("test/b.txt")
	}

	if wd.GoUp() == nil {
		t.Fatal("should not be able to go above test workdir")
	}
}
func TestGoDown(t *testing.T) {
	wd := NewTest("subfolder/test", t)

	// cd ..
	wd.GoUp()

	wd.GoDown("/test")

	// Now files are written to ./subfolder/test
	wd.WriteFile("a.txt", "I'm in ./subfolder/test")
	if !wd.HasFile("a.txt") {
		t.Fatal("expected a.txt")
	}

	wd.GoUp()
	if !wd.HasFile("test/a.txt") {
		t.Fatal("expected test/a.txt")
	}
}

func TestClearIgnores(t *testing.T) {
	wd := NewTest("test", t)
	wd.WriteFile("a.txt", "aloha")
	wd.Ignore("*.txt")
	if wd.HasFile("a.txt") {
		t.Fatal("a.txt should be ignored")
	}
	wd.ClearIgnores()
	if !wd.HasFile("a.txt") {
		t.Fatal("a.txt should be visible after ClearIgnores")
	}
}
func TestIgnoreNegation(t *testing.T) {
	wd := NewTest("test", t)
	wd.WriteFile(IgnoreFileName, "*.txt\n!a.txt")
	wd.WriteFile("a.txt", "not ignored")
	wd.WriteFile("b.txt", "ignored")
	if !wd.HasFile("a.txt") {
		t.Fatal("a.txt should be un-ignored by negation")
	}
	if wd.HasFile("b.txt") {
		t.Fatal("b.txt should be ignored")
	}
}

func TestIgnoreTrailingSlashDir(t *testing.T) {
	wd := NewTest("test", t)
	// A trailing-slash pattern (e.g. build/) should match the directory node
	// itself and exclude it from its parent's children, not just filter its
	// contents individually.
	wd.WriteFile(IgnoreFileName, "build/")
	wd.WriteFile("build/a.txt", "ignored")
	wd.WriteFile("src/main.go", "visible")

	tr, err := wd.Tree(tree.RootPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, child := range tr.Data().ChildrenBaseNames {
		if child == "build" {
			t.Fatal("build/ pattern should exclude the build dir from the tree")
		}
	}
	if !wd.HasFile("src/main.go") {
		t.Fatal("src/main.go should not be ignored")
	}
}

func TestNestedIgnores(t *testing.T) {
	wd := NewTest("test", t)
	wd.WriteFile("subfolder/.gitignore", "subsubfolder/*")
	wd.WriteFile("subfolder/subsubfolder/a.txt", "hi")
	wd.WriteFile("subfolder/subsubfolder/b.txt", "hello")
	wd.WriteFile("subfolder/c.txt", "aloha")
	wd.WriteFile("a.txt", "aloha")

	if !wd.HasFile("a.txt") {
		t.Fatal("expected a.txt")
	}
	if !wd.HasFile("subfolder/.gitignore") {
		t.Fatal("expected 1.txt")
	}

	if !wd.HasFile("subfolder/c.txt") {
		t.Fatal("expected c.txt")
	}
}

func TestDelete(t *testing.T) {
	wd := NewTest("test", t)
	wd.WriteFile("a.txt", "aloha")
	wd.WriteFile("sub/b.txt", "aloha")
	if !wd.HasFile("a.txt") {
		t.Fatal("setup failed")
	}
	if !wd.HasFile("sub/b.txt") {
		t.Fatal("setup failed")
	}
	wd.Delete("a.txt")
	wd.Delete("sub")

	tr, err := wd.Tree("")
	if err != nil {
		t.Fatal(err)
	}
	n := len(tr.Data().ChildrenBaseNames)
	if n != 0 {
		t.Fatal("should have zero children")
	}
}

func TestHas(t *testing.T) {
	wd := NewTest("test", t)
	wd.WriteFile("sub/a.txt", "aloha")
	wd.WriteFile("sub/sub/a.txt", "aloha")
	if !wd.HasFolder(".") {
		t.Fatal("should have .")
	}
	if !wd.HasFolder("sub") {
		t.Fatal("should have sub")
	}
	if !wd.HasFolder("sub/sub") {
		t.Fatal("should have sub")
	}
	if wd.HasFolder("random") {
		t.Fatal("should not have random")
	}
}

func TestSetModTime(t *testing.T) {
	wd := NewTest("test", t)
	wd.WriteFile("a.txt", "aloha")

	wd.SetModTime("a.txt", 1_000_000)
	tr, err := wd.Tree("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	data := tr.Data()
	if data.IsDir {
		t.Fatal("a.txt is not a dir")
	}
	if data.LastModifiedUnixMillis != 1_000_000 {
		t.Fatal("unexpected modtime")
	}
}
func TestCreateFolder(t *testing.T) {
	wd := NewTest("test", t)

	err := wd.CreateFolder("myfolder")
	if err != nil {
		t.Fatal(err)
	}

	wd.GoDown("myfolder")

	// already exist
	wd.WriteFile("sub/a.txt", "aloha")

	err = wd.CreateFolder("sub")
	if err == nil {
		t.Fatal("should err, already exist")
	}
	if !wd.HasFolder("sub") {
		t.Fatal("should have sub")
	}

}

func TestConflicts(t *testing.T) {
	wd := NewTest("test", t)
	has, err := wd.FileHasConflict("non-existing")
	if has || err != nil {
		t.Fatal("expected no conflicts for non existing file")
	}

	wd.WriteFile("a.txt", "abc")
	has, err = wd.FileHasConflict("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("expected no conflicts")
	}

	wd.WriteFile("a.txt", "\n"+diff3.ConflictStart)
	has, err = wd.FileHasConflict("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("expected conflicts")
	}
}

func TestCacheSpeedsUpTree(t *testing.T) {

	backup := enableFileHashCaching
	t.Cleanup(func() {
		enableFileHashCaching = backup
	})

	// Use a large minSizeToCacheFile to ensure cache speeds things up
	minSizeToCacheFile := 10 * 1024 * 1024 // 10MB
	wd := NewTestWithCustomMinFileToCache("test", int64(minSizeToCacheFile), t)
	wd.WriteFile("a.txt", strings.Repeat("a", minSizeToCacheFile+1))
	wd.WriteFile("b.txt", strings.Repeat("b", minSizeToCacheFile+1))

	// To reduce flakyness, read the tree many times
	const timesToMeasure = 2

	// Get tree with caching enabled
	enableFileHashCaching = true
	start := time.Now()
	for i := 0; i < timesToMeasure; i++ {
		_, err := wd.Tree("a.txt")
		if err != nil {
			t.Fatal(err)
		}
	}
	timeToGetWithCachingEnabled := time.Since(start).Microseconds()

	// Get tree with caching disabled
	enableFileHashCaching = false
	start = time.Now()
	for i := 0; i < timesToMeasure; i++ {
		_, err := wd.Tree("b.txt")
		if err != nil {
			t.Fatal(err)
		}
	}
	timeToGetWithCachingDisabled := time.Since(start).Microseconds()

	// Compare the times
	if timeToGetWithCachingEnabled > timeToGetWithCachingDisabled {
		t.Fatalf(
			"reading tree took %d, reading cached took %d",
			timeToGetWithCachingDisabled, timeToGetWithCachingEnabled)
	}

}

func TestSymlink(t *testing.T) {
	wd := NewTest("test", t)
	wd.WriteFile("a.txt", "aloha")
	wd.WriteSymlink("sl", "a.txt")

	tr, err := wd.Tree("sl")
	if err != nil {
		t.Fatal(err)
	}
	data := tr.Data()
	if !data.IsSymlink {
		t.Fatal("should be symlink")
	}
	if data.SymlinkTarget != "a.txt" {
		t.Fatalf("wrong target: %s", data.SymlinkTarget)
	}
	if data.IsDir {
		t.Fatal("symlink is not a dir")
	}
	if !data.IsText {
		t.Fatal("symlink has IsText=true bc it has a string representation")
	}
	if data.Size != 0 {
		t.Fatalf("symlink size should be zero, got %d", data.Size)
	}

	// GetFile must return the display string, not follow the symlink to a.txt
	wt, err := tr.GetFile()
	if err != nil {
		t.Fatal(err)
	}
	buf := bytes.NewBuffer(nil)
	if _, err = wt.WriteTo(buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != tree.SymlinkString("a.txt") {
		t.Fatalf("GetFile returned %q, want symlink display string", buf.String())
	}
}
func TestSymlinksAreNotResolved(t *testing.T) {
	// Symlink targets are stored as forward-slashed, but they're not resolved
	wd := NewTest("test", t)
	wd.WriteFile("target.txt", "hello")
	wd.WriteFile("sub/target.txt", "sub hello")
	wd.WriteFile("sub/sub/target.txt", "sub sub hello")

	err := wd.WriteSymlink("points_to_target", "target.txt")
	if err != nil {
		t.Fatal(err)
	}
	err = wd.WriteSymlink("sub/points_to_dotdot_sub_target", "../sub/target.txt")
	if err != nil {
		t.Fatal(err)
	}
	err = wd.WriteSymlink("sub/points_to_sub_target", "sub/target.txt")
	if err != nil {
		t.Fatal(err)
	}

	checkSymlinkTarget := func(treePath string, expectedTarget string) {
		tr, err := wd.Tree(treePath)
		if err != nil {
			t.Fatal(err)
		}
		if !tr.Data().IsSymlink {
			t.Fatalf("%s is not symling", treePath)
		}
		if tr.Data().SymlinkTarget != expectedTarget {
			t.Fatalf("want target %q, got %q", expectedTarget, tr.Data().SymlinkTarget)
		}
	}

	checkSymlinkTarget("points_to_target", "target.txt")
	checkSymlinkTarget("sub/points_to_dotdot_sub_target", "../sub/target.txt")
	checkSymlinkTarget("sub/points_to_sub_target", "sub/target.txt")

}
func TestSymlinkCreatesParentDir(t *testing.T) {
	// WriteSymlink must create the symlink's parent directory if it doesn't exist.
	wd := NewTest("test", t)
	wd.WriteFile("target.txt", "hello")
	if err := wd.WriteSymlink("newdir/sl", "../target.txt"); err != nil {
		t.Fatalf("WriteSymlink in non-existent dir failed: %v", err)
	}
	if !wd.HasFolder("newdir") {
		t.Fatal("symlink parent dir not created")
	}
	if !wd.HasFile("newdir/sl") {
		t.Fatal("symlink not created")
	}
}

func TestSymlinkEval(t *testing.T) {
	// Create some symlinks with WriteSymlink and eval them
	wd := NewTest("test", t)
	wd.WriteFile("a.txt", "a")
	wd.WriteFile("b.txt", "b")
	wd.WriteFile("sub/a.txt", "aa")
	wd.WriteFile("sub/b.txt", "bb")

	wd.WriteSymlink("points_to_a", "a.txt")
	wd.WriteSymlink("sub/points_to_sub_a", "a.txt")
	wd.WriteSymlink("sub/sub/points_to_a", "../../a.txt")

	checkResolves := func(path, expected string) {
		resolved, err := filepath.EvalSymlinks(filepath.Join(wd.Path(), path))
		if err != nil {
			t.Fatalf("symlink %q doesn't resolve: %v", path, err)
		}
		if resolved != filepath.Clean(filepath.Join(wd.Path(), expected)) {
			t.Fatalf("unexpected %q resolution: %s", path, resolved)
		}
	}
	checkResolves("points_to_a", "a.txt")
	checkResolves("sub/points_to_sub_a", "sub/a.txt")
	checkResolves("sub/sub/points_to_a", "a.txt")
}

func TestSymlinkWithoutCache(t *testing.T) {
	// With minSizeToCacheFile=0 every file enters the cache block.
	wd := NewTestWithCustomMinFileToCache("test", 0, t)
	wd.WriteFile("a.txt", "hello")
	wd.WriteSymlink("sl", "a.txt")
	tr, err := wd.Tree("sl")
	if err != nil {
		t.Fatalf("cache miss leaked as error: %v", err)
	}
	if !tr.Data().IsSymlink {
		t.Fatal("should be symlink")
	}
}

func TestCantUseFileAsAbsPath(t *testing.T) {
	// Write a dummy file
	wd := NewTest("test", t)
	wd.WriteFile("a.txt", "aloha")

	// Try using that file as the path of a workdir
	osWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	wdPath := filepath.ToSlash(osWd)
	_, err = New(wdPath+"/test/a.txt", nil, 0)
	if err == nil {
		t.Fatal("expected err bc test/a.txt is a file")
	}
}

func TestPurgeAndIsEmpty(t *testing.T) {
	// Write a dummy file
	wd := NewTest("test", t)
	wd.WriteFile("a.txt", "aloha")

	if !wd.HasFile("a.txt") {
		t.Fatal("expected to have a.txt")
	}
	err := wd.Purge()
	if err != nil {
		t.Fatal(err)
	}
	if wd.HasFile("a.txt") {
		t.Fatal("expected to a.txt to be deleted")
	}
	isEmpty, err := wd.IsEmpty()
	if err != nil {
		t.Fatal(err)
	}
	if !isEmpty {
		t.Fatal("expected empty")
	}

	// The entry still works bc the root folder itself was not deleted
	wd.WriteFile("b.txt", "I'm ok")
	if !wd.HasFile("b.txt") {
		t.Fatal("expected to b.txt to exist")
	}

	isEmpty, err = wd.IsEmpty()
	if err != nil {
		t.Fatal(err)
	}
	if isEmpty {
		t.Fatal("expected not empty")
	}

}