package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"monorepo/buildmeta"
	"monorepo/twigg/client"
	"monorepo/twigg/commit"
	diff3 "monorepo/twigg/diff/epiclabs-io"
	"monorepo/twigg/server"
	"monorepo/twigg/tree"
	"monorepo/twigg/workdir"
	"monorepo/twigg/xchange"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
)

const FakeApiKey = "fake-test-api-key"

func TestInvalidArg(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("invalid arg")
	h.CheckOutContains(invalidCommand)
}

func TestHelp(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("--help")
	h.CheckOutContains("/docs/v/2")
}
func TestDebug(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init", "--debug")
	h.CheckOutContains("[DEBUG]")
	h.Run("st", "--debug")
	h.CheckOutContains("[DEBUG]")
}

func TestNothingCreatedBeforeInit(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("status")
	h.CheckOutContains(repoNotInitialized)
	h.CheckDirectoryDoesntExist(storageFolder)
}

func TestInit(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")
	h.CheckOutContains(repoCreated)

	h.Run("init")
	h.CheckOutContains(repoAlreadyInitialized)

	h.CheckActiveCommit(CheckCommitArg{Id: 0, Version: 0, IsSubmitted: true,
		HasServerId: true, ServerId: 0, HasServerV: true, ServerV: 0})
}

func TestSetAndDumpKey(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")
	h.CheckOutContains(repoCreated)

	h.Run("key", "abcde")
	h.CheckOutContains(apiKeyConfiguredOk)
	h.Run("key-dump")
	h.CheckOutContains("abcde")
}

func TestCommandsInSubfolder(t *testing.T) {
	h := NewTestHelperAt("test-a", t)
	h.Run("init")
	h.CheckOutContains(repoCreated)
	h.WriteFile("a/a.txt", "aaa")

	h2 := NewTestHelperAt("test-a/a", t)
	h2.Run("init")
	h2.CheckOutContains(repoAlreadyInitialized)
}

func TestLogIsInit(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("is-init")
	h.CheckOutContains(repoNotInitialized)

	h.Run("init")
	h.Run("is-init")
	h.CheckOutContains(isInitLogMessage)
}

func TestLogVersion(t *testing.T) {
	h := NewTestHelper(t)
	buildmeta.Version = "3.0"

	h.Run("version")
	h.CheckOutContains("3.0")

	h.Run("--version")
	h.CheckOutContains("3.0")

	h.Run("-version")
	h.CheckOutContains("3.0")

	h.Run("--v")
	h.CheckOutContains("3.0")

	h.Run("-v")
	h.CheckOutContains("3.0")
}

func TestSimpleCommitAndStatus(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")
	h.Run("status")
	h.CheckOutContains(noChanges)
	h.WriteFile("a/b.txt", "bbb")
	h.Run("status")
	h.CheckOutContains(fileStatus("a/b.txt", tree.DiffTypeCreated))
	h.Run("commit", "c1")
	h.Run("status")
	h.CheckOutContains(noChanges)
}

func TestLog(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	if len(h.Log()) != 1 {
		t.Fatal("log failed")
	}

	if h.ActiveCommit().Id != 0 {
		t.Fatal("active commit is not root")
	}

	// Create this commit tree.
	// c5
	// |
	// c4
	// |
	// c3
	// |
	// c2
	// |
	// c1
	// |
	// root

	// c1:
	h.WriteFile("a.txt", "111")
	h.Run("commit", "c1")
	// c2:
	h.WriteFile("a.txt", "222")
	h.Run("commit", "c2")
	// c3:
	h.WriteFile("a.txt", "333")
	h.Run("commit", "c3")
	// c4:
	h.WriteFile("a.txt", "444")
	h.Run("commit", "c4")
	// c5:
	h.WriteFile("a.txt", "555")
	h.Run("commit", "c5")

	// By default logs current and 3 previous (configured in test params)
	h.CheckLogN( /*number=*/ 5, []int{5, 4, 3, 2, 1, 0})
	h.CheckLogN( /*number=*/ 10, []int{5, 4, 3, 2, 1, 0})
	h.CheckLogN( /*number=*/ 1, []int{5, 4})
	h.CheckLogN( /*number=*/ 0, []int{5})
}

func TestLogJson(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")
	h.WriteFile("a.txt", "111")
	h.Run("commit", "c1")
	h.WriteFile("a.txt", "222")
	h.Run("commit", "c2")

	h.Run("log", "--json")
	out := h.Out()
	var l JsonLog
	err := json.Unmarshal([]byte(out), &l)
	if err != nil {
		t.Fatalf("bad json output: %s", err)
	}
	if len(l.Commits) != 3 {
		t.Fatalf("bad json num of commits: %d", len(l.Commits))
	}
	// Children appear before parents
	expected := []JsonCommit{
		{Id: "2v0", ParentId: "1v0", Message: "c2", IsCurrent: true,
			HasDiffData: true, DiffDataLinesModified: 1},
		{Id: "1v0", ParentId: "0v0", Message: "c1",
			HasDiffData: true, DiffDataLinesCreated: 1},
		{Id: "0v0", ServerId: "c/0v0", Message: firstCommitMsg,
			IsSubmitted: true, IsPushed: true, HasDiffData: true},
	}
	if !reflect.DeepEqual(l.Commits, expected) {
		t.Fatalf("bad json commits:\n%+v\nexpected:\n%+v", l.Commits, expected)
	}
}

func TestEmptyAndLongMsg(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	longMsg := strings.Repeat("a", client.MaxMsgLen+1)
	// Commit
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", longMsg)
	h.CheckOutContains("title can't be >")
	h.Run("commit", "")
	h.CheckOutContains("title can't be empty")
	h.Run("commit", "1")
	h.CheckActiveCommit(CheckCommitArg{
		Id:      1,
		Version: 0,
	})

	// Amend
	h.WriteFile("b.txt", "bbb")
	h.Run("amend", longMsg)
	h.CheckOutContains("title can't be >")
	h.Run("amend")
	h.CheckActiveCommit(CheckCommitArg{
		Id:      1,
		Version: 1,
	})
}
func TestHideAndUnhide(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")
	h.Run("hide", "0")
	h.CheckOutContains(cantHideSubmitted)

	// Create this commit tree.
	// c3
	// |
	// c2   c1
	// |   /
	// root
	// c1:
	h.WriteFile("a.txt", "111")
	h.Run("commit", "c1")
	h.Run("down")
	// c2:
	h.WriteFile("a.txt", "222")
	h.Run("commit", "c2")
	// c3:
	h.WriteFile("a.txt", "333")
	h.Run("commit", "c3")
	h.Run("hide", "2")
	h.CheckOutContains("is now hidden")
	/// Despite c2 being hidden, we expected c2 and c3 to appear because we are
	//  on c3. The current commit must always be shown.
	//
	// c3 @
	// |
	// c2(hidden) c1
	// |          /
	// root-------
	h.CheckLog(0, 1, 2, 3)
	// Try to hide again
	h.Run("hide", "2")
	h.CheckOutContains(alreadyHidden)
	// go to 1. Check only root and 1 appear
	h.Run("goto", "1")
	h.CheckLog(0, 1)
	// Hidden commits should appear with --all
	h.CheckLogAll(0, 1, 2, 3)

	// Unhide commit 2 and all commits (0, 1, 2, 3) should appear in log
	h.Run("unhide", "2")
	h.CheckLog(0, 1, 2, 3)

	// Try to unhide again
	h.Run("unhide", "2")
	h.CheckOutContains(notHidden)
}

func TestHiddenAreNotRebased(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create this commit tree.
	// c2
	// |
	// c1
	// |
	// root
	h.WriteFile("a.txt", "111")
	h.Run("commit", "c1")
	h.WriteFile("a.txt", "222")
	h.Run("commit", "c2")
	// Go down and hide c2
	// Expect:
	// @c1
	// |
	// root
	h.Run("down")
	h.Run("hide", "2")
	h.CheckLog(0, 1)

	// Ammending c1 should not rebase c2
	h.WriteFile("a.txt", "AMMEND")
	h.Run("amend")
	h.CheckLog(0, 1)
}

func TestHiddenAreNotAutoRebased(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create this commit tree.
	// c3
	// |
	// c2
	// |
	// c1
	// |
	// root
	h.WriteFile("c1.txt", "c1")
	h.Run("commit", "c1")
	h.WriteFile("c2.txt", "c2")
	h.Run("commit", "c2")
	h.WriteFile("c3.txt", "c3")
	h.Run("commit", "c3")
	// Go to c1 and hide c3
	// Expect:
	// c3
	// |
	// c2
	// |
	// @c1
	// |
	// root
	h.Run("goto", "1")
	h.Run("hide", "3")
	h.CheckLog(0, 1, 2)

	// Ammending c1 should not rebase or log c3
	h.WriteFile("c1.txt", "AMEND")
	h.Run("amend")
	h.CheckLog(0, 1, 2)
}

func TestIgnore(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Root ignore:
	// **/*.bin <- ignores all files anywhere that end with .bin
	// ignored/ <- ignores any folder anywhere that is named "ignored"
	// /ignoredAtRootOnly/ <- ignores "/ignoredAtRootOnly" folders only at the current dir (root).
	//                        /a/b/c/ignoredAtRootOnly/a.txt is not ignored
	h.WriteFile(workdir.IgnoreFileName, "**/*.bin\nignored/\n/ignoredAtRootOnly/")
	h.WriteFile("ignored.bin", "ignored file")
	h.WriteFile("sub/sub/sub/ignored.bin", "another ignored file")
	h.WriteFile("ignored/ignored.txt", "another ignored file")
	h.WriteFile("ignored/ignored/ignored.txt", "yet another ignored file")
	h.WriteFile("some_other_dir/some_other_subdir/ignored/ignored.txt", "this should be ignored")
	h.WriteFile("ignoredAtRootOnly/ignored.txt", "this should be ignored")
	h.WriteFile("some_other_dir/some_other_subdir/ignoredAtRootOnly/notIgnored.txt", "this should not be ignored")

	// Subfolder ignore:
	// sub_ignored/
	// **/*.tar
	h.WriteFile(fmt.Sprintf("subfolder/%s", workdir.IgnoreFileName), "sub_ignored/\n**/*.tar")
	h.WriteFile("subfolder/notIgnored.txt", "not ignored file")
	h.WriteFile("notIgnored.tar", "non ignored tar")
	h.WriteFile("subfolder/sub_ignored/ignored.txt", "file inside sub_ignored")
	h.WriteFile("subfolder/sub_ignored/sub/ignored.txt", "file inside sub_ignored")
	h.WriteFile("subfolder/ignored.tar", "ignored tar bc inside subfolder")
	h.WriteFile("subfolder/a/b/c/d/e/ignored.tar", "ignored tar bc inside subfolder")

	h.Run("status")
	// .twigg is always ignored
	h.CheckOutDoesntContain(".twigg")
	// The ignore files are not ignored
	h.CheckOutContains(workdir.IgnoreFileName)
	h.CheckOutContains(fmt.Sprintf("subfolder/%s", workdir.IgnoreFileName))

	h.CheckOutDoesntContain("ignored.txt")
	h.CheckOutDoesntContain("ignored.tar")
	h.CheckOutDoesntContain("ignored.bin")
	h.CheckOutContains("notIgnored.tar")
	h.CheckOutContains("subfolder/notIgnored.txt")
	h.CheckOutContains("some_other_dir/some_other_subdir/ignoredAtRootOnly/notIgnored.txt")
}

func TestNegationIgnore(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")
	h.WriteFile("dir/build/a.txt", "aaa")
	h.WriteFile("dir/build/b.txt", "bbb")

	// Per the git spec, if a whole dir is ignored its contents can't be unignored
	const ignoreWholeDir = `
build/
!build/a.txt
!build/b.txt
	`
	h.WriteFile("dir/"+workdir.IgnoreFileName, ignoreWholeDir)
	h.Run("status")
	h.CheckOutDoesntContain("a.txt")
	h.CheckOutDoesntContain("b.txt")

	const ignoreDirContentsAndUnignoreA = `
build/*
!build/a.txt
`
	h.WriteFile("dir/"+workdir.IgnoreFileName, ignoreDirContentsAndUnignoreA)
	h.Run("status")
	h.CheckOutContains("a.txt")
	h.CheckOutDoesntContain("b.txt")
	h.Run("commit", "add build/a.txt")

	h.Run("down")
	h.Run("up", "-f")
	h.CheckFile("dir/build/a.txt", "aaa")

	h.WriteFile("dir/build/a.txt", "AAAAA")
	h.Run("status")
	h.CheckOutContains("a.txt")
	h.CheckOutDoesntContain("b.txt")
}

func TestTwiggSaplingAndGitAreAlwaysIgnored(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	h.WriteFile(".git/some-git-file.txt", "some git data")
	h.WriteFile(".sl/some-sapling-file.txt", "some sapling data")
	h.Run("status")
	h.CheckOutContains(noChanges)
}

func TestCommit(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	h.Run("commit", "first commit")
	h.CheckOutContains(nothingToCommit)

	h.WriteFile("a.txt", "some file")
	h.Run("commit", "first commit")

	if len(h.Log()) != 2 {
		t.Fatal("expected root and the new commit")
	}
	h.CheckActiveCommit(CheckCommitArg{Id: 1, Version: 0, HasServerId: false})
}

func TestCommitSingleExecutable(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")
	h.WriteFile("bin", "bbb")
	h.Run("commit", "create file")
	if len(h.Log()) != 2 {
		t.Fatal("expected root and the new commit")
	}
	h.WriteExecutable("bin", "bbb")
	h.Run("commit", "make it executable")
	if len(h.Log()) != 3 {
		t.Fatal("expected root and the new commit")
	}
}

func TestCommitExistingButEmptyFile(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	h.WriteFile("sub/empty.txt", "")
	h.WriteFile("sub/sub/empty.txt", "")
	h.Run("status")
	h.CheckOutContains("sub/empty.txt")
	h.CheckOutContains("sub/sub/empty.txt")
	h.Run("commit", "first commit")
	h.CheckActiveCommit(CheckCommitArg{Id: 1, Version: 0})

	h.Run("diff")
	h.CheckOutContains("sub/empty.txt")
	h.CheckOutContains("sub/sub/empty.txt")
}

func TestCantCreateCommitThatOnlyCreatesEmptyFolders(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create a file in a deep folder
	h.WriteFile("a/b/a.txt", "aaa")
	// Delete just the file. The folders will remain there
	h.DeleteFile("a/b/a.txt")
	h.CheckDirectoryExists("a/b")

	h.Run("commit", "first commit")
	h.CheckOutContains(nothingToCommit)
}

func TestAmendAndBecomeLikeParent(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create a commit with a.txt=aaa
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")
	h.CheckActiveCommit(CheckCommitArg{
		Id:      1,
		Version: 0,
	})
	// Create a new commit on top that modifies a.txt=AAA
	h.WriteFile("a.txt", "AAA")
	h.Run("commit", "AAA")
	h.CheckActiveCommit(CheckCommitArg{
		Id:      2,
		Version: 0,
	})
	// Now amend the commit so that it ends up becoming just like the parent.
	// This is kinda akward but is ok; we allow child commits to be just like
	// their parents if they end up that way due to an amend
	h.WriteFile("a.txt", "aaa")
	h.Run("amend", "aaa")
	h.CheckActiveCommit(CheckCommitArg{
		Id:      2,
		Version: 1,
	})
}

