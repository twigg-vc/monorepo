package workdir

import (
	"bytes"
	"io"
	"monorepo/twigg/cli/clidb"
	"monorepo/twigg/tree"
	"os"
	"path"
	"path/filepath"
	"testing"
)

// Used for testing. Simulates multiple partitions as well.
type testWd struct {
	w      *wd
	wdPath string
	t      testing.TB
}

func newTestWd(path_ string, cleanup bool, minSizeToCacheFile int64, t testing.TB) TestWorkdir {
	if path.IsAbs(path_) {
		t.Fatal("tried to create test wd with abs path")
	}
	// Note that osWd has os-specific format
	osWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// wdPath uses slashes format
	wdPath := filepath.ToSlash(osWd)

	rootPath := path_
	for path.Dir(rootPath) != rootPath &&
		path.Clean(path.Dir(rootPath)) != "." {
		rootPath = path.Dir(rootPath)
	}
	if cleanup {
		os.RemoveAll(filepath.Join(wdPath, rootPath))
		t.Cleanup(func() {
			os.RemoveAll(filepath.Join(wdPath, rootPath))
		})
	}

	cache, close, err := clidb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(close)
	wl, ul, commit, err := cache.BeginWrite()
	t.Cleanup(ul)
	if err != nil {
		t.Fatal(err)
	}
	sWl := cache.Bind(wl)
	w, err := newWd(path.Join(wdPath, path_), sWl, minSizeToCacheFile)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cache.ShouldCommit(wl) {
			err = commit()
			if err != nil {
				t.Fatal(err)
			}
		}
	})

	return &testWd{
		w:      w,
		wdPath: wdPath,
		t:      t,
	}
}

func (twd testWd) Tree(path string) (tree.Tree, error) {
	return twd.w.Tree(path)
}

func (twd testWd) ClearIgnores() {
	twd.w.ClearIgnores()
}
func (twd testWd) Ignore(pattern string) error {
	return twd.w.Ignore(pattern)
}
func (twd testWd) Delete(path string) error {
	return twd.w.Delete(path)
}
func (twd testWd) Purge() error {
	return twd.w.Purge()
}
func (twd testWd) IsEmpty() (bool, error) {
	return twd.w.IsEmpty()
}
func (twd testWd) Write(path string, isExecutable bool, wt io.WriterTo) error {
	return twd.w.Write(path, isExecutable, wt)
}
func (twd testWd) WriteSymlink(path, target string) error {
	return twd.w.WriteSymlink(path, target)
}
func (twd testWd) SetModTime(path string, unixMicros int64) error {
	return twd.w.SetModTime(path, unixMicros)
}
func (twd testWd) CreateFolder(dirName string) error {
	return twd.w.CreateFolder(dirName)
}
func (twd testWd) HasFolder(path string) bool {
	return twd.w.HasFolder(path)
}
func (twd testWd) Path() string {
	return twd.w.Path()
}
func (twd testWd) GoUp() error {
	if path.Dir(twd.w.rootAbsPathSlashForward) == twd.wdPath {
		return ErrCantGoUp
	}
	return twd.w.GoUp()
}
func (twd testWd) GoDown(dirName string) {
	if !twd.HasFolder(dirName) {
		panic("called GoDown() for a num existing dir")
	}
	twd.w.GoDown(dirName)
}
func (twd testWd) WriteFile(path string, content string) {
	buff := bytes.NewBufferString(content)
	err := twd.w.Write(path,
		/*isExecutable*/ false, buff)
	if err != nil {
		twd.t.Fatal(err)
	}
}
func (twd testWd) WriteExecutableFile(path string, content string) {
	buff := bytes.NewBufferString(content)
	err := twd.w.Write(path,
		/*isExecutable*/ true, buff)
	if err != nil {
		twd.t.Fatal(err)
	}
}

func (twd testWd) ReadFile(path string) string {
	buff := bytes.NewBuffer(nil)

	if twd.w.isIgnored(path) {
		twd.t.Fatalf("file %s is ignored found", path)
	}
	wt := newFileWt(twd.w.cleanAbsPath(path))
	_, err := wt.WriteTo(buff)
	if err != nil {
		twd.t.Fatal(err)
	}
	return buff.String()
}
func (twd testWd) HasFile(path string) bool {
	dfs, err := tree.Walk(twd.w)
	if err != nil {
		twd.t.Fatal(err)
	}
	var p string
	var file tree.Tree
	for dfs.CanGet() {
		p, _, _, file = dfs.Get()
		if p == path && !file.Data().IsDir {
			return true
		}
		err = dfs.Next()
		if err != nil {
			twd.t.Fatal(err)
		}
	}
	return false
}

func (twd testWd) FileHasConflict(path string) (bool, error) {
	return twd.w.FileHasConflict(path)
}

func (twd testWd) RootDirHash() [32]byte {
	it, err := tree.Walk(twd)
	if err != nil {
		twd.t.Fatal(err)
	}
	for {
		_, depth, _, tr := it.Get()
		if tr.DataIsComplete() && depth == 0 {
			return tr.Data().ContentHash
		}
		err = it.Next()
		if err != nil {
			twd.t.Fatal(err)
		}
	}
}