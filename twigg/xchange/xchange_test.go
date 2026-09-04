package xchange

import (
	"bytes"
	"errors"
	"io"
	"monorepo/twigg/cli/clidb"
	"monorepo/twigg/commit"
	"monorepo/twigg/repo"
	"monorepo/twigg/tree"
	"monorepo/twigg/workdir"
	"path"
	"reflect"
	"testing"
)

// Creates 6 repo versions:
//
// v0: empty
// v1: b.txt=bbb, sl -> b.txt
// v2: a.txt=aaaa, b.txt=bbb, sl -> b.txt
// v3: b.txt=BBB, c.txt=cccc
// v4: b.txt=BBB, c.txt=cccc, d/d1.txt=d1, d/d2.txt=d2
// v5: b.txt=BBB, c.txt=cccc, d/d1.txt=DDDDD, d/d3.txt=d3
func setupTestRepo(
	wd workdir.TestWorkdir,
	rep repo.Repo, l repo.Write, t *testing.T) (v0, v1, v2, v3, v4, v5 repo.TreeVersion) {
	v0, _, err := rep.Init(l)
	if err != nil {
		t.Fatal(err)
	}

	wd.WriteFile("b.txt", "bbb")
	wd.WriteSymlink("sl", "b.txt")
	v1, _, err = rep.Save(wd, v0, l)
	if err != nil {
		t.Fatal(err)
	}
	wd.WriteFile("a.txt", "aaaa")
	v2, _, err = rep.Save(wd, v1, l)
	if err != nil {
		t.Fatal(err)
	}
	wd.WriteFile("b.txt", "BBB")
	wd.WriteFile("c.txt", "ccc")
	wd.Delete("a.txt")
	wd.Delete("sl")
	v3, _, err = rep.Save(wd, v2, l)
	if err != nil {
		t.Fatal(err)
	}

	wd.WriteFile("d/d1.txt", "d1")
	wd.WriteFile("d/d2.txt", "d2")
	v4, _, err = rep.Save(wd, v4, l)
	if err != nil {
		t.Fatal(err)
	}

	wd.Delete("d/d2.txt")
	wd.WriteFile("d/d1.txt", "DDDDD")
	wd.WriteFile("d/d3.txt", "d3")
	v5, _, err = rep.Save(wd, v4, l)
	if err != nil {
		t.Fatal(err)
	}
	return
}