func TestCommitFolderWithOnlyIgnoredChildren(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")
	h.WriteFile(".gitignore", "ignored-file.txt\nignored-folder/")
	h.WriteFile("subfolder/ignored-file.txt", "ignored file")
	h.WriteFile("subfolder/ignored-folder/a.txt", "ignored file 2")
	h.Run("commit", "create only ignore file")
	h.CheckActiveCommit(CheckCommitArg{
		Id:      1,
		Version: 0,
	})
	h.Run("status")
	h.CheckOutContains(noChanges)

	// Create another commit just to check that the ignored are kept in place
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "create a.txt")
	h.Run("down")
	h.CheckFile("subfolder/ignored-file.txt", "ignored file")
	h.CheckFile("subfolder/ignored-folder/a.txt", "ignored file 2")
	h.CheckHasNoFile("a.txt")
	h.Run("up")
	h.CheckFile("subfolder/ignored-file.txt", "ignored file")
	h.CheckFile("subfolder/ignored-folder/a.txt", "ignored file 2")
	h.CheckFile("a.txt", "aaa")
}

func TestCommitAndAmendFilesWithFilter(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	h.WriteFile("a.txt", "aaa")
	h.WriteFile("b.txt", "bbb")
	h.WriteFile("c.txt", "ccc")
	h.WriteFile("d.txt", "ddd")
	h.WriteFile("e.txt", "eee")

	// Trying to commit with a filter that matches nothing
	h.Run("commit", "first commit", "--filter=dasfdsafdsfasdf")
	h.CheckOutContains(nothingToCommit)

	// First only commit a.txt using `--filter=<file>` syntax
	h.Run("commit", "first commit", "--filter=a.txt")
	c0 := h.ActiveCommit()
	h.Run("status")
	h.CheckOutContains(c0.IdString)
	h.CheckOutDoesntContain("a.txt")
	h.CheckOutContains(fileStatus("b.txt", tree.DiffTypeCreated))
	h.CheckOutContains(fileStatus("c.txt", tree.DiffTypeCreated))
	h.CheckOutContains(fileStatus("d.txt", tree.DiffTypeCreated))
	h.CheckOutContains(fileStatus("e.txt", tree.DiffTypeCreated))

	// Amend to add b.txt using `--filter "<file>"` syntax
	h.Run("amend", "--filter", "\"b.txt\"")
	h.Run("status")
	h.CheckOutDoesntContain("a.txt")
	h.CheckOutDoesntContain("b.txt")
	h.CheckOutContains(fileStatus("c.txt", tree.DiffTypeCreated))
	h.CheckOutContains(fileStatus("d.txt", tree.DiffTypeCreated))
	h.CheckOutContains(fileStatus("e.txt", tree.DiffTypeCreated))

	// Amend to add c.txt and d.txt using `--filter "<file>, <file>"` syntax
	h.Run("amend", "--filter", "c.txt, d.txt")
	h.Run("status")
	h.CheckOutDoesntContain("a.txt")
	h.CheckOutDoesntContain("b.txt")
	h.CheckOutDoesntContain("c.txt")
	h.CheckOutDoesntContain("d.txt")
	h.CheckOutContains(fileStatus("e.txt", tree.DiffTypeCreated))
}

func TestCommitAndAmendDirectoriesWithFilter(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	h.WriteFile("a/b/excluded.txt", "aaa")
	h.WriteFile("a/b/c/d/excluded.txt", "aaa")
	h.WriteFile("a/b/c/d/e/included.txt", "aaa")
	h.WriteFile("x/excluded.txt", "aaa")
	h.WriteFile("y/included.txt", "aaa")

	h.Run("commit", "first commit", "--filter=a/b/c/d/e/included.txt, y/")
	h.Run("status")
	h.CheckOutDoesntContain("included.txt")
	h.CheckOutContainsN("excluded.txt", 3)
}

func TestCommitMovingFileToFolder(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	h.WriteFile("x.txt", "xxx")
	h.Run("commit", "create x.txt")

	h.WriteFile("a/a1.txt", "aaa")
	h.Run("commit", "a/a1.txt")

	h.WriteFile("a2.txt", "aaa")
	h.Run("commit", "a2.txt")

	h.DeleteFile("a2.txt")
	h.WriteFile("a/a2.txt", "aaa")
	h.Run("commit", "move a2.txt to a/a2.txt")

	h.Run("down")
	h.CheckOutContains("switched to commit")
}

func TestSimpleConflictFreeAmend(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create first commit
	h.WriteFile("a.txt", "some file")
	h.Run("commit", "first commit")

	// Add another on top
	h.WriteFile("b.txt", "another file")
	h.Run("commit", "second commit")

	// Amend the first commit
	h.Run("down")
	h.WriteFile("a.txt", "ammended file")
	h.Run("amend")
	h.CheckActiveCommit(CheckCommitArg{
		Id:      1,
		Version: 1,
	})

	// Expected commit tree (* is obsolete):
	//
	// *c2_v0  c2_v1
	// |       |
	// *c1_v0  c1_v1
	// |       |
	// root---/

	// The child commit should be auto updated
	commits := h.Log()
	for _, c := range commits {
		if c.IsObsolete {
			t.Fatal("commits should be auto-updated as there was no conflict")
		}
	}

	// log --all shows obsolete commits
	h.CheckLogAll(2, 2, 1, 1, 0)

	// Amend mesage
	h.Run("amend", "new message")
	h.CheckActiveCommit(CheckCommitArg{
		Id:      1,
		Version: 2,
	})

	// Should only show root, the ammended commit and the child
	if len(h.Log()) != 3 {
		t.Fatal("expected 3 commits")
	}
	commits = h.Log()
	for _, c := range commits {
		if c.IsObsolete {
			t.Fatal("commits should be auto-updated as there was no conflict")
		}
	}
}

func TestAmendBigFile(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create first commit with large content
	content0 := strings.Repeat("a", 100_000)
	h.WriteFile("video.mp4", content0)
	h.Run("commit", "first commit")

	// Modify it slightly and amend it
	content1 := "b" + strings.Repeat("a", 100_000)
	h.WriteFile("video.mp4", content1)
	h.Run("amend")
	h.CheckFile("video.mp4", content1)

	h.Run("down")
	h.CheckHasNoFile("video.mp4")
	h.Run("up")
	h.CheckFile("video.mp4", content1)
}

func TestAmendAndRestore(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")
	h.CheckActiveCommit(CheckCommitArg{
		Id:      1,
		Version: 0,
	})
	h.WriteFile("a.txt", "AAA")
	h.Run("amend", "AAA")
	h.CheckActiveCommit(CheckCommitArg{
		Id:      1,
		Version: 1,
	})

	// Since the workdir is clean and we're at the latest version,
	// we'll automatically go-to the restored one
	h.Run("restore", "1V0")
	h.CheckActiveCommit(CheckCommitArg{
		Id:      1,
		Version: 2,
	})
	h.CheckFile("a.txt", "aaa")

	h.Run("goto", "1v1")
	h.CheckFile("a.txt", "AAA")
	h.CheckActiveCommit(CheckCommitArg{
		Id:             1,
		Version:        1,
		ObsoleteReason: "restore",
	})

	h.CheckLogAllVersions(
		IdVersionAndConflict{
			Id:      1,
			Version: 2,
		},
		IdVersionAndConflict{
			Id:      1,
			Version: 1,
		},
		IdVersionAndConflict{
			Id:      1,
			Version: 0,
		},
		IdVersionAndConflict{
			Id:      0,
			Version: 0,
		},
	)
}
func TestRestoreAfterConflict(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")
	h.WriteFile("a.txt", "aaA")
	h.Run("commit", "change last letter to A")
	h.Run("down")
	h.WriteFile("a.txt", "bbb")
	h.Run("amend", "cause conflict on c2")
	h.Run("up")
	h.CheckActiveCommit(CheckCommitArg{
		Id:           2,
		Version:      1,
		HasConflicts: true,
	})

	h.Run("restore", "2v0")
	h.CheckActiveCommit(CheckCommitArg{
		Id:           2,
		Version:      2,
		HasConflicts: false,
	})
}

func TestEmptyClone(t *testing.T) {
	h := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h.SetServerRootUrl(srv.RootUrl())

	h.Run("clone", srv.ServerPath(), FakeApiKey)
	h.CheckOutContains(cloneOk)
	h.Cd(path.Base(srv.ServerPath()))
	h.Run("status")
	h.CheckOutContains(noChanges)

	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "create a.txt")
	h.Run("push")
	h.CheckOutContains(pushOk)
}

func TestEmptyCloneTypingApiKey(t *testing.T) {
	h := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h.SetServerRootUrl(srv.RootUrl())

	h.PrepareInput(FakeApiKey + "\n")
	h.Run("clone", srv.ServerPath())
	h.CheckOutContains("What's your CLI key?")
	h.CheckOutContains(cloneOk)
	h.CheckDirectoryExists(path.Base(srv.ServerPath()))

	// Expect error if api key is not typed
	h2 := NewTestHelper(t)
	h2.Run("clone", srv.ServerPath())
	h2.CheckOutContains("What's your CLI key?")
	h2.CheckOutContains(apiKeyNotProvided)
}

func TestCantCloneIfDirIsNotEmpty(t *testing.T) {
	h := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h.SetServerRootUrl(srv.RootUrl())
	cloneDirName := path.Base(srv.ServerPath())

	h.WriteFile(cloneDirName+"/file.txt", "xxx")
	h.Run("clone", srv.ServerPath(), FakeApiKey)
	h.CheckOutContains(cloneDirectoryIsNotEmpty)

	// Delete the file but keep the folder
	h.DeleteFile(cloneDirName + "/file.txt")
	h.Run("clone", srv.ServerPath(), FakeApiKey)
	h.CheckOutContains(cloneOk)
}

func TestCantCloneInSubDir(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")
	h.CheckOutContains(repoCreated)
	// Create a subfolder and cd into it
	h.WriteFile("sub/a.txt", "")
	h.DeleteFile("sub/a.txt")
	h.Cd("sub")

	h.Run("clone", "server/path", "api key")
	h.CheckOutContains(repoAlreadyInitialized)
}

func TestClonePullsLastSubmittedOnly(t *testing.T) {
	h := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h.SetServerRootUrl(srv.RootUrl())

	h.Run("clone", srv.ServerPath(), FakeApiKey)
	h.Cd(path.Base(srv.ServerPath()))
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "c1")
	h.WriteFile("b.txt", "bbb")
	h.Run("commit", "c2")
	h.Run("push")
	srv.Submit(1)
	srv.Submit(2)

	h2 := NewTestHelper2(t)
	h2.SetServerRootUrl(srv.RootUrl())

	// Clone should only pull the last submitted commit
	h2.Run("clone", srv.ServerPath(), FakeApiKey)
	h2.CheckOutContains(cloneOk)
	h2.Cd(path.Base(srv.ServerPath()))
	h2.CheckFile("a.txt", "aaa")
	h2.CheckFile("b.txt", "bbb")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     0,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     1,
	})
	h2.CheckLog(1)
	h2.Run("down")
	h2.CheckOutContains(parentNotFound)
}

func TestCantPullAndPushWithoutApiKey(t *testing.T) {
	h := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h.SetServerRootUrl(srv.RootUrl())

	h.Run("init")
	h.Run("server", srv.ServerPath())

	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")

	h.Run("pull")
	h.CheckOutContains(apiKeyNotSet)
	h.Run("push")
	h.CheckOutContains(apiKeyNotSet)

	h.Run("key", FakeApiKey)
	h.Run("push")
	h.CheckOutContains(pushOk)
	h.Run("pull")
	h.CheckOutContains(nothingToPull)

	// Test pull and push with a wrong key
	h.Run("key", FakeApiKey+"wrong")
	h.WriteFile("b.txt", "bbb")
	h.Run("commit", "bbb")
	h.Run("pull")
	h.CheckOutContains(badApiKey)
	h.Run("push")
	h.CheckOutContains(badApiKey)
}

func TestCantRestoreSubmitted(t *testing.T) {
	h := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h.SetServerRootUrl(srv.RootUrl())

	h.Run("init")
	h.Run("server", srv.ServerPath())
	h.Run("key", FakeApiKey)

	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")
	h.Run("push")
	h.WriteFile("a.txt", "AAA")
	h.Run("amend", "AAA")
	h.Run("push")
	srv.Submit(1)
	h.Run("pull")

	h.Run("restore", "1v0")
	h.CheckOutContains(cantRestoreSubmitted)
}

func TestRestoreDoesntSwitchCommitIfWorkdirIsDirty(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")
	h.WriteFile("a.txt", "AAA")
	h.Run("amend", "AAA")
	h.CheckActiveCommit(CheckCommitArg{
		Id:      1,
		Version: 1,
	})

	// Make the workdir dirty. Restoring old version of c1 won't make us
	// go-to the restored version.
	h.WriteFile("b.txt", "bbb")
	h.Run("restore", "1v0")
	h.CheckActiveCommit(CheckCommitArg{
		Id:             1,
		Version:        1,
		ObsoleteReason: "restore",
	})
	h.CheckLogAll(0, 1, 1, 1) // v0, c1_v0, c1_v1, c1_v2,
}

func TestRestoreDoesntSwitchIfOnOldVersion(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")
	h.WriteFile("a.txt", "AAA")
	h.Run("amend", "AAA")
	h.Run("goto", "1v0")
	h.CheckActiveCommit(CheckCommitArg{
		Id:             1,
		Version:        0,
		ObsoleteReason: "amend",
	})
	// We won't go-to the restored commit bc we're on an old version
	h.Run("restore", "1v0")
	h.CheckActiveCommit(CheckCommitArg{
		Id:             1,
		Version:        0,
		ObsoleteReason: "amend",
	})
	h.CheckLogAll(0, 1, 1, 1) // v0, c1_v0, c1_v1, c1_v2,
}

func TestRestoreKeepsServerId(t *testing.T) {
	h := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h.SetServerRootUrl(srv.RootUrl())
	h.Run("init")
	h.Run("server", srv.ServerPath())
	h.Run("key", FakeApiKey)

	// Create v0 and v1 of a commit
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")
	h.WriteFile("a.txt", "AAA")
	h.Run("amend", "AAA")
	// Push v1 -> commit will get a server id and version
	h.Run("push")
	h.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     1,
		HasServerId: true,
		ServerId:    1,
		HasServerV:  true,
		ServerV:     0,
	})

	// Now restore the v0. Since the v1 now has a server Id, the restored
	// version (which will now be v2) will keep the server id
	h.Run("restore", "1v0")
	h.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     2,
		HasServerId: true,
		ServerId:    1,
		HasServerV:  false,
	})
}

