package clidb_test

import (
	"monorepo/twigg/tree"
	"monorepo/twigg/treev"
	"reflect"
	"testing"
)

func Test_TreeData(t *testing.T) {
	db := getNewDb(t)
	w, closeW, commitW, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	_, isNotFoundErr, err := db.GetTreeData(w, 99, tree.RootPath, 0)
	if err == nil || !isNotFoundErr {
		t.Fatalf("got no isNotFoundErr")
	}
	_, isNotFoundErr, err = db.GetLastVersionOfRootTree(w, 99)
	if err == nil || !isNotFoundErr {
		t.Fatalf("got no isNotFoundErr")
	}

	td := treev.TreeDataV{
		Data:             tree.Data{BaseName: tree.RootPath, IsDir: true},
		ChildrenVersions: []uint64{0, 1},
		BlobVersion:      3,
	}

	v, err := db.SetTreeData(w, "owner", 99, tree.RootPath, td)
	if err != nil {
		t.Fatal(err)
	}
	if v != 0 {
		t.Fatalf("v=%d, expected 0", v)
	}

	got, isNotFoundErr, err := db.GetTreeData(w, 99, tree.RootPath, 0)
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, td) {
		t.Fatalf("got=%+v, expected %+v", got, td)
	}

	rootV, isNotFoundErr, err := db.GetLastVersionOfRootTree(w, 99)
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if rootV != 0 {
		t.Fatalf("rootV=%d, expected 0", rootV)
	}

	// Second write must create version 1 and update the "last version"
	_, err = db.SetTreeData(w, "owner", 99, tree.RootPath, td)
	if err != nil {
		t.Fatal(err)
	}
	rootV, isNotFoundErr, err = db.GetLastVersionOfRootTree(w, 99)
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if rootV != 1 {
		t.Fatalf("rootV=%d, expected 1", rootV)
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}

func Test_TreeBlob(t *testing.T) {
	db := getNewDb(t)
	w, closeW, commitW, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	_, closeR, isNotFoundErr, err := db.GetTreeBlob(w, 99, "a/b.txt", 0)
	closeR()
	if err == nil || !isNotFoundErr {
		t.Fatalf("got no isNotFoundErr")
	}

	v, err := db.SetTreeBlob(w, "owner", 99, "a/b.txt", bytesWriterTo("file-contents"))
	if err != nil {
		t.Fatal(err)
	}
	if v != 0 {
		t.Fatalf("v=%d, expected 0", v)
	}

	r, closeR, isNotFoundErr, err := db.GetTreeBlob(w, 99, "a/b.txt", 0)
	if err != nil || isNotFoundErr {
		closeR()
		t.Fatal(err)
	}
	data := readAll(t, r, closeR)
	if string(data) != "file-contents" {
		t.Fatalf("data=%q, expected %q", data, "file-contents")
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}
