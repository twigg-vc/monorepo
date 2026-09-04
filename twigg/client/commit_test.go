package client

import (
	"errors"
	"monorepo/twigg/commit"
	"reflect"
	"testing"
)

func TestSimpleCommit(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)
	wd.WriteFile("a.txt", "hello")

	// Write a commit on top of root with a single file and check
	// the values of the struct created
	c, err := ag.Commit(wd, "first commit", &root, l)
	if err != nil {
		t.Fatal(err)
	}
	if c.Birth != commit.BirthReasonCommit {
		t.Fatal("wrong birth reason")
	}
	if c.L != 1 {
		t.Fatal("first commit should have N=1")
	}
	if c.Version != 0 {
		t.Fatal("commits should start with v0")
	}
	if c.TreeVersion != 1 {
		t.Fatal("should be the tree version 1")
	}
	if c.RootDirHash != wd.RootDirHash() {
		t.Fatal("wrong root hash")
	}
	if c.ParentL != root.L {
		t.Fatal("wrong parent L")
	}
	if c.Status != commit.StatusLatest {
		t.Fatal("commits should start up to date")
	}
	if c.ObsReason != commit.ObsoleteReasonNone {
		t.Fatal("expected none obsolete reason")
	}
	if !c.HasDiffData {
		t.Fatal("expected to have diff data")
	}
	if c.DiffDataLinesCreated != 1 ||
		c.DiffDataLinesDeleted != 0 ||
		c.DiffDataLinesModified != 0 ||
		c.DiffDataFilesCreated != 1 ||
		c.DiffDataFilesDeleted != 0 ||
		c.DiffDataFilesModified != 0 {
		t.Fatal("bad diff data")
	}
	if len(root.Children) != 1 || len(root.ChildrenVersions) != 1 ||
		root.Children[0] != c.L || root.ChildrenVersions[0] != 0 {
		t.Fatal("wrong root children")
	}
	// read root again and check persistence
	root, err = ag.GetLatest(0, l)
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Children) != 1 || len(root.ChildrenVersions) != 1 ||
		root.Children[0] != c.L || root.ChildrenVersions[0] != 0 {
		t.Fatal("wrong root children")
	}
}

func TestNothingToAmend(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)

	// Create a commit with a.txt=hello
	wd.WriteFile("a.txt", "hello")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	// Try amending without changing anything in the workdir nor the message
	_, err := ag.Amend(&c1, false, wd, "c1", l)
	if !errors.Is(err, ErrNothingToCommit) {
		t.Fatal("expected nothing to commit")
	}
	// Changing the message is ok
	_, err = ag.Amend(&c1, false, wd, "new message", l)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAmendAndBecomeLikeParent(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)

	// c1: a.txt=hello
	wd.WriteFile("a.txt", "hello")
	c1, _ := ag.Commit(wd, "a.txt=hello", &root, l)
	// c2_v0: a.txt=bye
	wd.WriteFile("a.txt", "bye")
	c2, _ := ag.Commit(wd, "c2", &c1, l)

	// Now try amending c2 to make it just like the parent. This should be ok.
	wd.WriteFile("a.txt", "hello")
	_, err := ag.Amend(&c2, false, wd, "a.txt=hello", l)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSimpleAmend(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)

	// Create a commit with a.txt=hello
	wd.WriteFile("a.txt", "hello")
	c1, _ := ag.Commit(wd, "c1", &root, l)

	// Amend the first commit without rebasing children
	wd.WriteFile("a.txt", "Hello, world!")
	c1New, err := ag.Amend(&c1, false, wd, "amend c1", l)
	if err != nil {
		t.Fatal(err)
	}
	// Version must have increased
	if c1New.Version != 1 {
		t.Fatal("wrong version after amend")
	}
	// Original commit becomes obsolete
	if c1.Status != commit.StatusObsolete {
		t.Fatal("c1 must become obsolete")
	}
	if c1.ObsReason != commit.ObsoleteReasonAmend {
		t.Fatal("c1 must show obsolete reason to be an amend")
	}
	// Version of original commit doesnt change
	if c1.Version != 0 {
		t.Fatal("c1 version should not change")
	}
	if c1New.Status != commit.StatusLatest {
		t.Fatal("c1New must be up to date")
	}
	if c1New.ParentL != 0 {
		t.Fatal("wrong parent")
	}
	if wd.RootDirHash() != c1New.RootDirHash {
		t.Fatal("wrong root hash")
	}
	if !c1New.HasDiffData {
		t.Fatal("expected to have diff data")
	}
	if c1New.DiffDataLinesCreated != 1 ||
		c1New.DiffDataLinesDeleted != 0 ||
		c1New.DiffDataLinesModified != 0 ||
		c1New.DiffDataFilesCreated != 1 ||
		c1New.DiffDataFilesDeleted != 0 ||
		c1New.DiffDataFilesModified != 0 {
		t.Fatal("bad diff data")
	}

	// Root should contain both versions as children
	root, _ = ag.GetLatest(root.L, l)
	if len(root.Children) != 2 ||
		len(root.ChildrenVersions) != 2 ||
		root.Children[0] != c1.L ||
		root.ChildrenVersions[0] != c1.Version ||
		root.Children[1] != c1New.L ||
		root.ChildrenVersions[1] != c1New.Version {
		t.Fatal("wrong root")
	}
}