func TestUpDown(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Commit tree:
	//
	// *c3_v0 @c3_v1
	// |      |       c4(hidden)
	// *c2_v0 c2_c1--/
	// |      |
	// c1_v0--/
	h.WriteFile("c1.txt", "c1")
	h.Run("commit", "c1")
	h.WriteFile("c2.txt", "c2_v0")
	h.Run("commit", "c2_v0")
	h.WriteFile("c3.txt", "c3")
	h.Run("commit", "c3")
	h.Run("goto", "2")
	h.WriteFile("c2.txt", "c2_v1")
	h.Run("amend")
	h.WriteFile("c4.txt", "hidden")
	h.Run("commit", "c4(hidden)")
	h.Run("hide", "4")
	h.Run("goto", "3")
	h.CheckLogAllVersions(
		IdVersionAndConflict{
			Id:      4,
			Version: 0,
		},
		IdVersionAndConflict{
			Id:      3,
			Version: 1,
		},
		IdVersionAndConflict{
			Id:      3,
			Version: 0,
		},
		IdVersionAndConflict{
			Id:      2,
			Version: 1,
		},
		IdVersionAndConflict{
			Id:      2,
			Version: 0,
		},
		IdVersionAndConflict{
			Id:      1,
			Version: 0,
		},
		IdVersionAndConflict{
			Id:      0,
			Version: 0,
		},
	)

	h.Run("down")
	h.Run("down")
	h.Run("down")
	if h.ActiveCommit().Id != 0 {
		t.Fatal("expected to be at root")
	}

	h.CheckHasNoFile("c1.txt")
	h.CheckHasNoFile("c2.txt")
	h.CheckHasNoFile("c3.txt")
	h.CheckHasNoFile("c4.txt")
	h.Run("up")
	h.CheckActiveCommit(CheckCommitArg{Id: 1, Version: 0})
	h.CheckFile("c1.txt", "c1")
	h.CheckHasNoFile("c2.txt")
	h.CheckHasNoFile("c3.txt")
	h.CheckHasNoFile("c4.txt")

	h.Run("up")
	// note we preffer non obsolete
	h.CheckActiveCommit(CheckCommitArg{Id: 2, Version: 1})
	h.CheckFile("c1.txt", "c1")
	h.CheckFile("c2.txt", "c2_v1")
	h.CheckHasNoFile("c3.txt")
	h.CheckHasNoFile("c4.txt")

	h.Run("up")
	// note we preffer non hidden
	h.CheckActiveCommit(CheckCommitArg{Id: 3, Version: 1})
	h.CheckFile("c1.txt", "c1")
	h.CheckFile("c2.txt", "c2_v1")
	h.CheckFile("c3.txt", "c3")
	h.CheckHasNoFile("c4.txt")
}

func TestUpDownN(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Commit tree:
	// @c4
	// |
	// c3
	// |
	// c2
	// |
	// c1
	h.WriteFile("c1.txt", "c1")
	h.Run("commit", "c1")
	h.WriteFile("c2.txt", "c2")
	h.Run("commit", "c2")
	h.WriteFile("c3.txt", "c3")
	h.Run("commit", "c3")
	h.WriteFile("c4.txt", "c4")
	h.Run("commit", "c4")
	h.CheckLog(4, 3, 2, 1, 0)
	h.CheckActiveCommit(CheckCommitArg{Id: 4})

	// Go down 4 times
	h.Run("down", "4")
	h.CheckActiveCommitLocalId(0)
	// Cant go down again
	h.Run("down", "1")
	h.CheckOutContains(parentNotFound)

	// Test up bad inputs
	h.Run("up", "0")
	h.CheckOutContains(badNumberMsgPrefix)
	h.Run("down", "0")
	h.CheckOutContains(badNumberMsgPrefix)
	h.Run("up", "-1") // shows "not supported flag"
	h.CheckActiveCommitLocalId(0)
	h.Run("down", "-1") // shows "not supported flag"
	h.CheckActiveCommitLocalId(0)

	// Go up n times
	h.Run("up", "2")
	h.CheckActiveCommitLocalId(2)
	h.Run("up", "1")
	h.CheckActiveCommitLocalId(3)

	// Shows child not found if cant go up that many times
	// Commit tree:
	// c4
	// |
	// @c3
	// |
	// c2
	// |
	// c1
	h.Run("up", "2")
	h.CheckOutContains(childNotFound)
	h.Run("up", "1")
	h.CheckActiveCommitLocalId(4)
}

func TestGoto(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// c1: a.txt = aaa
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")
	c1 := h.ActiveCommit()
	// c2: b.txt = bbb
	h.WriteFile("a.txt", "bbb")
	h.Run("commit", "bbb")
	// c3: c.txt = ccc
	h.WriteFile("c.txt", "ccc")
	h.Run("commit", "ccc")

	h.Run("goto", c1.IdString)
	h.CheckFile("a.txt", "aaa")
}
func TestGotoLocalSyntax(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Tree:
	// #2
	// |
	// #1
	// |
	// 0# root

	// #1: a.txt=aaa
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "create a.txt")

	// #2: b.txt=bbb
	h.WriteFile("b.txt", "bbb")
	h.Run("commit", "create b.txt")

	h.Run("goto", "#1")
	h.CheckActiveCommit(CheckCommitArg{Id: 1, Version: 0})
}
func TestGotoServerSyntax(t *testing.T) {
	h := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h.SetServerRootUrl(srv.RootUrl())

	h.Run("init")
	h.Run("server", srv.ServerPath())
	h.Run("key", FakeApiKey)

	// Tree:
	// #2 c/2
	// |
	// #1 c/1
	// |
	// 0# c/0 root

	// #1: a.txt=aaa
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "create a.txt")

	// #2: b.txt=bbb
	h.WriteFile("b.txt", "bbb")
	h.Run("commit", "create b.txt")

	h.Run("push")
	h.Run("goto", "c/1")
	h.CheckActiveCommit(CheckCommitArg{Id: 1, Version: 0,
		HasServerId: true, ServerId: 1, HasServerV: true, ServerV: 0})

	// Goto with version

	// Commit tree (* is obsolete)
	// *#2 c/2v0     #2 c/2v1
	// |               |
	// *#1 c/1v0    @#1 c/1v1
	// |               |
	// 0# c/0 root ----/
	h.WriteFile("a.txt", "AAA")
	h.Run("amend", "aaa -> AAA")

	// *#2 c/2v0	 @#2 c/2v1
	// |			   |
	// *#1 c/1v0	  #1 c/1v1
	// |			   |
	// 0# c/0 root ----/
	h.Run("up")

	h.Run("push")

	// Goto with server version
	h.Run("goto", "c1v1")
	h.CheckActiveCommit(CheckCommitArg{Id: 1, Version: 1,
		HasServerId: true, ServerId: 1, HasServerV: true, ServerV: 1})
}

func TestGotoVersion(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")
	h.WriteFile("b.txt", "bbb")
	h.Run("commit", "bbb")
	h.WriteFile("c.txt", "ccc")
	h.Run("commit", "ccc")

	// Lets amend c1 so all the children are updated
	h.Run("goto", "1")
	h.WriteFile("a.txt", "AAA")
	h.Run("amend", "aaa -> AAA")

	// Commit tree (* is obsolete)
	// *c3_v0 c3_v1
	// |      |
	// *c2_v0 c2_v1
	// |      |
	// *c1_v0 c1_v1
	// |      |
	// root --/

	// Now we can "goto" version of commit
	h.Run("goto", "1v0")
	// Log will show only the non-obsolete commits and the commit we're at
	//        c3_v1
	//        |
	//        c2_v1
	//        |
	// *c1_v0 c1_v1
	// |      |
	// root --/
	commits := h.Log()
	if len(commits) != 5 {
		t.Fatal("expected 5 commits")
	}
	for _, c := range commits {
		if c.Id == 3 && c.Version == 1 {
			continue
		}
		if c.Id == 2 && c.Version == 1 {
			continue
		}
		if c.Id == 1 && c.Version == 1 {
			continue
		}
		if c.Id == 1 && c.Version == 0 {
			if !c.IsActive {
				t.Fatal("we should be at c0 v0")
			}
			if !c.IsObsolete {
				t.Fatal("c0 v0 is obsolete")
			}
			continue
		}
		if c.Id == 0 && c.Version == 0 {
			continue
		}
		t.Fatal("unexpected commit")
	}

	// Check other version syntaxes
	h.Run("goto", "1v1")
	a := h.ActiveCommit()
	if a.Id != 1 || a.Version != 1 {
		t.Fatal("wrong commit")
	}
	h.Run("goto", "1v0")
	a = h.ActiveCommit()
	if a.Id != 1 || a.Version != 0 {
		t.Fatal("wrong commit")
	}
	h.Run("goto", "2v0")
	a = h.ActiveCommit()
	if a.Id != 2 || a.Version != 0 {
		t.Fatal("wrong commit")
	}
	h.Run("goto", "2v1")
	a = h.ActiveCommit()
	if a.Id != 2 || a.Version != 1 {
		t.Fatal("wrong commit")
	}

	h.Run("goto", "1V99")
	h.CheckOutContains(commitNotFound)
}

func TestWarp(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")
	c0 := h.ActiveCommit()

	// c1: a.txt = aaa
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")
	c1 := h.ActiveCommit()
	// c2: b.txt = bbb
	h.WriteFile("b.txt", "bbb")
	h.Run("commit", "bbb")
	c2 := h.ActiveCommit()

	// Warping just moves the commit marker
	h.Run("warp", c0.IdString)
	if h.ActiveCommit().Id != c0.Id {
		t.Fatal("wrong active commit")
	}
	h.CheckFile("a.txt", "aaa")
	h.CheckFile("b.txt", "bbb")

	// warp up/down also works
	h.Run("warp", "up")
	if h.ActiveCommit().Id != c1.Id {
		t.Fatal("wrong active commit")
	}
	h.CheckFile("a.txt", "aaa")
	h.CheckFile("b.txt", "bbb")
	h.Run("warp", "down")
	if h.ActiveCommit().Id != c0.Id {
		t.Fatal("wrong active commit")
	}
	h.CheckFile("a.txt", "aaa")
	h.CheckFile("b.txt", "bbb")

	h.Run("warp", c2.IdString)
	if h.ActiveCommit().Id != c2.Id {
		t.Fatal("wrong active commit")
	}
	h.CheckFile("a.txt", "aaa")
	h.CheckFile("b.txt", "bbb")
}

func TestLoad(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")
	c0 := h.ActiveCommit()

	// c1: a.txt = aaa
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")
	c1 := h.ActiveCommit()
	// c2: a.txt = AAA
	h.WriteFile("a.txt", "AAA")
	h.Run("commit", "AAA")
	c2 := h.ActiveCommit()

	h.Run("goto", c0.IdString, "--yolo")
	h.CheckHasNoFile("a.txt")

	h.Run("load", c1.IdString)
	h.CheckFile("a.txt", "aaa")

	// Load requires clean directory
	h.Run("load", c2.IdString)
	h.CheckOutContains(workdirIsDirty)
	h.Run("load", c2.IdString, "--yolo")
	h.CheckFile("a.txt", "AAA")
}

func TestDirtyWorkdir(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create a first commit
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")

	// Change the workdir but don't commit yet, and try do go down to root
	h.WriteFile("a.txt", "AAA")
	h.Run("down")

	h.CheckOutContains(workdirIsDirty)

	// Using force (--yolo, --force or -f if you're lame) works
	h.Run("down", "--yolo")
	if h.ActiveCommit().Id != 0 {
		t.Fatal("force didn't work")
	}
}

func TestCleanConfirmed(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create a first commit
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")

	// Dirty the workdir: modify a.txt and create b.txt
	h.WriteFile("a.txt", "AAA")
	h.WriteFile("b.txt", "bbb")

	// Confirm the clean
	h.PrepareInput("y\n")
	h.Run("clean")

	// The changes to be discarded were shown before the prompt
	h.CheckOutContains(fileStatus("a.txt", tree.DiffTypeAnyModified))
	h.CheckOutContains(fileStatus("b.txt", tree.DiffTypeCreated))
	h.CheckOutContains(cleanOk)
	h.CheckFile("a.txt", "aaa")
	h.CheckHasNoFile("b.txt")
}

func TestCleanDeclined(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create a first commit
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")

	// Dirty the workdir
	h.WriteFile("a.txt", "AAA")

	// Decline the clean: the workdir is untouched
	h.PrepareInput("n\n")
	h.Run("clean")

	h.CheckOutContains(cleanAborted)
	h.CheckFile("a.txt", "AAA")
}

func TestCleanNoInput(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create a first commit
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")

	// Dirty the workdir
	h.WriteFile("a.txt", "AAA")

	// No input (EOF) counts as declining
	h.Run("clean")

	h.CheckOutContains(cleanAborted)
	h.CheckFile("a.txt", "AAA")
}

func TestCleanAlreadyClean(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create a first commit
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")

	// Nothing to clean: no prompt is shown
	h.Run("clean")

	h.CheckOutContains(nothingToClean)
	h.CheckOutDoesntContain(cleanConfirm)
}

func TestCleanForce(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create a first commit
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")

	// Dirty the workdir
	h.WriteFile("a.txt", "AAA")
	h.WriteFile("b.txt", "bbb")

	// Force skips the confirmation prompt
	h.Run("clean", "--yolo")

	h.CheckOutDoesntContain(cleanConfirm)
	h.CheckOutContains(cleanOk)
	h.CheckFile("a.txt", "aaa")
	h.CheckHasNoFile("b.txt")
}

func TestStatus(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create a first commit with "a.txt=aaa", "b.txt=bbb" and bin=binary
	h.WriteFile("a.txt", "aaa")
	h.WriteFile("b.txt", "bbb")
	h.WriteFile("bin", "binary")
	h.Run("commit", "aaa")

	// Test status when no changes are present
	h.Run("status")
	h.CheckOutContains(noChanges)

	// Delete a.txt, modify b.txt, create c.txt
	h.DeleteFile("a.txt")
	h.WriteFile("b.txt", "BBB")
	h.WriteFile("c.txt", "ccc")
	h.DeleteFile("bin")
	h.WriteExecutable("bin", "binary")
	h.Run("status")

	h.CheckOutContains(fileStatus("a.txt", tree.DiffTypeDeleted))
	h.CheckOutContains(fileStatus("b.txt", tree.DiffTypeAnyModified))
	h.CheckOutContains(fileStatus("c.txt", tree.DiffTypeCreated))
	h.CheckOutContains(fileStatus("bin", tree.DiffTypeAnyModified))
}

func TestStatusSameContentButDifferentTime(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create a first commit with "a.txt=aaa"
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "aaa")

	// Overwrite that file with something else,
	// but then write the original contents back. This will mess with the
	// file's metadata (ModTime), but status should still not show the file
	// as changed.
	h.WriteFile("a.txt", "other content")
	h.WriteFile("a.txt", "aaa")
	h.Run("status")
	h.CheckOutContains(noChanges)
}

