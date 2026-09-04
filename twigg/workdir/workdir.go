package workdir

import (
	"errors"
	"fmt"
	"io"
	diff3 "monorepo/twigg/diff/epiclabs-io"
	"monorepo/twigg/gitignore"
	"monorepo/twigg/tree"
	"os"
	"path"
	"path/filepath"
	"time"
)

type wd struct {
	minSizeToCacheFile      int64
	rootAbsPathSlashForward string
	ignoreRules             []string
	ign                     *gitignore.GitIgnore
	hasIgnCache             bool
	maxReadDepth            uint32
	wl                      Write
}

// Doesnt commit the lock
func newWd(absPathSlashForward string, wl Write, minSizeToCacheFile int64) (*wd, error) {
	absPathSlashForward = filepath.ToSlash(absPathSlashForward)
	if !filepath.IsAbs(absPathSlashForward) {
		return &wd{}, errors.New("root must be absolute")
	}
	if err := os.MkdirAll(filepath.Clean(absPathSlashForward), 0755); err != nil {
		return nil, err
	}
	w := &wd{
		rootAbsPathSlashForward: absPathSlashForward,
		ignoreRules:             []string{},
		maxReadDepth:            0,
		wl:                      wl,
		hasIgnCache:             false,
		minSizeToCacheFile:      minSizeToCacheFile,
	}
	return w, nil
}

func (w wd) Delete(path string) error {
	basePath := filepath.Base(path)
	if basePath == ".twigg" || basePath == ".sl" {
		// Leaving this on now for safety
		panic("tried to delete version control folder")
	}
	return os.RemoveAll(w.cleanAbsPath(path))
}
func (w wd) Purge() error {
	entries, err := os.ReadDir(w.rootAbsPathSlashForward)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		err = os.RemoveAll(filepath.Join(w.rootAbsPathSlashForward, entry.Name()))
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *wd) IsEmpty() (bool, error) {
	iter, err := tree.Walk(w)
	if err != nil {
		return false, err
	}
	for iter.CanGet() {
		_, _, _, tr := iter.Get()
		if tr.IsRemovedChild() || !tr.DataIsComplete() {
			err := iter.Next()
			if err != nil {
				return false, err
			}
			continue
		}
		if !tr.Data().IsDir {
			return false, nil
		}
		err := iter.Next()
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

func (w *wd) WriteSymlink(path_, target string) error {
	// Delete the current file if it exists
	cleanAbsPath := w.cleanAbsPath(path_)
	err := os.RemoveAll(cleanAbsPath)
	if err != nil {
		return err
	}

	// Ensure that the directory of the symlink file
	err = os.MkdirAll(filepath.Dir(cleanAbsPath), os.ModePerm)
	if err != nil {
		return err
	}
	linkTarget := filepath.FromSlash(target) // convert to/from slashes

	// Link: cleanAbsPath->linkTarget
	return os.Symlink(linkTarget, cleanAbsPath)
}
func (w *wd) Write(path_ string, isExecutable bool, wt io.WriterTo) error {
	cleanAbsPath := w.cleanAbsPath(path_)
	err := os.MkdirAll(filepath.Dir(cleanAbsPath), 0755)
	if err != nil {
		return err
	}

	os.RemoveAll(cleanAbsPath)
	var f *os.File
	flag := os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	if isExecutable {
		f, err = os.OpenFile(cleanAbsPath, flag, 0755)
	} else {
		f, err = os.OpenFile(cleanAbsPath, flag, 0644)
	}
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()

	_, err = wt.WriteTo(f)
	return err
}

func (w wd) CreateFolder(dirName string) error {
	if w.HasFolder(w.rootAbsPathSlashForward + "/" + dirName) {
		return fmt.Errorf("folder %q already exist", dirName)
	}
	return os.Mkdir(w.absPathSlashForward(dirName), 0755)
}

func (w wd) SetModTime(path_ string, unixMillis int64) error {
	modTime := time.UnixMilli(unixMillis)
	return os.Chtimes(w.cleanAbsPath(path_), modTime, modTime)
}

func (w wd) HasFolder(path_ string) bool {
	fileInfo, err := os.Stat(w.cleanAbsPath(path_))
	if err == nil {
		return fileInfo.IsDir()
	}
	return false
}

// Change the root to the parent folder.
func (w *wd) GoUp() error {
	if w.rootAbsPathSlashForward == path.Dir(w.rootAbsPathSlashForward) {
		return ErrCantGoUp
	}
	w.rootAbsPathSlashForward = path.Dir(w.rootAbsPathSlashForward)
	return nil
}

// Equivalent to cd "dirName'
// Only returns error if it can not find dir.
func (w *wd) GoDown(dirName string) {
	if !w.HasFolder(dirName) {
		panic("called GoDown() for a num existing dir")
	}
	w.rootAbsPathSlashForward = w.rootAbsPathSlashForward + "/" + dirName
}

func (w *wd) Path() string {
	return w.rootAbsPathSlashForward
}

func (w wd) absPathSlashForward(pathSlashForward string) string {
	return w.rootAbsPathSlashForward + "/" + pathSlashForward
}
func (w wd) cleanAbsPath(path_ string) string {
	return filepath.Clean(w.absPathSlashForward(path_))
}

// func isRelPath(path_ string) bool {
// 	return !path.IsAbs(path_)
// }

// func (w wd) relPath(p string) string {
// 	if !path.IsAbs(p) {
// 		panic(fmt.Sprintf("%s is alread relative", p))
// 	}
// 	r, err := filepath.Rel(w.root, p)
// 	if err != nil {
// 		panic(fmt.Sprintf("failed to get rel of %s: %s", p, err))
// 	}
// 	return r
// }

func (w *wd) FileHasConflict(path string) (has bool, err error) {
	return containsConflictMarkers(w.absPathSlashForward(path))
}

func containsConflictMarkers(absPath string) (bool, error) {
	cleanAbsPath := filepath.Clean(absPath)

	file, err := os.Open(cleanAbsPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close()

	return hasConflictMarker(file)
}
func hasConflictMarker(r io.Reader) (bool, error) {
	patterns := [][]byte{
		[]byte("\n" + diff3.ConflictStart),
		[]byte("\n" + diff3.ConflictMid),
		[]byte("\n" + diff3.ConflictEnd),
	}
	// running match length for each pattern.
	// matchLen[i] = 1 -> for the pattern i, the rolling match length is 1
	matchLen := make([]int, len(patterns))

	buff := buffPool.Get().(*[]byte)
	defer buffPool.Put(buff)
	for {
		n, err := r.Read(*buff)
		if n > 0 {
			// Reach each byte
			for _, b := range (*buff)[:n] {
				// Adjust the `pos` of each pattern
				for i, p := range patterns {
					// If the byte matches
					if b == p[matchLen[i]] {
						matchLen[i]++
						if matchLen[i] == len(p) {
							return true, nil
						}
					} else {
						// Restart match
						// If the current byte is the same as the first,
						// reset to 1. Else, reset to 0
						if b == p[0] {
							matchLen[i] = 1
						} else {
							matchLen[i] = 0
						}
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
	}
	return false, nil
}

func (w *wd) Tree(path string) (tree.Tree, error) {
	return w.newTree(path)
}