// Checks that the iter currently is at `path`.
// if contents="", checks that it's a directory. Else, checks that the file
// has the contents `content`. It advances the iterator after checking
func checkIter(iter repo.DeltaIter,
	path_ string, content string, t *testing.T) {
	if !iter.CanGet() {
		t.Fatalf("should be able to get %s", path_)
	}
	treePath, gotTreeDepth, tree := iter.Get()
	if treePath != path_ {
		t.Fatalf("expected path %s, got %s", path_, treePath)
	}
	if tree.Data().BaseName != path.Base(path_) {
		t.Fatalf(
			"expected baseName %s, got %s",
			path.Base(path_), tree.Data().BaseName)
	}
	if gotTreeDepth != tree.Data().Depth {
		t.Fatalf("inconsistent depth %d %d", gotTreeDepth, tree.Data().Depth)
	}
	if content == "" {
		if !tree.Data().IsDir {
			t.Fatal("expected tree to be a directory")
		}
		err := iter.Pop()
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	if tree.Data().IsDir {
		t.Fatalf(
			"expected tree with content %s, but tree is a directory", content)
	}
	if tree.Data().Size != int64(len(content)) {
		t.Fatal("wrong file size")
	}
	wt, err := tree.GetFile()
	if err != nil {
		t.Fatal(err)
	}
	buff := bytes.NewBuffer(nil)
	_, err = wt.WriteTo(buff)
	if err != nil {
		t.Fatal(err)
	}
	s := buff.String()
	if s != content {
		t.Fatalf("expected content %s, got %s", content, s)
	}
	err = iter.Pop()
	if err != nil {
		t.Fatal(err)
	}
}

// Checks that the iter currently is at `filePath` and the that file
// has the provided size. It verifies that Writing the file actually causes
// an error (i.e. the iterator doesn't contain the file content.)
func checkIterCantReadFileContent(iter repo.DeltaIter,
	filePath string, size int64, t *testing.T) {
	if !iter.CanGet() {
		t.Fatalf("should be able to get %s", filePath)
	}
	treePath, _, tree := iter.Get()
	if treePath != filePath {
		t.Fatalf("expected filePath %s, got %s", filePath, treePath)
	}
	if tree.Data().BaseName != path.Base(treePath) {
		t.Fatalf("expected filename %s, got %s",
			path.Base(treePath), tree.Data().BaseName)
	}
	if tree.Data().Size != size {
		t.Fatal("wrong file size")
	}
	wt, err := tree.GetFile()
	if err != nil {
		t.Fatal(err)
	}
	buff := bytes.NewBuffer(nil)
	_, err = wt.WriteTo(buff)
	if err == nil {
		t.Fatal("expected fail because the iter should not have the content")
	}
	err = iter.Pop()
	if err != nil {
		t.Fatal(err)
	}
}

// Checks that the iter currently is at `filePath`.
// and that it's a symlink to target It advances the iterator after checking
func checkIterSymlink(iter repo.DeltaIter,
	filePath string, target string, t *testing.T) {
	if !iter.CanGet() {
		t.Fatalf("should be able to get %s", filePath)
	}
	treePath, _, tree := iter.Get()
	if treePath != filePath {
		t.Fatalf("expected filePath %s, got %s", filePath, treePath)
	}
	if tree.Data().BaseName != path.Base(treePath) {
		t.Fatalf("expected filename %s, got %s",
			path.Base(treePath), tree.Data().BaseName)
	}
	if !tree.Data().IsSymlink {
		t.Fatalf("expected %s to be a symlink", filePath)
	}
	if tree.Data().SymlinkTarget != target {
		t.Fatalf("expected %s to be a symlink to %s", filePath, target)
	}
	err := iter.Pop()
	if err != nil {
		t.Fatal(err)
	}
}

func TestWriteAndReadManyCommits(t *testing.T) {
	db, closeDb, err := clidb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDb)
	dbLock, dbUl, _, err := db.BeginWrite()
	defer dbUl()
	if err != nil {
		t.Fatal(err)
	}
	l := db.Bind(dbLock)

	rep := repo.New("owner", 1)
	wd := workdir.NewTest("testdataWd", t)
	v0, v1, v2, v3, v4, v5 := setupTestRepo(wd, rep, l, t)
	// Note that we're mocking the server Ids (ServerL, ServerV)
	v1Commit := commit.Commit{
		ServerL:     11,
		ServerV:     111,
		TreeVersion: v1,
		Message:     "create b.txt=bbb and sl->b.txt",
	}
	v2Commit := commit.Commit{
		ServerL:     22,
		ServerV:     222,
		TreeVersion: v2,
		Message:     "create a.txt=aaaa",
	}
	v3Commit := commit.Commit{
		ServerL:     33,
		ServerV:     333,
		TreeVersion: v3,
		Message:     "delete a.txt, b.txt=BBB, create c.txt=ccc",
	}
	v4Commit := commit.Commit{
		ServerL:     44,
		ServerV:     444,
		TreeVersion: v4,
		Message:     "create d/d1.txt and d/d2.txt",
	}
	v5Commit := commit.Commit{
		ServerL:     55,
		ServerV:     555,
		TreeVersion: v5,
		Message:     "edit d/d1.txt=DDDDD, delete d/d2.txt, create d/d3.txt=d3",
	}

	r, w := io.Pipe()
	writeErrs := make(chan error, 1)
	go func() {
		wt, cl, err := NewCommitWriter(w)
		defer cl()
		// Write all commits, in order
		wt.Write(v1Commit, 0, 0, v0, rep, l)
		errors.Join(err, wt.Write(
			v2Commit, 11, 111, v1Commit.TreeVersion, rep, l))
		// Simulate that we could only write 2 commits per time
		errors.Join(err, wt.WriteUnexpectedEof())
		// Continue writing
		errors.Join(err, wt.Write(
			v3Commit, 22, 222, v2Commit.TreeVersion, rep, l))
		errors.Join(err, wt.Write(
			v4Commit, 33, 333, v3Commit.TreeVersion, rep, l))
		errors.Join(err, wt.Write(
			v5Commit, 44, 444, v4Commit.TreeVersion, rep, l))
		// Write EOF to indicate its the end
		errors.Join(err, wt.WriteEof())
		writeErrs <- err
	}()

	cr, closeCr, err := NewCommitReader(r)
	defer closeCr()
	if err != nil {
		t.Fatal(err)
	}

	// Read one by one. First commit will be the v1Commit
	gotC, gotBaseServerL, gotBaseServerV, readIter, err := cr.Read()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotC, v1Commit) {
		t.Fatal("got wrong commit")
	}
	if gotBaseServerL != 0 || gotBaseServerV != 0 {
		t.Fatal("wrong base")
	}
	checkIter(readIter, "b.txt", "bbb", t)
	checkIterSymlink(readIter, "sl", "b.txt", t)
	checkIter(readIter, tree.RootPath, "", t)

	// Start reading again. This next part will be the v2Commit.
	gotC, gotBaseServerL, gotBaseServerV, readIter, err = cr.Read()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotC, v2Commit) {
		t.Fatal("got wrong commit")
	}
	if gotBaseServerL != v1Commit.ServerL || gotBaseServerV != v1Commit.ServerV {
		t.Fatal("wrong base")
	}
	checkIter(readIter, "a.txt", "aaaa", t)
	// Iter should but not contain contents of b.txt as it was not modified
	checkIterCantReadFileContent(readIter, "b.txt", 3, t)
	checkIterSymlink(readIter, "sl", "b.txt", t)
	checkIter(readIter, tree.RootPath, "", t)

	// Now we'll get an UnexpectedEOF. That's the writer telling
	// "I wanted to write more commits but I couldn't, please try again"
	_, _, _, _, err = cr.Read()
	if err != io.ErrUnexpectedEOF {
		t.Fatal(err)
	}

	// Start reading again. This last part will be v3Commit.
	gotC, gotBaseServerL, gotBaseServerV, readIter, err = cr.Read()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotC, v3Commit) {
		t.Fatal("got wrong commit")
	}
	if gotBaseServerL != v2Commit.ServerL || gotBaseServerV != v2Commit.ServerV {
		t.Fatal("wrong base")
	}

	checkIter(readIter, "b.txt", "BBB", t)
	checkIter(readIter, "c.txt", "ccc", t)
	checkIter(readIter, tree.RootPath, "", t)

	// Start reading again. This last part will be v4Commit.
	gotC, gotBaseServerL, gotBaseServerV, readIter, err = cr.Read()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotC, v4Commit) {
		t.Fatal("got wrong commit")
	}
	if gotBaseServerL != v3Commit.ServerL || gotBaseServerV != v3Commit.ServerV {
		t.Fatal("wrong base")
	}
	checkIterCantReadFileContent(readIter, "b.txt", 3, t)
	checkIterCantReadFileContent(readIter, "c.txt", 3, t)
	checkIter(readIter, "d/d1.txt", "d1", t)
	checkIter(readIter, "d/d2.txt", "d2", t)
	checkIter(readIter, "d", "", t)
	checkIter(readIter, tree.RootPath, "", t)

	// Start reading again. This last part will be v5Commit.
	gotC, gotBaseServerL, gotBaseServerV, readIter, err = cr.Read()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotC, v5Commit) {
		t.Fatal("got wrong commit")
	}
	if gotBaseServerL != v4Commit.ServerL || gotBaseServerV != v4Commit.ServerV {
		t.Fatal("wrong base")
	}

	checkIterCantReadFileContent(readIter, "b.txt", 3, t)
	checkIterCantReadFileContent(readIter, "c.txt", 3, t)
	checkIter(readIter, "d/d1.txt", "DDDDD", t)
	checkIter(readIter, "d/d3.txt", "d3", t)
	checkIter(readIter, "d", "", t)
	checkIter(readIter, tree.RootPath, "", t)

	// Reading again will EOF
	_, _, _, _, err = cr.Read()
	if err != io.EOF {
		t.Fatal("expected EOF after done reading")
	}

	err = <-writeErrs
	if err != nil {
		t.Fatal(err)
	}
}