func TestDiff(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create a first commit with "a.txt=aaa" and "b.txt=bbb"
	h.WriteFile("a.txt", "aaa")
	h.WriteFile("b.txt", "bbb")
	h.Run("commit", "c1")

	// Delete a.txt, modify b.txt and create c.txt
	h.DeleteFile("a.txt")
	h.WriteFile("b.txt", "BBB")
	h.WriteFile("c.txt", "ccc")
	h.Run("commit", "c2")

	// Create d.txt
	h.WriteFile("d.txt", "ddd")
	h.Run("commit", "c3")
	// Ammend it
	h.WriteFile("d.txt", "DDD")
	h.WriteFile("e.txt", "eee")
	h.Run("amend")

	// Go-to c2
	h.Run("down")

	// Here's the commit tree (~ just means doesnt exits):
	//
	// c3v0     ~a.txt~ , b.txt=BBB, c.txt=ccc, d.txt=ddd, ~e.txt~
	// |
	// |   c3v1 ~a.txt~ , b.txt=BBB, c.txt=ccc, d.txt=DDD, e.txt=eee
	// |  /
	// @c2 ~a.txt~ , b.txt=BBB, c.txt=ccc, ~d.txt~, ~e.txt~
	// |
	// c1  a.txt=aaa, b.txt=bbb, ~c.txt~, ~d.txt~, ~e.txt~
	// |
	// root

	// Without arguments, diff compares current commit with its parent
	h.Run("diff")
	h.CheckOutContains(fileStatus("a.txt", tree.DiffTypeDeleted))
	h.CheckOutContains(fileStatus("b.txt", tree.DiffTypeAnyModified))
	h.CheckOutContains(fileStatus("c.txt", tree.DiffTypeCreated))

	// When an argument is passed, it diffs the commit with its parent
	h.Run("diff", "3v1")
	h.CheckOutContains(fileStatus("d.txt", tree.DiffTypeCreated))
	h.CheckOutContains(fileStatus("e.txt", tree.DiffTypeCreated))

	// When two are passed, it diffs those two
	h.Run("diff", "3", "3v0")
	h.CheckOutContains(fileStatus("d.txt", tree.DiffTypeAnyModified))
	h.CheckOutContains(fileStatus("e.txt", tree.DiffTypeCreated))
	h.Run("diff", "3v0", "2")
	h.CheckOutContains(fileStatus("d.txt", tree.DiffTypeCreated))
	h.CheckOutDoesntContain("e.txt")

	// Test dumping the diff
	h.Run("diff", "--all", "3v0", "2")
	h.CheckOutContains("+ddd")
}

func TestDiffJson(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// c1 creates a.txt and b.txt
	h.WriteFile("a.txt", "aaa")
	h.WriteFile("b.txt", "bbb")
	h.Run("commit", "c1")
	// c2 deletes a.txt, modifies b.txt and creates c.txt
	h.DeleteFile("a.txt")
	h.WriteFile("b.txt", "BBB")
	h.WriteFile("c.txt", "ccc")
	h.Run("commit", "c2")

	// Without arguments, diff compares the current commit with its parent
	c2Files := []JsonDiffFile{
		{Path: "a.txt", Status: "deleted"},
		{Path: "b.txt", Status: "modified"},
		{Path: "c.txt", Status: "created"},
	}
	h.Run("diff", "--json")
	h.CheckOutJsonDiffFile(c2Files)
	// When two commits are passed, it diffs those two
	h.Run("diff", "2", "1", "--json")
	h.CheckOutJsonDiffFile(c2Files)
	// When one commit is passed, it diffs it with its parent
	h.Run("diff", "1", "--json")
	h.CheckOutJsonDiffFile([]JsonDiffFile{
		{Path: "a.txt", Status: "created"},
		{Path: "b.txt", Status: "created"},
	})
	// A commit has no changes against itself
	h.Run("diff", "1", "1", "--json")
	h.CheckOutJsonDiffFile([]JsonDiffFile{})

	h.Run("diff", "--json", "--all")
	h.CheckOutContains(allNotSupportedWithJson)
}

func TestDump(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")
	h.ActiveCommit()

	// Create c0 with "a.txt=aaa"
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "c0")
	c0 := h.ActiveCommit()
	// Create c1 with "b.txt=bbb"
	h.DeleteFile("a.txt")
	h.WriteFile("b.txt", "bbb")
	h.Run("commit", "c1")

	// Dump b.txt from the current commit
	h.Run("dump", "b.txt")
	h.CheckOutContains("bbb")

	// Dump a.txt from c0
	h.Run("dump", "a.txt", c0.IdString)
	h.CheckOutContains("aaa")

	// Try readind non existing
	h.Run("dump", "non_existing.txt", c0.IdString)
	h.CheckOutIsEmpty()
	h.Run("dump", "a.txt", "999999999999")
	h.CheckOutContains(commitNotFound)
}

func TestDumpParentShortcut(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")
	h.ActiveCommit()

	// Create c0 with "a.txt=aaa"
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "c0")
	// Create c1 with "b.txt=bbb"
	h.DeleteFile("a.txt")
	h.WriteFile("b.txt", "bbb")
	h.Run("commit", "c1")

	// Dump a.txt from parent using parent alias
	h.Run("dump", "a.txt", "p")
	h.CheckOutContains("aaa")
	h.Run("dump", "a.txt", "parent")
	h.CheckOutContains("aaa")
	h.Run("dump", "a.txt", "down")
	h.CheckOutContains("aaa")

}

func TestLogRootPath(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")
	h.Run("root")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	h.CheckOutContains(cwd)
}

func TestLogId(t *testing.T) {
	h := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h.SetServerRootUrl(srv.RootUrl())

	h.Run("init")
	h.Run("server", srv.ServerPath())
	h.Run("key", FakeApiKey)

	// Run id on dirty workdir
	h.WriteFile("a.txt", "aaa")
	h.Run("id")
	h.CheckOutContains("dirty")

	// Run id on non pushed commit
	h.Run("commit", "aaa")
	h.Run("id")
	h.CheckOutContains("not-pushed")
	h.Run("push")
	srv.Submit(1)

	// Run id on submitted commit
	h.Run("pull")
	h.Run("id")
	h.CheckOutContains("c1v1")
}

func TestSymlinkDeletionAndCreation(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create a first commit with a symlink `sl` -> `a.txt`
	h.WriteFile("a.txt", "aaa")
	h.WriteSymlink("sl", "a.txt")
	h.Run("commit", "aaa")

	// Delete the symlink and check status
	h.DeleteFile("sl")
	h.Run("status")
	h.CheckOutContains(fileStatus("sl", tree.DiffTypeDeleted))

	// Recreate the symlink
	h.Run("goto", "1", "-f")
	h.CheckSymlink("sl", "a.txt")

	// Dump a symlink. We should see the textual representation.
	h.Run("dump", "sl")
	h.CheckOutContains(tree.SymlinkString("a.txt"))
}

func TestAbsolutePathSymlinks(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	h.WriteSymlinkWithAbsPath("symlink1", "../relpath/to/a.bin")
	h.WriteSymlinkWithAbsPath("symlink2", "/bin/files/a.bin")
	h.WriteSymlinkWithAbsPath("symlink3", "path/a.bin")
	h.Run("status")
	h.CheckOutContains(fileStatus("symlink1", tree.DiffTypeCreated))
	h.CheckOutContains(fileStatus("symlink2", tree.DiffTypeCreated))
	h.CheckOutContains(fileStatus("symlink3", tree.DiffTypeCreated))
}

func TestSimpleRebaseUp(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")

	// c0: a.txt=aaa
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	c0 := h1.ActiveCommit()

	// c1: b.txt=bbb
	h1.Run("down")
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "bbb")

	// Tree:
	// c0   c1
	// |   /
	// root

	// Rebase c1 into c0:
	// c1
	// |
	// c0
	// |
	// root
	h1.Run("rebase", c0.IdString)
	h1.CheckOutContains(rebaseOk)
	h1.CheckFile("a.txt", "aaa")
	h1.CheckFile("b.txt", "bbb")
}

func TestSimpleRebaseDown(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	root := h1.ActiveCommit()

	// c0: a.txt=aaa
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "create a.txt")

	// c1: a.txt=aaa, b.txt=bbb
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "create b.txt")

	// Tree:
	// c1
	// |
	// c0
	// |
	// root

	// Rebase c1 into root:
	// c0   c1
	// |   /
	// root
	h1.Run("rebase", root.IdString)
	h1.CheckOutContains(rebaseOk)
	h1.CheckFile("b.txt", "bbb")
	h1.CheckHasNoFile("a.txt")
}

func TestSimpleTreeRebase(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")

	// c1: a.txt=aaa
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	c1 := h1.ActiveCommit()

	// c2: b.txt=bbb
	h1.Run("down")
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "bbb")
	c2 := h1.ActiveCommit()

	// c3: b.txt=bbb, c.txt=ccc
	h1.WriteFile("c.txt", "ccc")
	h1.Run("commit", "ccc")
	h1.CheckHasNoFile("a.txt")

	// Here's the commit tree. We're at @
	// c3@
	// |
	// c2   c1
	// |   /
	// root

	// Rebase c2 into c1. c3 should also be rebased.
	// We should be taken to the new c3:
	// c3'@
	// |
	// c2'
	// |
	// c1
	// |
	// root
	h1.Run("rebase", c2.IdString, c1.IdString)
	h1.CheckOutContains(rebaseOk)

	// c3
	h1.CheckFile("a.txt", "aaa")
	h1.CheckFile("b.txt", "bbb")
	h1.CheckFile("c.txt", "ccc")
	h1.Run("down")

	// c2
	h1.CheckFile("a.txt", "aaa")
	h1.CheckFile("b.txt", "bbb")
	h1.CheckHasNoFile("c.txt")
	h1.Run("down")

	// c1
	h1.CheckFile("a.txt", "aaa")
	h1.CheckHasNoFile("b.txt")
	h1.CheckHasNoFile("c.txt")
	h1.Run("down")

	// root
	h1.CheckHasNoFile("a.txt")
	h1.CheckHasNoFile("b.txt")
	h1.CheckHasNoFile("c.txt")
}

func TestRebaseIntoGrandparent(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")

	// c1: sub/a.txt=aaa
	h1.WriteFile("sub/a.txt", "aaa")
	h1.Run("commit", "aaa")

	// c2: sub/b.txt=bbb
	h1.WriteFile("sub/b.txt", "bbb")
	h1.Run("commit", "bbb")

	// Tree:
	// c2
	// |
	// c1
	// |
	// root

	// Rebase c2 into root:
	// c2*
	// |
	// c1   c2
	// |   /
	// root
	h1.Run("rebase", "0")
	h1.CheckOutContains(rebaseOk)
	h1.CheckFile("sub/b.txt", "bbb")
	h1.CheckHasNoFile("sub/a.txt")
}

func TestRebaseIntoGrandchild(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create this commit tree.
	// c3
	// |
	// c2
	// |
	// c1
	// |
	// root
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "c1")
	h.WriteFile("b.txt", "bbb")
	h.Run("commit", "c2")
	h.WriteFile("c.txt", "ccc")
	h.Run("commit", "c3")

	// Rebase c1 (granpa) into its grandkid.
	// Since c3 is a descendent of c1, the descendents of c1 are not auto
	// rebased to avoid an infinite recursion
	// c1
	// |
	// c3
	// |
	// c2
	// |
	// c1*
	// |
	// root
	h.Run("rebase", "1", "3")
	h.CheckLogAllVersions(
		IdVersionAndConflict{Id: 0, Version: 0},
		IdVersionAndConflict{Id: 1, Version: 0},
		IdVersionAndConflict{Id: 2, Version: 0},
		IdVersionAndConflict{Id: 3, Version: 0},
		IdVersionAndConflict{Id: 1, Version: 1},
	)
}

func TestRebaseDryRun(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")

	// c1: a.txt=aaa
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	// c2: a.txt=AAA
	h1.WriteFile("a.txt", "AAA")
	h1.Run("commit", "AAA")
	// Rebase c2 into root with dry run
	h1.Run("rebase", "0", "--dry-run")
	h1.CheckOutContains(rebaseWillSucceed)

	// Nothing should be modified; expect only:
	// c2v0
	// |
	// c1v0
	// |
	// root
	h1.CheckActiveCommit(CheckCommitArg{Id: 2, Version: 0})
	h1.CheckLogAll(2, 1, 0)
}

func TestSimpleRebaseConflict(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")

	// c1: a.txt=aaa
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")

	// c2: a.txt=AAA
	h1.Run("down")
	h1.WriteFile("a.txt", "AAA")
	h1.Run("commit", "AAA")

	// Rebase c2 into c1
	h1.Run("rebase", "1")
	h1.CheckOutContains(gotConflict)

	// Should move to the rebased commit
	h1.CheckActiveCommit(CheckCommitArg{Id: 2, Version: 1, HasConflicts: true})
}

func TestRebaseNoOp(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")

	// c1: creates a.txt=aaa, b.txt=bbb and c.txt=ccc
	h1.WriteFile("a.txt", "aaa")
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "c1")
	h1.Run("diff")
	h1.CheckOutContains("a.txt")
	h1.CheckOutContains("b.txt")

	// c2: aaa->c2
	h1.WriteFile("a.txt", "c2")
	h1.Run("commit", "c2")
	h1.Run("diff")
	h1.CheckOutContains("a.txt")
	h1.CheckOutDoesntContain("b.txt")

	// c3: bbb->c3
	h1.WriteFile("b.txt", "c3")
	h1.Run("commit", "c3")
	h1.Run("diff")
	h1.CheckOutDoesntContain("a.txt")
	h1.CheckOutContains("b.txt")

	// Tree:
	// c3 bbb->c3
	// |
	// c2 aaa->c2
	// |
	// c1 (create all)
	// |
	// root
	h1.CheckLog(0, 1, 2, 3)

	// Create a copy of c2 and rebase it into c3
	// Tree:
	// c3
	// |
	// c2     c2Copy
	// |      |
	// c1 ----/
	// |
	// root

	h1.Run("goto", "2")
	h1.Run("warp", "1")
	h1.Run("commit", "copy of c2")
	h1.Log()
	h1.Run("diff")
	h1.CheckOutContains("a.txt")
	h1.CheckOutDoesntContain("b.txt")
	h1.Run("rebase", "3")
	h1.CheckActiveCommit(CheckCommitArg{Id: 4, Version: 1})
	// The rebase should be no-op
	h1.Run("diff")
	h1.CheckOutDoesntContain("a.txt")
	h1.CheckOutDoesntContain("b.txt")
}

func TestCantCommitOnTopOfConflict(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")

	// c1: a.txt=aaa
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")

	// c2: a.txt=AAA
	h1.WriteFile("a.txt", "AAA")
	h1.Run("commit", "AAA")

	// Amend c1 so that c2 has a conflict
	h1.Run("down")
	h1.WriteFile("a.txt", "bbb")
	h1.Run("amend")
	h1.Run("up")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:           2,
		Version:      1,
		HasConflicts: true,
	})
	// Now try creating a commit on top of this one, which has conflicts.
	// This is now allowed
	h1.WriteFile("new-file.txt", "my new file")
	h1.Run("commit", "create a commit on top of a conflicting one")
	h1.CheckOutContains(cantCommitOnTopOfConflicts)
}

func TestRebaseOntoCommitWithConflict(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")

	// Create c1 and c2; amend c1 to cause conflict on c2.
	// Then create a new commit c3; and try to rebase it into c2:
	//
	//
	// *c2_v0  c2_v1(conflicts)
	// |       |
	// *c1_v0  c1_v1
	// |       |        c3
	// root----/--------/

	// c1: a.txt=aaa
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "c1")
	// c2: a.txt=AAA
	h1.WriteFile("a.txt", "AAA")
	h1.Run("commit", "c2")
	// Amend c1
	h1.Run("down")
	h1.WriteFile("a.txt", "bbb")
	h1.Run("amend", "c1_v1")
	// Create c3
	h1.Run("down")
	h1.WriteFile("b.txt", "new file")
	h1.Run("commit", "c3")

	// Now try rebasing c3 into c2
	h1.Run("rebase", "3", "2")
	h1.CheckOutContains(cantRebaseIntoConflicts)
}

func TestNothingToPull(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)

	h1.Run("pull")
	h1.CheckOutContains(nothingToPull)
	h1.Run("pull")
	h1.CheckOutContains(nothingToPull)
}