func TestCantAmmendObsolete(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)

	// Create a commit and amend it
	wd.WriteFile("a.txt", "hello")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	wd.WriteFile("a.txt", "Hello, world!")
	_, err := ag.Amend(&c1, false, wd, "amend c1", l)
	if err != nil {
		t.Fatal(err)
	}
	if c1.Status != commit.StatusObsolete {
		t.Fatal("commit should be obsolete")
	}
	// Try amending again. We expect an error becahse c1 is obsolete
	_, err = ag.Amend(&c1, false, wd, "re-amend c1", l)
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestCantAmmendSubmitted(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)
	if !root.IsSubmitted {
		t.Fatal("root commit should always be submitted")
	}
	// Ammending a submitted commit is not possible
	wd.WriteFile("a.txt", "Hello, world!")
	_, err := ag.Amend(&root, false, wd, "amend c1", l)
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestSimpleNoConflictRebase(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)

	// Create the following tree:
	//
	//                  c3(a.txt=aaa, b.txt=BBB)
	//                  |
	// c1 (a.txt=aaa)   c2(a.txt=aaa, b.txt=bbb)
	// |               /
	// root------------
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "aaa", &root, l)
	wd.WriteFile("b.txt", "bbb")
	c2, _ := ag.Commit(wd, "bbb", &root, l)
	wd.WriteFile("b.txt", "BBB")
	c3, _ := ag.Commit(wd, "BBB", &c2, l)

	// Rebase c2 into c1
	c2New, err := ag.Rebase(&c2, &c1,
		/*isAutoRebaseOfChildren*/ false,
		/*rebaseChildren*/ false, l)
	if err != nil {
		t.Fatal(err)
	}
	if c2.Status != commit.StatusObsolete {
		t.Fatal("c2 should become obsolete")
	}
	if !c1.HasChild(c2New) {
		t.Fatal("c1 should now have the new one as child")
	}
	if c2New.Birth != commit.BirthReasonManualRebase {
		t.Fatal("birth reason of c1 is manual rebase")
	}
	if c2New.HasRebaseConflicts {
		t.Fatal("should have no conflicts")
	}
	if c2New.Version != 1 {
		t.Fatal("created -> v0. rebased -> v1")
	}
	if c2New.Status != commit.StatusLatest {
		t.Fatal("c2 should be up to date")
	}
	if c2New.ObsReason != commit.ObsoleteReasonNone {
		t.Fatal("c2 should have none obs reason")
	}
	if len(c2New.Children) != 0 {
		t.Fatal("rebased commits start out without children")
	}
	if !c2New.HasDiffData {
		t.Fatal("expected to have diff data")
	}
	if c2New.DiffDataLinesCreated != 1 ||
		c2New.DiffDataLinesDeleted != 0 ||
		c2New.DiffDataLinesModified != 0 ||
		c2New.DiffDataFilesCreated != 1 ||
		c2New.DiffDataFilesDeleted != 0 ||
		c2New.DiffDataFilesModified != 0 {
		t.Fatal("bad diff data")
	}

	// Check getting the commit is equivalent
	c2New_, _ := ag.GetLatest(c2.L, l)
	c2New_.CreatedOn = c2New.CreatedOn // time doesnt work well with DeepEqual
	if !reflect.DeepEqual(c2New, c2New_) {
		t.Fatal("c2New unexpected")
	}

	// Root should still have the previous version as child
	root, _ = ag.GetLatest(0, l)
	if !reflect.DeepEqual(root.Children, []commit.LocalId{1, 2}) {
		t.Fatal("wrong root children")
	}
	if !reflect.DeepEqual(root.ChildrenVersions, []uint64{0, 0}) {
		t.Fatal("wrong root children vers")
	}
	// C1 should have the nwe version of c2 as child
	c1, _ = ag.GetLatest(1, l)
	if !reflect.DeepEqual(c1.Children, []commit.LocalId{2}) {
		t.Fatal("wrong c1 children")
	}
	if !reflect.DeepEqual(c1.ChildrenVersions, []uint64{1}) {
		t.Fatal("wrong c1 children vers")
	}
	// First version of c2 should be obsolete but still have the children
	c2, _ = ag.GetVersion(2, 0, l)
	if c2.Status != commit.StatusObsolete {
		t.Fatal("c2 should be obsolete")
	}
	if !reflect.DeepEqual(c2.Children, []commit.LocalId{c3.L}) {
		t.Fatal("wrong c2 children")
	}
	if !reflect.DeepEqual(c2.ChildrenVersions, []commit.LocalId{c3.Version}) {
		t.Fatal("wrong c2 children vers")
	}

	// Load the rebased commit to the workdir to check its contents
	err = ag.Load(c2New.TreeVersion, wd, l)
	if err != nil {
		t.Fatal(err)
	}
	if wd.ReadFile("a.txt") != "aaa" {
		t.Fatal("wrong content of a")
	}
	if wd.ReadFile("b.txt") != "bbb" {
		t.Fatal("wrong content of b")
	}
	if wd.RootDirHash() != c2.RootDirHash {
		t.Fatal("wrong root hash")
	}
}

