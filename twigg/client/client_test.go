package client

import (
	"bytes"
	"errors"
	"monorepo/twigg/tree"
	"path"
	"reflect"
	"strings"
	"testing"
)

func TestSetupCreatesRoot(t *testing.T) {
	root, _, _, _ := newTestClient("owner", 1, t)
	if root.L != 0 {
		t.Fatal("root should have n=0")
	}
	if !root.IsOnServer() {
		t.Fatal("root commit starts out with server L and V")
	}
	if !root.IsSubmitted {
		t.Fatal("root commit starts out as submitted")
	}
}
func TestInvalidMsgLen(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)
	wd.WriteFile("a.txt", "hello")

	_, err := ag.Commit(wd, "", &root, l)
	if !errors.Is(err, errMsgTooShort) {
		t.Fatal("expected msg too short")
	}
	_, err = ag.Commit(wd, strings.Repeat("a", MaxMsgLen+1), &root, l)
	if !strings.Contains(err.Error(), "title can't be >") {
		t.Fatal("expected msg too long")
	}

	// exactly MaxMsgLen characters with multi-byte UTF-8 chars must succeed
	wd.WriteFile("b.txt", "hello")
	msg50Accented := strings.Repeat("é", MaxMsgLen)
	_, err = ag.Commit(wd, msg50Accented, &root, l)
	if err != nil {
		t.Fatalf("expected success for %d-char accented message, got: %v", MaxMsgLen, err)
	}
}
func TestNothingToCommit(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)
	_, err := ag.Commit(wd, "first commit", &root, l)
	if !errors.Is(err, ErrNothingToCommit) {
		t.Fatal("should show nothing to commit err")
	}

}
func TestGet(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)

	_, err := ag.GetLatest(9999, l)
	if err != ErrCommitNotFound {
		t.Fatalf("expecteed commit not found err: %s", err)
	}
	_, err = ag.GetVersion(0, 99999, l)
	if err != ErrCommitNotFound {
		t.Fatalf("expecteed commit not found err: %s", err)
	}

	wd.WriteFile("a.txt", "hello")
	c, _ := ag.Commit(wd, "first commit", &root, l)

	cGot, err := ag.GetLatest(c.L, l)
	if err != nil {
		t.Fatal(err)
	}
	// time doesnt work well with DeepEqual
	c.CreatedOn = cGot.CreatedOn
	if !reflect.DeepEqual(c, cGot) {
		t.Fatal("got wrong commit")
	}

}
func TestLoad(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)

	// c0 commit has a.txt="hello"
	wd.WriteFile("a.txt", "hello")
	c0, err := ag.Commit(wd, "c0", &root, l)
	if err != nil {
		t.Fatal(err)
	}
	if c0.RootDirHash != wd.RootDirHash() {
		t.Fatal("wrong root hash")
	}

	// c1 commit has a.txt="bye"
	wd.WriteFile("a.txt", "bye")
	c1, err := ag.Commit(wd, "c1", &root, l)
	if err != nil {
		t.Fatal(err)
	}
	if c1.RootDirHash != wd.RootDirHash() {
		t.Fatal("wrong root hash")
	}

	// Load c0 into workdir and check a.txt
	err = ag.Load(c0.TreeVersion, wd, l)
	if err != nil {
		t.Fatal(err)
	}
	if wd.ReadFile("a.txt") != "hello" {
		t.Fatal("wrong a.txt after loading c0")
	}
	if wd.RootDirHash() != c0.RootDirHash {
		t.Fatal("wrong root hash")
	}

	// Load c1 into workdir and check a.txt
	err = ag.Load(c1.TreeVersion, wd, l)
	if err != nil {
		t.Fatal(err)
	}
	if wd.ReadFile("a.txt") != "bye" {
		t.Fatal("wrong a.txt after loading c1")
	}
	if wd.RootDirHash() != c1.RootDirHash {
		t.Fatal("wrong root hash")
	}
}

// Helper to check iterator. It checks and advances it
func checkItFileAndDiffType(it tree.ParallelIterator,
	expectedFilePath string, expectedDiff tree.DiffType, t *testing.T) {
	if !it.CanGet() {
		t.Fatal("should be able to get " + expectedFilePath)
	}
	m := it.GetDiff()
	if m.Type != expectedDiff {
		t.Fatal("wrong diff type for " + expectedFilePath)
	}
	if m.Type == tree.DiffTypeUndefined {
		err := it.Next()
		if err != nil {
			t.Fatal(err)
		}
		return
	}

	itPath, itPathDepth, _, _ := it.Get()
	if itPath != expectedFilePath {
		t.Fatalf("expected path %s got %s", expectedFilePath, itPath)
	}
	if itPath == tree.RootPath {
		if itPathDepth != 0 {
			t.Fatalf("expected depth 0 got %d", itPathDepth)
		}
	} else {
		expectedDepth := uint32(strings.Count(itPath, "/") + 1)
		if itPathDepth != expectedDepth {
			t.Fatalf("expected depth %d got %d", expectedDepth, itPathDepth)
		}
	}
	if m.Data.BaseName != path.Base(itPath) {
		t.Fatal("wrong diff file for " + expectedFilePath)
	}
	err := it.Next()
	if err != nil {
		t.Fatal(err)
	}
}