func TestPushOneByOne(t *testing.T) {
	h1 := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())

	h1.Run("init")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)

	// Create 2 stacked commits
	h1.WriteFile("b/b.txt", "bbb")
	h1.Run("commit", "bbb")
	h1.WriteFile("a.txt", "aaaa")
	h1.Run("commit", "aaaa")
	// Go back to the first commit an push it.
	// The child will not be pushed
	h1.Run("down")
	h1.Run("push")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     0,
		HasServerId: true,
		ServerId:    1,
		HasServerV:  true,
		ServerV:     0,
	})
	h1.Run("up")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          2,
		Version:     0,
		HasServerId: false,
		HasServerV:  false,
	})
	// Now push the child
	h1.Run("push")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          2,
		Version:     0,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     0,
	})
}

func TestCommitParentDataIsUpdatedOnPush(t *testing.T) {
	h1 := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())

	h1.Run("init")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)

	// Create 2 stacked commits
	h1.WriteFile("c1.txt", "c1")
	h1.Run("commit", "c1")
	h1.WriteFile("c2.txt", "c2")
	h1.Run("commit", "c2")
	// Push both
	h1.Run("push")

	// Now amend c1 to create c1_v1. c2_v1 will be auto-created.
	// Note that c1_v1 doesn't have serverL/ServerV yet. So c2_v1 doesn't have
	// ParentServerL nor ParentServerV
	h1.Run("down")
	h1.WriteFile("c1.txt", "C1")
	h1.Run("amend")

	// Now let's just push c1_v1
	h1.Run("push")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     1,
		HasServerId: true,
		ServerId:    1,
		HasServerV:  true,
		ServerV:     1,
	})

	// Now we push c2_v1. It's parent data should be updated.
	h1.Run("up")
	h1.Run("push")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          2,
		Version:     1,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     1,
	})
	c2V2OnServer := srv.GetLatest(2)
	if !c2V2OnServer.ParentIsOnServer() {
		t.Fatal("parent of c2v2 should be on the server")
	}
	if c2V2OnServer.ParentL != 1 {
		t.Fatal("wrong ParentL of commit on server")
	}
	if c2V2OnServer.ParentV != 1 {
		t.Fatal("wrong ParentV of commit on server")
	}
}

func TestPushAmmended(t *testing.T) {
	h1 := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())

	h1.Run("init")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)

	// Create 2 stacked commits and push both
	h1.WriteFile("b/b.txt", "bbb")
	h1.Run("commit", "bbb")
	h1.WriteFile("a.txt", "aaaa")
	h1.Run("commit", "aaaa")
	h1.Run("push")

	// Now amend the second and push again.
	// The previous pushed one will be used as base
	h1.WriteFile("a.txt", "AAAA")
	h1.Run("amend", "AAAA")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          2,
		Version:     1,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  false,
	})
	h1.Run("push")
	h1.CheckOutContains(pushOk)
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          2,
		Version:     1,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     1,
	})
}

func TestPushStackAndCheckDiffOnTheServer(t *testing.T) {
	h1 := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())

	h1.Run("init")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)

	// Create 2 stacked commits and push once
	h1.WriteFile("b/b.txt", "bbb")
	h1.Run("commit", "bbb")
	h1.WriteFile("a.txt", "aaaa")
	h1.Run("commit", "aaaa")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:      2,
		Version: 0,
	})
	h1.Run("push")
	h1.CheckOutContains(pushOk)
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          2,
		Version:     0,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     0,
	})

	root := srv.GetLatest(0)
	c1 := srv.GetLatest(1)
	d := srv.Diff(c1, root)

	// Remove some fields we don't care about
	stripFields := func(d []tree.Diff) {
		for i := range d {
			d[i].Data.LastModifiedUnixMillis = 0
			d[i].Data.ContentHash = [32]byte{}
		}
	}
	// removes entries that are not files
	filterOutDirs := func(diffs []tree.Diff) []tree.Diff {
		c := []tree.Diff{}
		for _, d := range diffs {
			if d.Data.IsDir {
				continue
			}
			c = append(c, d)
		}
		return c
	}

	d = filterOutDirs(d)
	stripFields(d)
	if !reflect.DeepEqual(
		d, []tree.Diff{
			{
				Type: tree.DiffTypeCreated,
				Data: tree.Data{
					BaseName: "b.txt",
					IsDir:    false, IsText: true, Size: 3, Depth: 2,
				},
			},
		}) {
		t.Fatal("wrong diff for c1")
	}
	c2 := srv.GetLatest(2)
	d = srv.Diff(c2, c1)
	d = filterOutDirs(d)
	stripFields(d)
	if !reflect.DeepEqual(
		d, []tree.Diff{
			{
				Type: tree.DiffTypeCreated,
				Data: tree.Data{
					BaseName: "a.txt",
					IsDir:    false, Depth: 1, IsText: true, Size: 4,
				},
			},
			{
				Type: tree.DiffTypeNoChange,
				Data: tree.Data{
					BaseName: "b.txt",
					IsDir:    false, IsText: true, Size: 3, Depth: 2,
				},
			},
		}) {
		t.Fatal("wrong diff for c2")
	}
}

func TestPushCommitAndPush(t *testing.T) {
	h1 := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())

	h1.Run("init")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)

	// Create 2 stacked commits and push once
	h1.WriteFile("b/b.txt", "bbb")
	h1.Run("commit", "bbb")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:      1,
		Version: 0,
	})
	h1.WriteFile("a.txt", "aaaa")
	h1.Run("commit", "aaaa")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:      2,
		Version: 0,
	})
	h1.Run("push")
	h1.CheckOutContains(pushOk)
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          2,
		Version:     0,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     0,
	})

	// Write a new commit and push this one
	h1.WriteFile("c/c.txt", "ccc")
	h1.Run("commit", "ccc")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:      3,
		Version: 0,
	})
	h1.Run("push")
	h1.CheckOutContains(pushOk)
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          3,
		Version:     0,
		HasServerId: true,
		ServerId:    3,
		HasServerV:  true,
		ServerV:     0,
	})
}

func TestPushSymlink(t *testing.T) {
	h1 := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())

	h1.Run("init")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)

	// Create a first commit with a symlink `a` -> `b.txt`
	h1.WriteFile("a/b.txt", "bbb")
	h1.WriteSymlink("a/a", "a/b.txt")
	h1.Run("commit", "aaa")
	h1.Run("push")
	h1.CheckOutContains(pushOk)
	srv.Submit(1)

	h2 := NewTestHelper2(t)
	h2.SetServerRootUrl(srv.RootUrl())

	h2.Run("init")
	h2.Run("server", srv.ServerPath())
	h2.Run("key", FakeApiKey)
	h2.Run("pull")
	h2.Run("up")
	h2.CheckSymlink("a/a", "a/b.txt")
	h2.CheckFile("a/b.txt", "bbb")
}

func TestSimplePushSubmit(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())

	// Client 1 creates a commit, sets server url and pushes
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)

	h1.Run("push")
	commits := h1.Log()
	if len(commits) != 2 {
		t.Fatal("client should have 2 commits")
	}
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     0,
		HasServerId: true,
		ServerId:    1,
		HasServerV:  true,
		ServerV:     0,
	})

	// Server submits the first CL
	srv.Submit(1)

	// Client 1 puls
	h1.Run("pull")
	commits = h1.Log()
	if len(commits) != 2 {
		t.Fatal("client should still have 2 commits")
	}
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     1,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    1,
		HasServerV:  true,
		ServerV:     1,
	})
}

func TestSimplePushThatRequiresRebasing(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())

	// Client creates a commit, sets server url and pushes
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "c1")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)
	h1.Run("push")

	// Client create another commit on top of the root, so that it requires
	// rebasing
	h1.Run("down")
	h1.WriteFile("dir/b.txt", "bbb")
	h1.Run("commit", "c2")
	h1.Run("push")

	commits := h1.Log()
	if len(commits) != 3 {
		t.Fatal("client should still have 3 commits (root, c1, c2)")
	}

	// Server submits the commits without a problem.
	// Note that the second requires a rebase.
	srv.Submit(1)
	srv.Submit(2)
}

func TestOtherClientPullsSubmitted(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	h2 := NewTestHelper2(t)
	h2.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())
	h2.SetServerRootUrl(srv.RootUrl())

	// Client 1 creates a commit, sets server url and pushes
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)
	h1.Run("push")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     0,
		HasServerId: true,
		ServerId:    1,
		HasServerV:  true,
		ServerV:     0,
	})

	// Server submits the first CL
	srv.Submit(1)

	// Client 2 puls
	h2.Run("server", srv.ServerPath())
	h2.Run("key", FakeApiKey)
	h2.Run("pull")
	h2Commits := h2.Log()
	if len(h2Commits) != 2 {
		t.Fatal("client 2 should have pulled c0")
	}

	// Client 2 can go to the pulled commit
	h2.Run("up")
	h2.CheckFile("a.txt", "aaa")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     0, // Locally it has v0
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    1,
		HasServerV:  true,
		ServerV:     1, // On the server its v1 bc v0 was submitted to become v1
	})
}

func TestPullSubmitWithRebase(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)

	// Client creates two commits that are children of root
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	h1.Run("push")
	h1.Run("down")
	h1.DeleteFile("a.txt")
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "bbb")
	h1.Run("push")
	// This is the commit tree
	// aaa(1)  @bbb(2)
	// |      /
	// root---

	// Server submits one CL at a time
	srv.Submit(1)
	srv.Submit(2)
	// This is the tree we expect. Note that (2) will be rebased.
	// Since the workdir was clean, we'll go-to the new commit
	// @bbb(2)
	// |
	// aaa(1)
	// |
	// root
	h1.Run("pull")

	// Check the commits
	h1.Run("goto", "1", "--yolo")
	h1.CheckFile("a.txt", "aaa")
	h1.CheckHasNoFile("b.txt")
	h1.Run("up")
	h1.CheckFile("a.txt", "aaa")
	h1.CheckFile("b.txt", "bbb")
}

func TestPullGotoIfWorkdirIsDirty(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)

	// Create one commit and push and submit it
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	h1.Run("push")
	srv.Submit(1)

	// Before pulling, change something in the workdir.
	// We won't auto switch to the new one
	h1.WriteFile("a.txt", "aAa")
	h1.Run("pull")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:             1,
		Version:        0,
		HasServerId:    true,
		ServerId:       1,
		HasServerV:     true,
		ServerV:        0,
		ObsoleteReason: "pull",
	})
	h1.Run("status")
	h1.CheckOutContains(fileStatus("a.txt", tree.DiffTypeAnyModified))

	// If the workdir is clean, go-to the new version of the current commit
	h2 := NewTestHelper2(t)
	h2.SetServerRootUrl(srv.RootUrl())

	h2.Run("init")
	h2.Run("server", srv.ServerPath())
	h2.Run("key", FakeApiKey)
	h2.WriteFile("b.txt", "bbb")
	h2.Run("commit", "bbb")
	h2.Run("push")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     0,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     0,
	})
	srv.Submit(2)
	h2.Run("pull")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     1,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     1,
		IsSubmitted: true,
	})
	h2.Run("status")
	h2.CheckOutContains(noChanges)
}

func TestAmendAndPush(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())

	// Create a commit and push
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	h1.CheckActiveCommit(
		CheckCommitArg{Id: 1, Version: 0, HasServerId: false})
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)
	h1.Run("push")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     0,
		HasServerId: true,
		ServerId:    1,
		HasServerV:  true,
		ServerV:     0,
	})

	// Amend and push again
	h1.Run("amend", "ammended message")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     1,
		HasServerId: true,
		ServerId:    1})
	h1.Run("push")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     1,
		HasServerId: true,
		ServerId:    1,
		HasServerV:  true,
		ServerV:     1})

	c1 := srv.GetLatest(1)
	if c1.IsSubmitted || !c1.HasServerV || !c1.HasServerL {
		t.Fatal("c1 should be pending on the server")
	}
}

func TestPushSubmitAmendPush(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())

	// Create a commit and push
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	h1.CheckActiveCommit(CheckCommitArg{Id: 1, Version: 0})
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)
	h1.Run("push")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     0,
		HasServerId: true,
		ServerId:    1,
		HasServerV:  true,
		ServerV:     0})

	// Submit it on the server
	srv.Submit(1)

	// Before pulling, amend the commit. Then try pushing again
	h1.WriteFile("a.txt", "AAA")
	h1.Run("amend")
	h1.CheckActiveCommit(CheckCommitArg{
		Id: 1, Version: 1,
		HasServerId: true,
		ServerId:    1})
	h1.Run("push")
	h1.CheckOutContains("already submitted and can't be modified")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     1,
		HasServerId: true,
		ServerId:    1})
}

func TestPushSubmitAmendPull(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())

	// Create a commit and push
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)
	h1.Run("push")
	// Add a child and push
	h1.WriteFile("c.txt", "ccc")
	h1.Run("commit", "ccc")
	h1.Run("push")
	h1.Log()
	// Commit tree:
	//
	// @c2_v0
	// |
	// c1_v0
	// |
	// root

	// Submit it on the server
	srv.Submit(1)

	// Before pulling, amend the first commit.
	// Pulling will "overwrite" it (i.e. create a sucessor).
	// You shouldn't really do this but here's what happens:
	h1.Run("down")
	h1.WriteFile("a.txt", "AAA")
	h1.Run("amend", "AAA")
	// Commit tree (* represents obsolete):
	//
	// *c2_v0  c2_v1
	// |       |
	// *c1_v0  @c1_v1
	// |       |
	// root ---/
	h1.Run("pull")
	// Pull the submitted version of c1 will be pulled, and it'll overwrite
	// the c1_v1. c2_v1 won't be auto rebased (we don't do that on pulls)
	//
	// Commit tree (we're at @, * represents obsolete):
	//
	// *c2_v0  c2_v1
	// |       |
	// *c1_v0  *c1_v1 @c1_v2
	// |       |      |
	// root ---/------/

	h1.CheckLogAllVersions(
		IdVersionAndConflict{2, 1, false},
		IdVersionAndConflict{2, 0, false},
		IdVersionAndConflict{1, 2, false},
		IdVersionAndConflict{1, 1, false},
		IdVersionAndConflict{1, 0, false},
		IdVersionAndConflict{0, 0, false},
	)

	// *c2_v0  c2_v1
	// |       |
	// *c1_v0  *c1_v1 @c1_v2
	// |       |      |
	// root ---/------/
	h1.Run("goto", "-f", "2") // -> go-to c2_v1
	h1.CheckFile("c.txt", "ccc")
	h1.Run("down") // -> go-to c1_c1
	h1.CheckFile("a.txt", "AAA")
	h1.Run("down") // -> go-to root
	h1.Run("up")   // -> go-to c1_v2 (submitted takes precedence)
	h1.CheckFile("a.txt", "aaa")
}

