package workdir

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"monorepo/twigg/tree"
	"os"
	"path"
	"sort"
	"strings"
)

// Creates a new tree with path=treePath
func (w *wd) newTree(treePath string) (tree_, error) {
	var depth uint32
	if treePath == tree.RootPath {
		depth = 0
	} else {
		depth = uint32(strings.Count(treePath, "/") + 1)
	}
	if depth > w.maxReadDepth+1 {
		// The ignores are read "level per level".
		// I.e. we expect to read depth=0, then depth=1, etc.
		// If we read depth=2 and then depth=4, we might skip an ignore file.
		panic("skipped a workdir depth during read")
	}
	cleanAbsPath := w.cleanAbsPath(treePath)
	w.maxReadDepth = max(w.maxReadDepth, depth)

	// Base case is if the tree is just a file, so we check that firs
	info, err := os.Lstat(cleanAbsPath)
	if errors.Is(err, os.ErrNotExist) {
		return tree_{}, tree.ErrTreeNotFound
	}
	if err != nil {
		return tree_{},
			fmt.Errorf("failed to stat path %q: %w", treePath, err)
	}
	if w.isIgnored(treePath) {
		return tree_{}, tree.ErrTreeNotFound
	}
	if !info.IsDir() {
		data, _, err := w.getFile(treePath, info)
		if err != nil {
			return tree_{}, err
		}
		return tree_{
			cleanAbsPath: cleanAbsPath,
			data:         data,
		}, nil
	}
	childrenDirEntries, err := os.ReadDir(cleanAbsPath)
	if errors.Is(err, os.ErrNotExist) {
		return tree_{}, tree.ErrTreeNotFound
	}
	if err != nil {
		return tree_{}, err
	}

	// First check for ignore files
	for _, childDirEntry := range childrenDirEntries {
		if !childDirEntry.IsDir() && isIgnoreFile(childDirEntry.Name()) {
			err = w.parseIgnoreFile(path.Join(treePath, childDirEntry.Name()))
			if err != nil {
				return tree_{}, err
			}
			break
		}
	}

	childrenNames := make([]string, 0, len(childrenDirEntries))
	for _, childDirEntry := range childrenDirEntries {
		childPath := path.Join(treePath, childDirEntry.Name())
		if w.isIgnored(childPath) {
			continue
		}
		childrenNames = append(childrenNames, childDirEntry.Name())
	}
	sort.Slice(childrenNames, func(i, j int) bool {
		return childrenNames[i] < childrenNames[j]
	})
	return tree_{
		cleanAbsPath: cleanAbsPath,
		data:         tree.NewUnknownDirData(path.Base(treePath), depth, childrenNames),
	}, nil
}

type tree_ struct {
	cleanAbsPath string
	data         tree.Data
}

func (t tree_) IsRemovedChild() bool {
	return false
}
func (t tree_) DataIsComplete() bool {
	return !t.data.IsDir
}
func (t tree_) Data() tree.Data {
	return t.data
}
func (t tree_) GetFile() (wt io.WriterTo, err error) {
	if t.data.IsDir {
		panic("called GetFile for IsDir=false")
	}
	if t.data.IsSymlink {
		return bytes.NewBufferString(tree.SymlinkString(t.data.SymlinkTarget)), nil
	}
	return newFileWt(t.cleanAbsPath), nil
}