func TestDiff(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)

	// c0 creates a.txt, b.txt
	wd.WriteFile("a.txt", "aaa")
	wd.WriteFile("b.txt", "bbb")
	wd.WriteFile("unchanged.txt", "unchanged")
	c0, _ := ag.Commit(wd, "first", &root, l)
	// c1 deletes a.txt, modify b.txt and create c.txt
	wd.Delete("a.txt")
	wd.WriteFile("b.txt", "BBB")
	wd.WriteFile("c.txt", "ccc")
	c1, _ := ag.Commit(wd, "second", &c0, l)

	// Diff c1 and c0
	diffIt, err := ag.Diff(c1.TreeVersion, c0.TreeVersion, l)
	if err != nil {
		t.Fatal(err)
	}
	checkItFileAndDiffType(diffIt, tree.RootPath, tree.DiffTypeAnyModified, t)
	checkItFileAndDiffType(diffIt, "a.txt", tree.DiffTypeDeleted, t)
	checkItFileAndDiffType(diffIt, "b.txt", tree.DiffTypeAnyModified, t)
	checkItFileAndDiffType(diffIt, "c.txt", tree.DiffTypeCreated, t)
	checkItFileAndDiffType(diffIt, "unchanged.txt", tree.DiffTypeNoChange, t)
	checkItFileAndDiffType(diffIt, tree.RootPath, tree.DiffTypeAnyModified, t)
	if diffIt.CanGet() {
		t.Fatal("should be done")
	}

	// Get all the unified diff
	buff := bytes.NewBuffer(nil)
	err = ag.WriteDiffAll(c1.TreeVersion, c0.TreeVersion, buff, l)
	if err != nil {
		t.Fatal(err)
	}
	checkDiffContains := func(s string) {
		if !strings.Contains(buff.String(), s) {
			t.Fatalf("diff %q doesnt contain %s", buff.String(), s)
		}
	}
	checkDiffContains("aaa")
	checkDiffContains("bbb")
	checkDiffContains("BBB")
	checkDiffContains("ccc")
	if strings.Contains(buff.String(), "unchanged") {
		t.Fatalf("diff %q contain %s", buff.String(), "unchanged")
	}
}

func TestWriteDiff(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)

	// c0 creates a.txt
	wd.WriteFile("a.txt", "aaa")
	c0, _ := ag.Commit(wd, "first", &root, l)
	// c1 deletes modifies a.txt and creates b.txt
	wd.WriteFile("a.txt", "AAA")
	wd.WriteFile("b.txt", "bbb")
	c1, _ := ag.Commit(wd, "second", &c0, l)

	// Get the text diff for a.txt
	buff := bytes.NewBuffer(nil)
	err := ag.WriteDiff(c1.TreeVersion, c0.TreeVersion, "a.txt", buff, l)
	if err != nil {
		t.Fatal(err)
	}
	textDiff := buff.String()
	if !strings.Contains(textDiff, "+AAA") {
		t.Error("wrong diff:\n" + textDiff)
	}
	if !strings.Contains(textDiff, "-aaa") {
		t.Error("wrong diff:\n" + textDiff)
	}

	// Get the text diff for b.txt
	buff = bytes.NewBuffer(nil)
	err = ag.WriteDiff(c1.TreeVersion, c0.TreeVersion, "b.txt", buff, l)
	if err != nil {
		t.Fatal(err)
	}
	textDiff = buff.String()
	if !strings.Contains(textDiff, "+bbb") {
		t.Error("wrong diff:\n" + textDiff)
	}

	// Test not found
	err = ag.WriteDiff(c1.TreeVersion, c0.TreeVersion, "non_existing_file.txt", nil, l)
	if err == nil {
		t.Fatal("should err bc file doesnt exist")
	}
}

func TestDiffWorkdir(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)

	// c0 creates a.txt and b.txt
	wd.WriteFile("a.txt", "aaa")
	wd.WriteFile("b.txt", "bbb")
	c0, _ := ag.Commit(wd, "first", &root, l)

	// Delete a.txt, modify b.txt and create c.txt
	wd.Delete("a.txt")
	wd.WriteFile("b.txt", "BBB")
	wd.WriteFile("c.txt", "ccc")
	diffsIt, err := ag.DiffWorkdir(wd, c0.TreeVersion, l)
	if err != nil {
		t.Fatal(err)
	}
	checkItFileAndDiffType(diffsIt, tree.RootPath, tree.DiffTypeUndefined, t)
	checkItFileAndDiffType(diffsIt, "a.txt", tree.DiffTypeDeleted, t)
	checkItFileAndDiffType(diffsIt, "b.txt", tree.DiffTypeAnyModified, t)
	checkItFileAndDiffType(diffsIt, "c.txt", tree.DiffTypeCreated, t)
	checkItFileAndDiffType(diffsIt, tree.RootPath, tree.DiffTypeAnyModified, t)
	if diffsIt.CanGet() {
		t.Fatal("should be empty")
	}
}

func TestRead(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)

	// c0 creates a.txt and b.txt
	wd.WriteFile("a.txt", "aaa")
	c0, _ := ag.Commit(wd, "first", &root, l)

	// Try reading non existing file
	err := ag.Read(c0.TreeVersion, "non existing file", nil, l)
	if !errors.Is(err, ErrFileNotFound) {
		t.Fatal("error shoud be file not found")
	}
	// Try reading non existing tree version
	err = ag.Read(99999, "", nil, l)
	if !errors.Is(err, ErrFileNotFound) {
		t.Fatal("error shoud be file not found")
	}

	// Read the actual file
	buff := bytes.NewBuffer(nil)
	err = ag.Read(c0.TreeVersion, "a.txt", buff, l)
	if err != nil {
		t.Fatal(err)
	}
	if buff.String() != "aaa" {
		t.Fatal("wrong content of a.txt")
	}

}