func TestConflictRebase(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)

	// Create the following tree:
	//
	// c0 (a.txt=aaa)   c1(a.txt=bbb)
	// |               /
	// root------------
	wd.WriteFile("a.txt", "aaa")
	c0, _ := ag.Commit(wd, "aaa", &root, l)
	wd.WriteFile("a.txt", "bbb")
	c1, _ := ag.Commit(wd, "bbb", &root, l)

	// Rebase c1 into c0
	c1New, err := ag.Rebase(&c1, &c0, false, false, l)
	if err != nil {
		t.Fatal(err)
	}
	// Check that conflicts happened
	if !c1New.HasRebaseConflicts {
		t.Fatal("should haveconflicts")
	}
	if !c1New.HasDiffData {
		t.Fatal("expected to have diff data")
	}
	// Given that there will be conflict markers we only roughly check
	// that there will be one modified file with some created and some modified lines
	if c1New.DiffDataLinesCreated == 0 ||
		c1New.DiffDataLinesDeleted != 0 ||
		c1New.DiffDataLinesModified == 0 ||
		c1New.DiffDataFilesCreated != 0 ||
		c1New.DiffDataFilesDeleted != 0 ||
		c1New.DiffDataFilesModified != 1 {
		t.Fatal("bad diff data")
	}
}

func TestSimpleRebaseDown(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)

	// Create the following tree:
	//
	// c1(a.txt=aa, b.txt=bbb)
	// |
	// c0 (a.txt=aa)
	// |
	// root
	wd.WriteFile("a.txt", "aa")
	c0, _ := ag.Commit(wd, "aa", &root, l)
	if !root.HasChild(c0) {
		t.Fatal("root should have c0 child")
	}
	wd.WriteFile("b.txt", "bbb")
	c1, _ := ag.Commit(wd, "bbb", &c0, l)

	// Rebase c1 into root
	c1New, err := ag.Rebase(&c1, &root, false, false, l)
	if err != nil {
		t.Fatal(err)
	}
	if !root.HasChild(c1New) {
		t.Fatal("should have child")
	}
	// Check that conflicts didn't happen
	if c1New.HasRebaseConflicts {
		t.Fatal("should not have conflicts")
	}
}