func TestPushSubmitRebasePull(t *testing.T) {
	// This test is similar to TestPushSubmitAmendPull,
	// but instead of ammending we rebase the original commit
	h1 := NewTestHelper(t)
	h1.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())

	// Create some commits and push
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)
	h1.Run("push")
	h1.WriteFile("c.txt", "ccc")
	h1.Run("commit", "ccc")
	h1.WriteFile("d.txt", "ddd")
	h1.Run("commit", "ddd")
	h1.Run("push")
	h1.Run("down")
	h1.Run("down")
	h1.Run("down")
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "bbb")

	// This is the local tree:
	// c3_v0
	// |
	// c2_v0
	// |
	// c1_v0   c4_v0
	// |       /
	// root ---
	h1.CheckLogAllVersions(
		IdVersionAndConflict{4, 0, false},
		IdVersionAndConflict{3, 0, false},
		IdVersionAndConflict{2, 0, false},
		IdVersionAndConflict{1, 0, false},
		IdVersionAndConflict{0, 0, false},
	)

	// Submit it on the server
	srv.Submit(1)

	// Now before pulling rebase that commit into something else locally
	h1.Run("rebase", "1", "4")
	// Commit tree (* represents obsolete)
	//          c3_v1
	//          |
	// *c3_v0   c2_v1
	// |        |
	// *c2_v0   c1_v1
	// |        |
	// *c1_v0   c4_v0
	// |       /
	// root ---
	//
	// Amend to change the message and make things more complicated :)
	h1.Run("goto", "-f", "1")
	h1.Run("amend", "c1 rebased and ammended")
	// Commit tree (* represents obsolete)
	//
	//          *c3_v1   c3_v2
	//          |        |
	// *c3_v0   *c2_v1   c2_v2
	// |        |        |
	// *c2_v0   *c1_v1   c1_v2
	// |        |        |
	// *c1_v0   c4_v0----/
	// |       /
	// root ---
	h1.CheckLogAllVersions(
		IdVersionAndConflict{4, 0, false},
		IdVersionAndConflict{3, 2, false},
		IdVersionAndConflict{3, 1, false},
		IdVersionAndConflict{3, 0, false},
		IdVersionAndConflict{2, 2, false},
		IdVersionAndConflict{2, 1, false},
		IdVersionAndConflict{2, 0, false},
		IdVersionAndConflict{1, 2, false},
		IdVersionAndConflict{1, 1, false},
		IdVersionAndConflict{1, 0, false},
		IdVersionAndConflict{0, 0, false},
	)

	// Since c1 was submitted, it'll be pulled and will replace c1_v2.
	h1.Run("pull")
	// Commit tree (* represents obsolete)
	//
	//          *c3_v1   c3_v2
	//          |        |
	// *c3_v0   *c2_v1   c2_v1
	// |        |        |
	// *c2_v0   *c1_v1   *c1_v2
	// |        |        |
	// *c1_v0   c4_v0----/       c1_v3
	// |       /                 |
	// root ---------------------/
	h1.CheckLogAllVersions(
		IdVersionAndConflict{4, 0, false},
		IdVersionAndConflict{3, 2, false},
		IdVersionAndConflict{3, 1, false},
		IdVersionAndConflict{3, 0, false},
		IdVersionAndConflict{2, 2, false},
		IdVersionAndConflict{2, 1, false},
		IdVersionAndConflict{2, 0, false},
		IdVersionAndConflict{1, 3, false},
		IdVersionAndConflict{1, 2, false},
		IdVersionAndConflict{1, 1, false},
		IdVersionAndConflict{1, 0, false},
		IdVersionAndConflict{0, 0, false},
	)

	h1.Run("goto", "-f", "1")
	if !h1.ActiveCommit().IsSubmitted {
		t.Fatal("1 is submitted")
	}
	h1.CheckFile("a.txt", "aaa")
	h1.Run("goto", "-f", "2")
	h1.CheckFile("c.txt", "ccc")
	h1.Run("up")
	h1.CheckFile("d.txt", "ddd")
	h1.Run("goto", "4")
	h1.CheckFile("b.txt", "bbb")
}

func TestPullRebasedCommit(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	h2 := NewTestHelper2(t)
	h2.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())
	h2.SetServerRootUrl(srv.RootUrl())

	// Client 1 creates a commit, sets server url and pushes
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)
	h1.Run("push")
	// Commit tree:
	//
	// aaa
	// |
	// root

	// Client 2 does the same
	h2.WriteFile("b.txt", "bbb")
	h2.Run("commit", "bbb")
	h2.Run("server", srv.ServerPath())
	h2.Run("key", FakeApiKey)
	h2.Run("push")
	// Client 2 creates another commit on top of the first one and pushes
	h2.WriteFile("c.txt", "ccc")
	h2.Run("commit", "ccc")
	h2.Run("push")
	h2.Log()
	// Commit tree:
	//
	// ccc(2)
	// |
	// bbb(1)
	// |
	// root

	// The server now has both commits. Neither is attached to the root
	// because they were not submitted yet:
	//
	//             ccc (c/3)
	//
	// aaa (c/1)   bbb (c/2)
	//
	// root

	// Server submits the first CL and then the second.
	// This will cause the commit from client 2 (bbb) to be rebased into aaa:
	//
	// ccc (c/3)
	//
	// bbb (c/2)
	// |
	// aaa (c/1)
	// |
	// root
	srv.Submit(1)
	srv.Submit(2)

	// Client 2 puls and gets the following tree (* are obsolete):
	//
	// ccc(2)   bbb(1) (Submitted)
	// |        |
	// bbb*(1)  aaa(3) (Submitted)
	// |        |
	// root ----/
	h2.Run("pull")
	h2.CheckLogAllVersions(
		IdVersionAndConflict{3, 0, false},
		IdVersionAndConflict{2, 0, false},
		IdVersionAndConflict{1, 1, false},
		IdVersionAndConflict{1, 0, false},
		IdVersionAndConflict{0, 0, false},
	)
	h2.Run("goto", "-f", "3")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          3,
		Version:     0,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    1,
		HasServerV:  true,
		ServerV:     1,
	})
	h2.CheckFile("a.txt", "aaa")
	h2.Run("up")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     1,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     1,
	})
	h2.CheckFile("a.txt", "aaa")
	h2.CheckFile("b.txt", "bbb")
	h2.Run("goto", "2")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          2,
		Version:     0,
		IsSubmitted: false,
		HasServerId: true,
		ServerId:    3,
		HasServerV:  true,
		ServerV:     0,
	})
	h2.CheckFile("c.txt", "ccc")
	h2.CheckFile("b.txt", "bbb")
	h2.CheckHasNoFile("a.txt")
}

func TestTop(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	h2 := NewTestHelper2(t)
	h2.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())
	h2.SetServerRootUrl(srv.RootUrl())

	// Push and submit 2 commits from client1
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "c1")
	h1.DeleteFile("a.txt")
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "c2")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)
	h1.Run("push")
	srv.Submit(1)
	srv.Submit(2)
	// Here's the final commit tree:
	//
	// *c2_v0 (a.txt deleted, b.txt=bbb)  c2_v1(a.txt deleted, b.txt=bbb)
	// |                                  |
	// *c1_v0 (a.txt=aaa)                 c1_v1 (a.txt=aaa)
	// |                                  |
	// root ------------------------------/

	// Client 2 puls
	h2.Run("server", srv.ServerPath())
	h2.Run("key", FakeApiKey)
	h2.Run("pull")
	h2Commits := h2.Log()
	if len(h2Commits) != 3 {
		t.Fatal("client 2 should have pulled c0 and c1")
	}

	// Top will take us to the second commit
	h2.Run("top")
	h2.CheckFile("b.txt", "bbb")
	h2.CheckHasNoFile("a.txt")
	h2.Run("top")
	h2.CheckOutContains(alreadyAtTop)
}

func TestCantAmendWithWorkdirConflict(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")

	// Cause a rebase conflict
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	c0 := h1.ActiveCommit()
	h1.Run("down")
	h1.WriteFile("a.txt", "AAA")
	h1.Run("commit", "AAA")
	h1.Run("rebase", c0.IdString)
	h1.CheckOutContains(gotConflict)

	h1.WriteFile("b.txt", "random file")
	h1.Run("amend")
	h1.CheckOutContains(fileHasUnresolvedConflicts("a.txt"))
	h1.CheckOutContains(cantAmendWithConflicts)
	if !h1.ActiveCommit().HasConflicts {
		t.Fatal("expected conflicts to not be solved")
	}

	// Files which just so happen to have conflict markers are ok
	h1.WriteFile("random_other_file.txt", "\n"+diff3.ConflictStart)
	// But the conflict on a.txt must be solved.
	// Write a huge file to make sure the conflict checker handles it well
	h1.WriteFile("a.txt", strings.Repeat("x", 100_000))
	h1.Run("amend")
	if h1.ActiveCommit().HasConflicts {
		t.Fatal("conflict should be solved")
	}
}

func TestCantAmendWithNoChanges(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")

	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	// Amending without changing anything doesnt work
	h1.Run("amend")
	h1.CheckOutContains(nothingToCommit)
	// It does work if you change the message
	h1.Run("amend", "better message")
	if h1.ActiveCommit().Version != 1 {
		t.Fatal("version should increase")
	}
}

func TestForceAmendWithConflict(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")

	// Cause a rebase conflict
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	c0 := h1.ActiveCommit()
	h1.Run("down")
	h1.WriteFile("a.txt", "AAA")
	h1.Run("commit", "AAA")
	h1.Run("rebase", c0.IdString)
	h1.CheckOutContains(gotConflict)

	// Force amend without any message change is not allowed
	h1.Run("amend", "--yolo")
	h1.CheckOutContains(nothingToCommit)

	// You can force (ignore conflicts) as long as a message is provided
	h1.Run("amend", "force marking as non-conflict", "--yolo")
	if h1.ActiveCommit().HasConflicts {
		t.Fatal("conflict was solved")
	}
}

func TestSimpleRebaseUpdatesChildren(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")

	// Create the following tree:
	//                 c3(c.txt=ccc)
	//                 |
	// c1(a.txt=aaa)   c2(b.txt=bbb)
	// |              /
	// root-----------
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	h1.Run("down")
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "bbb")
	h1.WriteFile("c.txt", "ccc")
	h1.Run("commit", "ccc")

	// Rebase 2 into 1. 3 should auto rebase cleanly.
	// Expected (* represents obsolete):
	//
	// c3(c.txt=ccc)
	// |
	// c2(b.txt=bbb)   c3*(c.txt=ccc)
	// |               |
	// c1(a.txt=aaa)   c2*(b.txt=bbb)
	// |              /
	// root-----------

	h1.Run("rebase", "2", "1")
	h1.CheckOutContains(rebaseOk)
	h1.CheckLogAllVersions(
		IdVersionAndConflict{3, 1, false},
		IdVersionAndConflict{3, 0, false},
		IdVersionAndConflict{2, 1, false},
		IdVersionAndConflict{2, 0, false},
		IdVersionAndConflict{1, 0, false},
		IdVersionAndConflict{0, 0, false},
	)
}

func TestAmendCausesConflictOnChild(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")

	// Create the following tree:
	// c4 (a.txt=AAAAAA)
	// |
	// c3(c2+b.txt=bbb)
	// |
	// c2(a.txt=AAA)
	// |
	// c1(a.txt=aaa)
	// |
	// root
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	h1.WriteFile("a.txt", "AAA")
	h1.Run("commit", "AAA")
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "bbb")
	h1.WriteFile("a.txt", "AAAAAA")
	h1.Run("commit", "AAAAAA")

	// Amend c1 and cause a conflict in c2
	h1.Run("goto", "1")
	h1.WriteFile("a.txt", "aaaAAA")
	h1.Run("amend", "amend c1")

	// What we expect (* is obsolete):
	// c4 (a.txt=AAAAAA)
	// |
	// c3(c2+b.txt=bbb)
	// |
	// c2*(a.txt=AAA)  c2(conflicts)
	// |               |
	// c1*(a.txt=aaa)  c1(a.txt=aaaAAA)
	// |               |
	// root -----------/
	h1.CheckLogAllVersions(
		IdVersionAndConflict{4, 0, false},
		IdVersionAndConflict{3, 0, false},
		IdVersionAndConflict{2, 1, true},
		IdVersionAndConflict{2, 0, false},
		IdVersionAndConflict{1, 1, false},
		IdVersionAndConflict{1, 0, false},
		IdVersionAndConflict{0, 0, false},
	)

	// Go to c2 and fix the conflict
	h1.Run("goto", "2")
	h1.Run("status")
	h1.CheckOutContains(" unresolved") // should show unresolved conflicts
	h1.WriteFile("a.txt", "aaaAAABBB")
	h1.Run("status")
	h1.CheckOutContains(" resolved") // should show resolved conflicts
	h1.Run("amend", "amend c2")
	// What we expect (* is obsolete):
	// c4 (a.txt=AAAAAA)
	// |
	// c3(c2+b.txt=bbb)
	// |
	// c2*(a.txt=AAA)  c2*(conflicts)    c2(a.txt=aaaAAAABBB)
	// |               |                 |
	// c1*(a.txt=aaa)  c1(a.txt=aaaAAA)--/
	// |               |
	// root -----------/
	h1.CheckLogAllVersions(
		IdVersionAndConflict{4, 0, false},
		IdVersionAndConflict{3, 0, false},
		IdVersionAndConflict{2, 2, false},
		IdVersionAndConflict{2, 1, true},
		IdVersionAndConflict{2, 0, false},
		IdVersionAndConflict{1, 1, false},
		IdVersionAndConflict{1, 0, false},
		IdVersionAndConflict{0, 0, false},
	)

	// Then rebase c3 into the new c2
	// TODO: we could do this automatcially in the future. Just need to add
	// a rebase state and keep rebasing.
	h1.Run("rebase", "3", "2")
	// What we expect (* is obsolete):
	// c4* (a.txt=AAAAAA)                c4(conflict)
	// |                                 |
	// c3*(c2+b.txt=bbb)                 c3(c2+b.txt=bbb)
	// |                                 |
	// c2*(a.txt=AAA)  c2*(conflicts)    c2(a.txt=aaaAAAABBB)
	// |               |                 |
	// c1*(a.txt=aaa)  c1(a.txt=aaaAAA)--/
	// |               |
	// root -----------/
	h1.CheckLogAllVersions(
		IdVersionAndConflict{4, 1, true},
		IdVersionAndConflict{4, 0, false},
		IdVersionAndConflict{3, 1, false},
		IdVersionAndConflict{3, 0, false},
		IdVersionAndConflict{2, 2, false},
		IdVersionAndConflict{2, 1, true},
		IdVersionAndConflict{2, 0, false},
		IdVersionAndConflict{1, 1, false},
		IdVersionAndConflict{1, 0, false},
		IdVersionAndConflict{0, 0, false},
	)
	// Finally, fix the conflict on c4
	h1.Run("goto", "4")
	h1.WriteFile("a.txt", "aaaAAAAAAAAA")
	h1.Run("amend", "amend c4")
	h1.CheckLogAllVersions(
		IdVersionAndConflict{4, 2, false},
		IdVersionAndConflict{4, 1, true},
		IdVersionAndConflict{4, 0, false},
		IdVersionAndConflict{3, 1, false},
		IdVersionAndConflict{3, 0, false},
		IdVersionAndConflict{2, 2, false},
		IdVersionAndConflict{2, 1, true},
		IdVersionAndConflict{2, 0, false},
		IdVersionAndConflict{1, 1, false},
		IdVersionAndConflict{1, 0, false},
		IdVersionAndConflict{0, 0, false},
	)

}