func TestCommitWriterErrMsg(t *testing.T) {
	r, w := io.Pipe()
	writeErrs := make(chan error, 1)
	go func() {
		wt, cl, err := NewCommitWriter(w)
		defer cl()
		errors.Join(err, wt.WriteErrMsg("hello"))
		errors.Join(err, wt.WriteEof())
		writeErrs <- err
	}()
	cr, closeCr, err := NewCommitReader(r)
	defer closeCr()
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, _, err = cr.Read()
	if err.Error() != "hello" {
		t.Fatal("expected to read the error message")
	}
	_, _, _, _, err = cr.Read()
	if err != io.EOF {
		t.Fatal("expected EOF after done reading")
	}
	err = <-writeErrs
	if err != nil {
		t.Fatal(err)
	}
}

func TestIdWriteAndRead(t *testing.T) {
	r, w := io.Pipe()

	writeErrs := make(chan error, 1)
	go func() {
		idWriter, closeIdWriter, err := NewCommitIdWriter(w)
		defer closeIdWriter()
		err = errors.Join(err, idWriter.Write(2, 3))
		err = errors.Join(err, idWriter.Write(9, 10))
		err = errors.Join(err, idWriter.WriteErrMsg("hello error"))
		err = errors.Join(err, idWriter.WriteEof())
		writeErrs <- err
	}()

	idReader, closeIdReader, err := NewCommitIdReader(r)
	defer closeIdReader()
	if err != nil {
		t.Fatal(err)
	}

	l, v, err := idReader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if l != 2 || v != 3 {
		t.Fatal("wrong vals 1")
	}
	l, v, err = idReader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if l != 9 || v != 10 {
		t.Fatal("wrong vals 2")
	}
	_, _, err = idReader.Read()
	if err.Error() != "hello error" {
		t.Fatal("expected hello error")
	}
	_, _, err = idReader.Read()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF got %s", err)
	}

	err = <-writeErrs
	if err != nil {
		t.Fatal(err)
	}
}