func TestRebaseChildren(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)

	// Create the following tree:
	//                          c5(a.txt=bye, c.txt=cc, d.txt=dd)
	//                          |
	// c3(a.txt=BYE, b.txt=bb)  c4(a.txt=bye, c.txt=cc)
	// |                        |
	// c2(a.txt=bye)------------/
	// |
	// c1(a.txt=hello)
	// |
	// root
	wd.WriteFile("a.txt", "hello")
	c1_v0, _ := ag.Commit(wd, "c1", &root, l)
	wd.WriteFile("a.txt", "bye")
	c2_v0, _ := ag.Commit(wd, "c2", &c1_v0, l)
	wd.WriteFile("a.txt", "BYE")
	wd.WriteFile("b.txt", "bb")
	c3_v0, _ := ag.Commit(wd, "c3", &c2_v0, l)
	wd.WriteFile("a.txt", "bye")
	wd.Delete("b.txt")
	wd.WriteFile("c.txt", "cc")
	c4_v0, _ := ag.Commit(wd, "c4", &c2_v0, l)
	wd.WriteFile("d.txt", "dd")
	ag.Commit(wd, "c5", &c4_v0, l)

	// Amend c1 so that a conflict happens in c2.
	// Use `rebaseChildren` so that they're rebased
	//                          c5(a.txt=bye, c.txt=cc, d.txt=dd)
	//                          |
	// c3(a.txt=BYE, b.txt=bb)  c4(a.txt=bye, c.txt=cc)
	// |                        |
	// c2(a.txt=bye)OBSOLETE----/
	// |
	// c1(a.txt=hello)OBSOLETE
	// |
	// |                           c2_v1(conflict in a.txt)
	// |                           |
	// |                           c1_v1(a.txt=HELLO)
	// root------------------------/
	wd.Delete("b.txt")
	wd.Delete("c.txt")
	wd.WriteFile("a.txt", "HELLO")
	c1_v1, err := ag.Amend(&c1_v0, true, wd, "amend c1", l)
	if err != nil {
		t.Fatal(err)
	}
	checkCommit(
		c1_v0,
		/*version=*/ 0,
		/*birth*/ commit.BirthReasonCommit,
		/*conflicts*/ false,
		/*children*/ []commit.LocalId{2},
		/*childVers*/ []uint64{0},
		commit.ObsoleteReasonAmend,
		t)
	checkCommit(
		c1_v1,
		/*version=*/ 1,
		/*birth*/ commit.BirthReasonAmend,
		/*conflicts*/ false,
		/*children*/ []commit.LocalId{2},
		/*childVers*/ []uint64{1},
		commit.ObsoleteReasonNone,
		t)
	c2_v1, err := ag.GetLatest(2, l)
	if err != nil {
		t.Fatal(err)
	}
	checkCommit(
		c2_v1,
		/*version=*/ 1,
		/*birth*/ commit.BirthReasonAutoRebaseOfChildren,
		/*conflicts*/ true,
		/*children*/ []commit.LocalId{},
		/*childVers*/ []uint64{},
		commit.ObsoleteReasonNone,
		t)

	// Now lets amend c2_v1 to solve the conflict
	//
	//                          c5(a.txt=bye, c.txt=cc, d.txt=dd)
	//                          |
	// c3(a.txt=BYE, b.txt=bb)  c4(a.txt=bye, c.txt=cc)
	// |                        |
	// c2(a.txt=bye)OBSOLETE----/
	// |
	// c1(a.txt=hello)OBSOLETE
	// |
	// |
	// |
	// |            c2_v1(conflict in a.txt)OBSOLETE    c2_v2(a.txt=bye)
	// |            |                                   |
	// |            c1_v1(a.txt=HELLO)----------------- /
	// root---------/
	wd.WriteFile("a.txt", "bye")
	c2_v2, err := ag.Amend(&c2_v1, true, wd, "amend c2_v2", l)
	if err != nil {
		t.Fatal(err)
	}
	checkCommit(
		c2_v2,
		/*version=*/ 2,
		/*birth*/ commit.BirthReasonAmend,
		/*conflicts*/ false,
		/*children*/ []commit.LocalId{},
		/*childVers*/ []uint64{},
		commit.ObsoleteReasonNone,
		t)
	c1_v1, err = ag.GetLatest(1, l)
	if err != nil {
		t.Fatal(err)
	}
	checkCommit(
		c1_v1,
		/*version=*/ 1,
		/*birth*/ commit.BirthReasonAmend,
		/*conflicts*/ false,
		/*children*/ []commit.LocalId{2, 2},
		/*childVers*/ []uint64{1, 2},
		commit.ObsoleteReasonNone,
		t)
	// Now lets keep going and rebase c3 and c4
	//
	//                          c5(a.txt=bye, c.txt=cc, d.txt=dd)
	//                          |OBSOLETE
	//                          |
	// c3(a.txt=BYE, b.txt=bb)  c4(a.txt=bye, c.txt=cc)
	// |OBSOLETE                |OBSOLETE
	// |                        |
	// c2(a.txt=bye)------------/
	// |OBSOLETE
	// |
	// c1(a.txt=hello)                                           c5_v1
	// |OBSOLETE                                                 |
	// |                                        c3_v1            c4_v1
	// |                                        |                |
	// |            c2_v1(conflict in a.txt)    c2_v2(a.txt=bye)-/
	// |            |OBSOLETE                   |
	// |            |                           |
	// |            c1_v1(a.txt=HELLO)----------/
	// root---------/
	c3_v1, _ := ag.Rebase(&c3_v0, &c2_v2, false, true, l)
	checkCommit(
		c3_v0,
		/*version=*/ 0,
		commit.BirthReasonCommit,
		/*conflicts*/ false,
		/*children*/ []commit.LocalId{},
		/*childVers*/ []uint64{},
		commit.ObsoleteReasonManualRebase,
		t)
	checkCommit(
		c3_v1,
		/*version=*/ 1,
		commit.BirthReasonManualRebase,
		/*conflicts*/ false,
		/*children*/ []commit.LocalId{},
		/*childVers*/ []uint64{},
		commit.ObsoleteReasonNone,
		t)
	c4_v1, _ := ag.Rebase(&c4_v0, &c2_v2, false, true, l)
	checkCommit(
		c4_v0,
		/*version=*/ 0,
		commit.BirthReasonCommit,
		/*conflicts*/ false,
		/*children*/ []commit.LocalId{5},
		/*childVers*/ []uint64{0},
		commit.ObsoleteReasonManualRebase,
		t)
	checkCommit(
		c4_v1,
		/*version=*/ 1,
		commit.BirthReasonManualRebase,
		/*conflicts*/ false,
		/*children*/ []commit.LocalId{5},
		/*childVers*/ []uint64{1},
		commit.ObsoleteReasonNone,
		t)
	c5_v0, err := ag.GetVersion(5, 0, l)
	if err != nil {
		t.Fatal(err)
	}
	checkCommit(
		c5_v0,
		/*version=*/ 0,
		commit.BirthReasonCommit,
		/*conflicts*/ false,
		/*children*/ []commit.LocalId{},
		/*childVers*/ []uint64{},
		commit.ObsoleteReasonAutoRebaseOfChildren,
		t)
	c5_v1, err := ag.GetLatest(5, l)
	if err != nil {
		t.Fatal(err)
	}
	checkCommit(
		c5_v1,
		/*version=*/ 1,
		commit.BirthReasonAutoRebaseOfChildren,
		/*conflicts*/ false,
		/*children*/ []commit.LocalId{},
		/*childVers*/ []uint64{},
		commit.ObsoleteReasonNone,
		t)

}