func TestConflictMarkers(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")

	// Create c1 and c2. Amend c1 to cause conflict on c2:
	//
	// c2_v0* c2_v1
	// |      |
	// c1_v0* c1_v1
	// |      |
	// root---/
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "c1")
	h1.WriteFile("a.txt", "AAA")
	h1.Run("commit", "c2")
	h1.Run("down")
	h1.WriteFile("a.txt", "BBB")
	h1.Run("amend", "amend to cause conflict on c2")
	h1.Run("up")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:           2,
		Version:      1,
		HasConflicts: true,
	})
	h1.CheckFileLine("a.txt", 0, "<<<<<<<<< #2v0")
	h1.CheckFileLine("a.txt", 1, "AAA")
	h1.CheckFileLine("a.txt", 2, "=========")
	h1.CheckFileLine("a.txt", 3, "BBB")
	h1.CheckFileLine("a.txt", 4, ">>>>>>>>> #1v1")
}

func TestAmendChildOfObsoleteAndComplexRebase(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")

	// Create the following tree:
	// c3(b.txt=bbb)
	// |
	// c2(a.txt=AAA)
	// |
	// c1(a.txt=aaa)
	// |
	// root
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	h1.WriteFile("a.txt", "AAA")
	h1.Run("commit", "AAA")
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "bbb")

	// Amend c1 and cause a conflict in c2
	h1.Run("goto", "1")
	h1.WriteFile("a.txt", "aaaAAA")
	h1.Run("amend", "amend c1")
	// What we expect: (* are obsolete)
	// c3(b.txt=bbb)
	// |
	// c2*(a.txt=AAA) c2(conflict)
	// |              |
	// c1*(a.txt=aaa) c1(a.txt=aaaAAA)
	// |              |
	// root-----------/
	h1.CheckLogAllVersions(
		IdVersionAndConflict{3, 0, false},
		IdVersionAndConflict{2, 1, true},
		IdVersionAndConflict{2, 0, false},
		IdVersionAndConflict{1, 1, false},
		IdVersionAndConflict{1, 0, false},
		IdVersionAndConflict{0, 0, false},
	)

	// Ammending child of obsolete is fine
	h1.Run("goto", "3")
	h1.WriteFile("b.txt", "BBB")
	h1.Run("amend", "amend to BBB")
	// What we expect: (* are obsolete)
	//
	// c3*(b.txt=bbb)  c3(b.txt=BBB)
	// |               |
	// c2*(a.txt=AAA)--/   c2(conflict)
	// |                   |
	// c1*(a.txt=aaa)      c1(a.txt=aaaAAA)
	// |                   |
	// root----------------/
	h1.CheckLogAllVersions(
		IdVersionAndConflict{3, 1, false},
		IdVersionAndConflict{3, 0, false},
		IdVersionAndConflict{2, 1, true},
		IdVersionAndConflict{2, 0, false},
		IdVersionAndConflict{1, 1, false},
		IdVersionAndConflict{1, 0, false},
		IdVersionAndConflict{0, 0, false},
	)

	// We can't rebase c3 into c2 still because c2 is in conflict
	h1.Run("rebase", "2")
	h1.CheckOutContains(cantRebaseIntoConflicts)

	// Lets first amend c2 to then rebase
	h1.Run("goto", "2", "-f")
	h1.WriteFile("a.txt", "aaaAAABBB")
	h1.Run("amend", "solve conflict")
	// Now rebase
	h1.Run("goto", "3")
	h1.Run("rebase", "2")
	h1.CheckOutContains(rebaseOk)
	// What we expect: (* are obsolete)
	//
	// c3*(b.txt=bbb)  c3*(b.txt=BBB)        c3(b.txt=BBB)
	// |               |                     |
	// c2*(a.txt=AAA)--/   c2*(conflict)     c2(a.txt=aaaAAABBB)
	// |                   |                 |
	// c1*(a.txt=aaa)      c1(a.txt=aaaAAA)--/
	// |                   |
	// root----------------/
	h1.CheckLogAllVersions(
		IdVersionAndConflict{3, 2, false},
		IdVersionAndConflict{3, 1, false},
		IdVersionAndConflict{3, 0, false},
		IdVersionAndConflict{2, 2, false},
		IdVersionAndConflict{2, 1, true},
		IdVersionAndConflict{2, 0, false},
		IdVersionAndConflict{1, 1, false},
		IdVersionAndConflict{1, 0, false},
		IdVersionAndConflict{0, 0, false},
	)

	// Note that the obsolete ones are hidden because they don't have
	// non-obsolete children
	h1.CheckLog(3, 2, 1, 0)
	commits := h1.Log()
	for _, c := range commits {
		if c.HasConflicts {
			t.Fatal("expected no conflicts")
		}
	}

}

func TestSimpleRebaseDryRun(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")

	// c0: a.txt=aaa
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	c0 := h1.ActiveCommit()

	// c1: b.txt=bbb
	h1.Run("down")
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "bbb")

	// Rebase c1 into c0 in dry run
	h1.Run("rebase", c0.IdString, "-d")
	h1.CheckOutContains(rebaseWillSucceed)
}

func TestRebaseIntoParent(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	root := h1.ActiveCommit()

	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	h1.Run("rebase", root.IdString)
	h1.CheckOutContains(rebaseIntoParent)
	h1.CheckOutDoesntContain(rebaseOk)
}

func TestRebaseIntoSelf(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")

	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	c0 := h1.ActiveCommit()
	h1.Run("rebase", c0.IdString)
	h1.CheckOutContains(rebaseIntoSelf)
	h1.CheckOutDoesntContain(rebaseOk)
}

func TestRebaseRoot(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	root := h1.ActiveCommit()

	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "aaa")
	c0 := h1.ActiveCommit()
	h1.Run("rebase", root.IdString, c0.IdString)
	h1.CheckOutContains(rebaseRoot)
}
func TestGettingCorrectParent(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create this commit tree.
	// @2v0  2v1
	// |      |
	// 1v0   1v1
	// |      |
	// root---/

	// c1:
	h.WriteFile("a.txt", "111")
	h.Run("commit", "c1")
	// c2:
	h.WriteFile("a.txt", "222")
	h.Run("commit", "c2")
	// Amend at c1
	h.Run("down")
	h.WriteFile("b.txt", "aaa")
	h.Run("amend", "aaa")
	// goto c2v0
	h.Run("goto", "2v0")

	// Now we check if we parse correct its parent. expect 1v0
	h.Run("goto", "parent")
	h.CheckActiveCommit(CheckCommitArg{
		Id:             1,
		Version:        0,
		ObsoleteReason: "amend",
	})
}

func TestTopWithTopCommitButWrongVersion(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h.SetServerRootUrl(srv.RootUrl())
	h.Run("server", srv.ServerPath())
	h.Run("key", FakeApiKey)

	// Create this commit tree.
	// @1v0   1v1 submitted
	// |      |
	// root---/

	// c1:
	h.WriteFile("a.txt", "111")
	h.Run("commit", "c1")
	// Push, submit and pull
	h.Run("push")
	srv.Submit(1)
	h.Run("pull")
	h.Run("goto", "1v0")

	// Now we check if command top goes to expected 1v1
	h.Run("top")
	h.CheckOutDoesntContain(alreadyAtTop)
	h.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     1,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    1,
		HasServerV:  true,
		ServerV:     1,
	})
}

func TestSetUnsafeServerUrl(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")
	h.Run("key", FakeApiKey)
	srv := server.NewTestServer(FakeApiKey, t)
	h.Run("enable-unsafe-dev-mode", "true")

	h.Run("unsafe-server", srv.RootUrl()+"/"+srv.ServerPath())

	h.Run("pull")
	h.CheckOutContains(nothingToPull)
}

func TestRandomServerResponse(t *testing.T) {
	h := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h.SetServerRootUrl(srv.RootUrl())
	h.Run("clone", srv.RandomResponsePath(), FakeApiKey)
	h.CheckOutContains("server is not speaking twigg")
}

func TestEnableUnsafeDevMode(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")
	h.Run("key", FakeApiKey)

	h.Run("enable-unsafe-dev-mode")
	h.CheckOutContains(boolNotProvided)

	h.Run("enable-unsafe-dev-mode", "hhhh")
	h.CheckOutContains(invalidBoolValue)

	h.Run("enable-unsafe-dev-mode", "true")
	h.CheckOutContains(setEnableUnsafeDevModeOk(true))

	h.Run("enable-unsafe-dev-mode", "false")
	h.CheckOutContains(setEnableUnsafeDevModeOk(false))
}

func TestPullStay(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	h2 := NewTestHelper2(t)
	h2.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())
	h2.SetServerRootUrl(srv.RootUrl())

	// Client 1 creates one commit and submits it
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "c1")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)
	h1.Run("push")
	srv.Submit(1)

	// Client 2 pulls with --stay
	h2.Run("server", srv.ServerPath())
	h2.Run("key", FakeApiKey)
	h2.Run("pull", "--stay")
	h2.CheckOutContains(pullOk)
	h2.CheckActiveCommitLocalId(0)
}

func TestPullDetachedTop(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	h2 := NewTestHelper2(t)
	h2.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())
	h2.SetServerRootUrl(srv.RootUrl())

	// Client 1 creates two commits and submits them
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "c1")
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "c2")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)
	h1.Run("push")
	srv.Submit(1)
	srv.Submit(2)

	// Client 2 pulls just the top commit
	h2.Run("server", srv.ServerPath())
	h2.Run("key", FakeApiKey)
	h2.Run("pull", "top")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     0,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     1,
	})
	h2.CheckFile("a.txt", "aaa")
	h2.CheckFile("b.txt", "bbb")
	h2.Run("pull")
	h2.CheckOutContains(nothingToPull)

	// Now client 1 submits another one. Client 2 should get it when pulling
	h1.WriteFile("c.txt", "ccc")
	h1.Run("commit", "c3")
	h1.Run("push")
	srv.Submit(3)
	h2.Run("pull")
	h2.CheckOutContains(pullOk)
	h2.Run("up")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          2,
		Version:     0,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    3,
		HasServerV:  true,
		ServerV:     1,
	})
	h2.CheckFile("c.txt", "ccc")
	h2.Run("down")
	h2.Run("down")
	// Can't go down twice because c1 was not pulled yet
	h2.CheckOutContains(parentNotFound)
	// Same if try to "goto down"
	h2.Run("goto", "down")
	h2.CheckOutContains(parentNotFound)

	h2.Run("goto", "c0")
	h2.Run("up")
	h2.CheckOutContains(childNotFound)
	h2.Run("pull")
	h2.Run("up")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          3,
		Version:     0,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    1,
		HasServerV:  true,
		ServerV:     1,
	})
	h2.Run("up")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     0,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     1,
	})
	h2.Run("up")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          2,
		Version:     0,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    3,
		HasServerV:  true,
		ServerV:     1,
	})
}

func TestDetachedCommitCantBeRebased(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	h2 := NewTestHelper2(t)
	h2.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())
	h2.SetServerRootUrl(srv.RootUrl())

	// Client 1 creates c1 and c2
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "c1")
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "c2")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)
	h1.Run("push")

	// Client 2 creates a dummy commit
	h2.WriteFile("c.txt", "ccc")
	h2.Run("commit", "create c.txt")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:      1,
		Version: 0,
	})

	// Client 2 pulls c2
	h2.Run("server", srv.ServerPath())
	h2.Run("key", FakeApiKey)
	h2.Run("pull", "c2v0")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          2,
		Version:     0,
		IsSubmitted: false,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     0,
	})

	// Client 2 tries to rebase c2 but fails bc parent is not present
	h2.Run("rebase", "c2", "1")
	h2.CheckOutContains(commitIsDetached)
}

func TestPullDetachedCommitByVersion(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	h2 := NewTestHelper2(t)
	h2.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())
	h2.SetServerRootUrl(srv.RootUrl())

	// Client 1 creates two commits and submits them
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "c1")
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "c2")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)
	h1.Run("push")
	srv.Submit(1)
	srv.Submit(2)

	// Client 2 pulls c2v0
	h2.Run("server", srv.ServerPath())
	h2.Run("key", FakeApiKey)
	h2.Run("pull", "c2v0")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     0,
		IsSubmitted: false,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     0,
	})
	// Client 2 then pulls c2 (i.e. latest version)
	h2.Run("pull", "c2")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     1,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     1,
	})
	h2.Run("goto", "c0")
	h2.Run("pull")
	h2.CheckLog(0, 2, 1)
}

func TestPullDetachedAmendAndPush(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	h2 := NewTestHelper2(t)
	h2.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())
	h2.SetServerRootUrl(srv.RootUrl())

	// Client 1 creates two commits and pushes them
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "c1")
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "c2")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)
	h1.Run("push")
	// Client 1:
	//
	// #2v0-c2v0
	// |
	// #1v0-c1v0
	// |
	// root

	// Client 2 pulls c2, amends it and pushes
	h2.Run("server", srv.ServerPath())
	h2.Run("key", FakeApiKey)
	h2.Run("pull", "c2")
	// Client 2:
	//
	// #1v0-c2v0
	// |
	// ~
	h2.CheckFile("a.txt", "aaa")
	h2.CheckFile("b.txt", "bbb")
	h2.WriteFile("a.txt", "AAA")
	h2.Run("amend")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     1,
		IsSubmitted: false,
		HasServerId: true,
		ServerId:    2,
	})
	h2.Run("push")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     1,
		IsSubmitted: false,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     1,
	})
	// Client 2:
	//
	// #1v0-c2v0*  #1v1-c2v1
	// |           |
	// ~           ~

	// Submit both
	srv.Submit(1)
	srv.Submit(2)
	// Both clients pull
	h1.Run("pull")
	// Client 1:
	//
	// #2v0-c2v0*  #2v1-c2v2
	// |           |
	// #1v0-c1v0*  #1v1-c1v1
	// |           |
	// root--------/
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          2,
		Version:     1,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     2,
	})
	h1.CheckLogAll(0, 1, 1, 2, 2)
	h2.Run("pull")
	// Pull will always pull everything after the last submitted one
	// Client 2:
	//
	// #1v0-c2v0*  #1v1-c2v1*
	// |           |
	// ~           ~
	//
	// #1v2-c2v2
	// |
	// #2v0-c1v1
	// |
	// root
	h2.CheckLogAll(0, 2, 1)
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     2,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     2,
	})
	h2.Run("down")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          2,
		Version:     0,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    1,
		HasServerV:  true,
		ServerV:     1,
	})
	h2.Run("down")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          0,
		Version:     0,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    0,
		HasServerV:  true,
		ServerV:     0,
	})
	// Nothing else to pull
	h2.Run("pull")
	h2.CheckOutContains(nothingToPull)
}