func TestOldProtocolErrForCommitWriter(t *testing.T) {
	r, w := io.Pipe()
	writeErrs := make(chan error, 1)
	go func() {
		UseMockProtocolVersionSentByWriters = true
		MockProtocolVersionSentByWriters = CurrentProtocol + 1
		t.Cleanup(func() { UseMockProtocolVersionSentByWriters = false })

		cw, cl, err := NewCommitWriter(w)
		defer cl()
		err = errors.Join(err, cw.WriteEof())
		writeErrs <- err
	}()
	_, closeCr, err := NewCommitReader(r)
	defer closeCr()
	if !errors.Is(err, ErrOldProtocol) {
		t.Fatalf("expected old protocol version got %s", err)
	}
}

func TestOldProtocolErrForIdWriter(t *testing.T) {
	r, w := io.Pipe()
	writeErrs := make(chan error, 1)
	go func() {
		UseMockProtocolVersionSentByWriters = true
		MockProtocolVersionSentByWriters = CurrentProtocol + 1
		t.Cleanup(func() { UseMockProtocolVersionSentByWriters = false })

		idW, closeIdWriter, err := NewCommitIdWriter(w)
		defer closeIdWriter()
		err = errors.Join(err, idW.WriteEof())
		writeErrs <- err
	}()
	_, closeIdReader, err := NewCommitIdReader(r)
	defer closeIdReader()
	if !errors.Is(err, ErrOldProtocol) {
		t.Fatalf("expected old protocol version got %s", err)
	}
}