func TestRestoreSimpleAmend(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)

	// Create a commit and amend it
	wd.WriteFile("a.txt", "hello")
	c1_v0, _ := ag.Commit(wd, "c1", &root, l)
	wd.WriteFile("a.txt", "Hello, world!")
	c1_v1, err := ag.Amend(&c1_v0, false, wd, "amend c1", l)
	if err != nil {
		t.Fatal(err)
	}

	// Create a commit that is the restore of c1_v0
	c1_v2, err := ag.Restore(&c1_v1, c1_v0, l)
	if err != nil {
		t.Fatal(err)
	}
	if c1_v1.Status != commit.StatusObsolete {
		t.Fatal("c1_v1 should become obsolete")
	}
	if c1_v2.L != c1_v0.L {
		t.Fatal("wrong L")
	}
	if c1_v2.RootDirHash != c1_v0.RootDirHash {
		t.Fatal("content of c1_v2 should be the same as c1_v0")
	}
	if !c1_v2.HasDiffData {
		t.Fatal("expected to have diff data")
	}
	if c1_v2.DiffDataLinesCreated != 1 ||
		c1_v2.DiffDataLinesDeleted != 0 ||
		c1_v2.DiffDataLinesModified != 0 ||
		c1_v2.DiffDataFilesCreated != 1 ||
		c1_v2.DiffDataFilesDeleted != 0 ||
		c1_v2.DiffDataFilesModified != 0 {
		t.Fatal("bad diff data")
	}
	checkCommit(
		c1_v2, 2,
		commit.BirthReasonRestore,
		false, nil, nil, commit.ObsoleteReasonNone, t)
}