func TestAmendAndPullDetachedNewVersion(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	h2 := NewTestHelper2(t)
	h2.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())
	h2.SetServerRootUrl(srv.RootUrl())

	// Client 1 creates 3 commits and pushes them
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "c1")
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "c2")
	h1.WriteFile("c.txt", "ccc")
	h1.Run("commit", "c3")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)
	h1.Run("push")
	// Client 1:
	// #3v0-c3v0
	// |
	// #2v0-c2v0
	// |
	// #1v0-c1v0
	// |
	// root

	// Client 2 pulls c3, amends it and pushes
	h2.Run("server", srv.ServerPath())
	h2.Run("key", FakeApiKey)
	h2.Run("pull", "c3")
	// Client 2:
	//
	// #3v0-c3v0
	// |
	// ~
	h2.CheckFile("a.txt", "aaa")
	h2.CheckFile("b.txt", "bbb")
	h2.WriteFile("c.txt", "CCC")
	h2.Run("amend")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     1,
		IsSubmitted: false,
		HasServerId: true,
		ServerId:    3,
	})
	h2.Run("push")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     1,
		IsSubmitted: false,
		HasServerId: true,
		ServerId:    3,
		HasServerV:  true,
		ServerV:     1,
	})

	// Client 1 now amends a base commit:
	h1.Run("goto", "1")
	h1.WriteFile("a.txt", "aaaaaaa")
	h1.Run("amend", "c1v1")
	// Client 1:
	// #3v0-c3v0*  #3v1
	// |           |
	// #2v0-c2v0*  #2v1
	// |           |
	// #1v0-c1v0*  #1v1
	// |           |
	// root--------/

	// Client 1 now pulls the latest commit 3:
	// #3v0-c3v0* #3v2-c3v1    #3v1*
	// |         /             |
	// #2v0-c2v0*              #2v1
	// |                       |
	// #1v0-c1v0*              #1v1
	// |                       |
	// root--------------------/

	// This will just make the current one obsolete
	h1.LogAll()
	h1.Run("pull", "c3")
	h1.CheckLogAllVersions(
		IdVersionAndConflict{Id: 0, Version: 0},
		IdVersionAndConflict{Id: 1, Version: 0},
		IdVersionAndConflict{Id: 1, Version: 1},
		IdVersionAndConflict{Id: 2, Version: 0},
		IdVersionAndConflict{Id: 2, Version: 1},
		IdVersionAndConflict{Id: 3, Version: 0},
		IdVersionAndConflict{Id: 3, Version: 1},
		IdVersionAndConflict{Id: 3, Version: 2},
	)
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          3,
		Version:     2,
		HasServerId: true,
		ServerId:    3,
		HasServerV:  true,
		ServerV:     1,
	})
	h1.Run("down")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:             2,
		Version:        0,
		HasServerId:    true,
		ServerId:       2,
		HasServerV:     true,
		ServerV:        0,
		ObsoleteReason: "auto-rebase",
	})
	h1.Run("down")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:             1,
		Version:        0,
		HasServerId:    true,
		ServerId:       1,
		HasServerV:     true,
		ServerV:        0,
		ObsoleteReason: "amend",
	})
}

func TestPullParentOfDetached(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	h2 := NewTestHelper2(t)
	h2.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())
	h2.SetServerRootUrl(srv.RootUrl())
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)
	h2.Run("server", srv.ServerPath())
	h2.Run("key", FakeApiKey)

	// Client 1 creates two commits, pushes and submits them
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "c1")
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "c2")
	h1.Run("push")
	srv.Submit(1)
	srv.Submit(2)

	// Client 2 pulls them, so neither client is at the root
	h2.Run("pull")
	h2.CheckLog(0, 1, 2)
	h2.Run("top")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          2,
		Version:     0,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     1,
	})

	// Client 1 creates and pushes 4 more commits
	h1.WriteFile("c.txt", "ccc")
	h1.Run("commit", "c3")
	h1.WriteFile("d.txt", "ddd")
	h1.Run("commit", "c4")
	h1.WriteFile("e.txt", "eee")
	h1.Run("commit", "c5")
	h1.WriteFile("f.txt", "fff")
	h1.Run("commit", "c6")
	h1.Run("push")

	// Client 2 pulls c5, which is far from its current commit (c2)
	h2.Run("pull", "c5")
	h2.CheckOutContains(pullOk)
	// Client 2:
	//
	// #3v0-c5v0
	// |
	// ~
	//
	// #2v0-c2v1
	// |
	// #1v0-c1v1
	// |
	// root
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          3,
		Version:     0,
		IsSubmitted: false,
		HasServerId: true,
		ServerId:    5,
		HasServerV:  true,
		ServerV:     0,
	})
	h2.CheckFile("a.txt", "aaa")
	h2.CheckFile("b.txt", "bbb")
	h2.CheckFile("c.txt", "ccc")
	h2.CheckFile("d.txt", "ddd")
	h2.CheckFile("e.txt", "eee")
	h2.CheckHasNoFile("f.txt")

	// Client 2 now pulls the parent of the detached commit. It must attach to
	// the child that is already there
	h2.Run("pull", "c4")
	h2.CheckOutContains(pullOk)
	// Client 2:
	//
	// #3v0-c5v0
	// |
	// #4v0-c4v0
	// |
	// ~
	//
	// #2v0-c2v1
	// |
	// #1v0-c1v1
	// |
	// root
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          4,
		Version:     0,
		IsSubmitted: false,
		HasServerId: true,
		ServerId:    4,
		HasServerV:  true,
		ServerV:     0,
	})
	h2.CheckFile("a.txt", "aaa")
	h2.CheckFile("b.txt", "bbb")
	h2.CheckFile("c.txt", "ccc")
	h2.CheckFile("d.txt", "ddd")
	h2.CheckHasNoFile("e.txt")
	h2.CheckHasNoFile("f.txt")

	// The child is now attached to the pulled parent
	h2.CheckLog(3, 4)

	// Client 2 can now walk between the two detached commits
	h2.Run("up")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          3,
		Version:     0,
		IsSubmitted: false,
		HasServerId: true,
		ServerId:    5,
		HasServerV:  true,
		ServerV:     0,
	})
	h2.CheckFile("e.txt", "eee")
	h2.Run("down")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          4,
		Version:     0,
		IsSubmitted: false,
		HasServerId: true,
		ServerId:    4,
		HasServerV:  true,
		ServerV:     0,
	})
	h2.CheckHasNoFile("e.txt")
	// c3 was never pulled, so c4 is still detached
	h2.Run("down")
	h2.CheckOutContains(parentNotFound)
}

func TestDiffOfDetachedInstructsToPullParent(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	h2 := NewTestHelper2(t)
	h2.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())
	h2.SetServerRootUrl(srv.RootUrl())
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)
	h2.Run("server", srv.ServerPath())
	h2.Run("key", FakeApiKey)

	// Client 1 creates two commits, pushes and submits them
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "c1")
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "c2")
	h1.Run("push")
	srv.Submit(1)
	srv.Submit(2)

	// Client 2 pulls the second detached
	h2.Run("pull", "c2")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     0,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     1,
	})
	// Running diff instructs to pull the parent
	h2.Run("diff")
	h2.CheckOutContains(instructToPullParent(commit.Commit{ParentServerL: 1, ParentServerV: 1}))

	h2.Run("pull", "c1v1", "--stay")
	h2.Run("diff", "--all")
	h2.CheckOutContains("bbb")
}

func TestPlainPullAttachesDetachedCommit(t *testing.T) {
	h1 := NewTestHelper(t)
	h1.Run("init")
	h2 := NewTestHelper2(t)
	h2.Run("init")
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())
	h2.SetServerRootUrl(srv.RootUrl())
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)
	h2.Run("server", srv.ServerPath())
	h2.Run("key", FakeApiKey)

	// Client 1 creates two commits, pushes and submits them
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "c1")
	h1.WriteFile("b.txt", "bbb")
	h1.Run("commit", "c2")
	h1.Run("push")
	srv.Submit(1)
	srv.Submit(2)

	// Client 2 pulls them
	h2.Run("pull")
	h2.Run("top")

	// Client 1 creates 3 more commits, pushes and submits them
	h1.WriteFile("c.txt", "ccc")
	h1.Run("commit", "c3")
	h1.WriteFile("d.txt", "ddd")
	h1.Run("commit", "c4")
	h1.WriteFile("e.txt", "eee")
	h1.Run("commit", "c5")
	h1.Run("push")
	srv.Submit(3)
	srv.Submit(4)
	srv.Submit(5)

	// Client 2 pulls c5, which is far from its current commit (c2)
	h2.Run("pull", "c5")
	h2.CheckActiveCommit(CheckCommitArg{
		Id:          3,
		Version:     0,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    5,
		HasServerV:  true,
		ServerV:     1,
	})
	h2.CheckLog(3)

	// A plain pull from c2 brings c3 and c4.
	// Client 2 must end up with c5 now attached.
	h2.Run("goto", "2")
	h2.Run("pull")
	h2.CheckOutContains(pullOk)
	h2.CheckLog(0, 1, 2, 3, 4, 5)
	// #3v0-c5v1
	// |
	// #5v0-c4v1
	// |
	// #4v0-c3v1
	// |
	// #2v0-c2v1
	// |
	// #1v0-c1v1
	// |
	// root

	// Switch to c5 and walk down checking all files
	h2.Run("goto", "c5")
	h2.CheckFile("e.txt", "eee")
	// c4
	h2.Run("down")
	h2.CheckHasNoFile("e.txt")
	h2.CheckFile("d.txt", "ddd")
	// c3
	h2.Run("down")
	h2.CheckHasNoFile("e.txt")
	h2.CheckHasNoFile("d.txt")
	h2.CheckFile("c.txt", "ccc")
	// c2
	h2.Run("down")
	h2.CheckHasNoFile("e.txt")
	h2.CheckHasNoFile("d.txt")
	h2.CheckHasNoFile("c.txt")
	h2.CheckFile("b.txt", "bbb")
	// c1
	h2.Run("down")
	h2.CheckHasNoFile("e.txt")
	h2.CheckHasNoFile("d.txt")
	h2.CheckHasNoFile("c.txt")
	h2.CheckHasNoFile("b.txt")
	h2.CheckFile("a.txt", "aaa")
	// root
	h2.Run("down")
	h2.CheckHasNoFile("e.txt")
	h2.CheckHasNoFile("d.txt")
	h2.CheckHasNoFile("c.txt")
	h2.CheckHasNoFile("b.txt")
	h2.CheckHasNoFile("a.txt")
}

func TestSimpleRollback(t *testing.T) {
	h1 := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())
	h1.Run("init")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)

	// Create a commit, push, submit
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "c1")
	h1.Run("push")
	srv.Submit(1)

	// Create a rollback commit and submit it
	const authorId = 88
	srv.CreateRollback(1, authorId)
	srv.Submit(2)

	// Pull to check the tree:
	// c2 (- a.txt)
	// |
	// c1 (+ a.txt)
	// |
	// root
	h1.Run("pull")
	h1.CheckLog(0, 1, 2)
	h1.Run("top")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          2,
		Version:     0,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    2,
		HasServerV:  true,
		ServerV:     1,
	})
	h1.CheckHasNoFile("a.txt")
	h1.Run("down")
	h1.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     1,
		IsSubmitted: true,
		HasServerId: true,
		ServerId:    1,
		HasServerV:  true,
		ServerV:     1,
	})
	h1.CheckFile("a.txt", "aaa")
	h1.Run("down")
	h1.CheckHasNoFile("a.txt")
	if h1.ActiveCommit().Id != 0 {
		t.Fatal("expected to be at root")
	}
}

func TestRollbackTwice(t *testing.T) {
	h1 := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())
	h1.Run("init")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)

	// Create a commit, push, submit
	h1.WriteFile("a.txt", "aaa")
	h1.Run("commit", "c1")
	h1.Run("push")
	srv.Submit(1)

	// Create a rollback commit and submit it
	const authorId = 88
	srv.CreateRollback(1, authorId)
	srv.Submit(2)
	// Do this again
	srv.CreateRollback(2, authorId)
	srv.Submit(3)

	// Pull to check the tree:
	// c3 (+a.txt)
	// |
	// c2 (-a.txt)
	// |
	// c1 (+a.txt)
	// |
	// root
	h1.Run("pull")
	h1.CheckLog(0, 1, 2, 3)
	h1.Run("top")
	h1.CheckFile("a.txt", "aaa")
	h1.Run("down")
	h1.CheckHasNoFile("a.txt")
	h1.Run("down")
	h1.CheckFile("a.txt", "aaa")
	h1.Run("down")
	h1.CheckHasNoFile("a.txt")
}

func TestOldClientProtocol(t *testing.T) {
	h1 := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h1.SetServerRootUrl(srv.RootUrl())
	h1.Run("init")
	h1.Run("server", srv.ServerPath())
	h1.Run("key", FakeApiKey)

	xchange.UseMockProtocolVersionSentByWriters = true
	xchange.MockProtocolVersionSentByWriters = math.MaxUint8
	t.Cleanup(func() { xchange.UseMockProtocolVersionSentByWriters = false })
	h1.Run("pull")
	h1.CheckOutContains(updateRequired)
}

func TestCiList(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")
	h.Run("status")
	h.Run("ci-list")
	h.CheckOutContains(noCiWillRun)

	// Create a commit that creates CI.json, subfolder/CI.json and subfolder2/CI.json
	h.WriteFile("CI.json", "fake ci file")
	h.WriteFile("subfolder/CI.json", "another fake ci file")
	h.WriteFile("subfolder2/CI.json", "yet another fake ci file")
	h.Run("commit", "c1")
	h.Run("ci-list")
	h.CheckOutContains("CI.json")
	h.CheckOutContains("subfolder/CI.json")
	h.Run("cil", "1v0")
	h.CheckOutContains("CI.json")
	h.CheckOutContains("subfolder/CI.json")

	// Create another commit that deletes "subfolder2/CI.json" and modifies subfolder
	h.DeleteFile("subfolder2/CI.json")
	h.WriteFile("subfolder/a.txt", "aaa")
	h.Run("commit", "c2")
	h.Run("cil")
	h.CheckOutContains("CI.json")
	h.CheckOutContains("subfolder/CI.json")
	h.CheckOutDoesntContain("subfolder2/CI.json")
}

func TestGotoUpAndDown(t *testing.T) {
	h := NewTestHelper(t)
	h.Run("init")

	// Create this commit tree.
	// @c2
	// |
	// c1
	// |
	// root
	h.WriteFile("c1.txt", "c1")
	h.Run("commit", "c1")
	h.WriteFile("c2.txt", "c2")
	h.Run("commit", "c2")

	h.Run("goto", "up")
	h.CheckOutContains(childNotFound)
	h.CheckActiveCommit(CheckCommitArg{
		Id:      2,
		Version: 0,
	})

	h.Run("goto", "down")
	h.CheckActiveCommit(CheckCommitArg{
		Id:      1,
		Version: 0,
	})

	h.Run("goto", "up")
	h.CheckActiveCommit(CheckCommitArg{
		Id:      2,
		Version: 0,
	})
}

func TestPullRenamedCommit(t *testing.T) {
	h := NewTestHelper(t)
	srv := server.NewTestServer(FakeApiKey, t)
	h.SetServerRootUrl(srv.RootUrl())
	h.Run("init")
	h.Run("server", srv.ServerPath())
	h.Run("key", FakeApiKey)

	// Push a commit
	h.WriteFile("a.txt", "aaa")
	h.Run("commit", "original message")
	h.Run("push")
	h.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     0,
		HasServerId: true,
		ServerId:    1,
		HasServerV:  true,
		ServerV:     0,
	})

	// Server renames the commit
	srv.RenameCommit(1, "renamed message", 42)

	// Client pulls the latest version of c/1
	h.Run("pull", "c/1")
	h.CheckOutContains(pullOk)
	h.CheckActiveCommit(CheckCommitArg{
		Id:          1,
		Version:     1,
		HasServerId: true,
		ServerId:    1,
		HasServerV:  true,
		ServerV:     1,
	})

	// Log shows the renamed message; files are unchanged
	h.Log()
	h.CheckOutContains("renamed message")
	h.CheckFile("a.txt", "aaa")
}