func TestCantRestoreIfSubmitted(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)

	// Create a commit and amend it
	wd.WriteFile("a.txt", "hello")
	c1_v0, _ := ag.Commit(wd, "c1", &root, l)
	wd.WriteFile("a.txt", "Hello, world!")
	c1_v1, err := ag.Amend(&c1_v0, false, wd, "amend c1", l)
	if err != nil {
		t.Fatal(err)
	}
	// Mock as if c1_v1 was submitted.
	c1_v1.IsSubmitted = true
	_, err = ag.Restore(&c1_v1, c1_v0, l)
	if err == nil {
		t.Fatal("should when trying to restore submitted commits")
	}
}

func TestRestoreWithCHildren(t *testing.T) {
	root, ag, wd, l := newTestClient("owner", 1, t)

	// Create the following tree:
	//
	// *c2_v0  c2_v1
	// |       |      c3_v0
	// *c1_v0  c1_v1--/
	// |       |
	// root----/
	wd.WriteFile("a.txt", "aaa")
	c1_v0, _ := ag.Commit(wd, "c1", &root, l)
	wd.WriteFile("b.txt", "bbb")
	ag.Commit(wd, "c2", &c1_v0, l)
	wd.WriteFile("a.txt", "AAA")
	c1_v1, err := ag.Amend(&c1_v0, true, wd, "amend c1", l)
	if err != nil {
		t.Fatal(err)
	}
	wd.WriteFile("c.txt", "ccc")
	ag.Commit(wd, "c3", &c1_v1, l)

	// Restoring v0 of c1 we expect:
	//
	// *c2_v0  c2_v1
	// |       |       c3_v0
	// *c1_v0  *c1_v1--/       c1_v2 (restore of c1_v0)
	// |       |              |
	// root----/--------------/
	c1_v2, _ := ag.Restore(&c1_v1, c1_v0, l)
	checkCommit(
		c1_v1,
		1,
		commit.BirthReasonAmend,
		false,
		[]commit.LocalId{2, 3},
		[]uint64{1, 0},
		commit.ObsoleteReasonRestored,
		t,
	)
	checkCommit(
		c1_v2,
		2,
		commit.BirthReasonRestore,
		false,
		nil,
		nil,
		commit.ObsoleteReasonNone,
		t,
	)

}

func checkCommit(
	c commit.Commit,
	version uint64,
	birth commit.BirthReason,
	hasConflict bool,
	children []commit.LocalId,
	childrenVers []uint64,
	obsReason commit.ObsoleteReason,
	t *testing.T) {
	if c.Birth != birth {
		t.Fatalf("wrong birth reason")
	}
	if c.Version != version {
		t.Fatalf("expected v %d got %d", version, c.Version)
	}
	if c.HasRebaseConflicts != hasConflict {
		t.Fatalf("expected conflict %v got %v", hasConflict, c.HasRebaseConflicts)
	}
	if len(c.Children) != 0 || len(children) != 0 {
		if !reflect.DeepEqual(c.Children, children) {
			t.Fatal("unexpected children")
		}
	}
	if len(c.ChildrenVersions) != 0 || len(childrenVers) != 0 {
		if !reflect.DeepEqual(c.ChildrenVersions, childrenVers) {
			t.Fatal("unexpected children vers")
		}
	}
	isObsolete := c.Status == commit.StatusObsolete
	if isObsolete && c.ObsReason == commit.ObsoleteReasonNone {
		t.Fatalf("commit is obsolete without reason")
	}
	if !isObsolete && c.ObsReason != commit.ObsoleteReasonNone {
		t.Fatal("commit is not obsolete but has obsolete reason")
	}
	if c.ObsReason != obsReason {
		t.Fatal("wrong obsolete reason")
	}
}
