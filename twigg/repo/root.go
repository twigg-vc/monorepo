package repo

import (
	"errors"
	"fmt"
	"monorepo/base/fifo"
	"monorepo/base/queue"
	"monorepo/twigg/tree"
	"monorepo/twigg/treev"
	"path"
	"strings"
)

type root struct {
	repoId      uint64
	l_          Read
	rootVersion TreeVersion
	cache       fifo.Cache[string, treev.TreeDataV]
}

// Increasing cacheSize shouldn't really help much.
// Almost all cache hits happen when we need to lookup a parent tree when we're
// traversing the tree and need the parent tree. I.e. the VAST majority of
// cache lookups will just be of the most recent key set
const cacheSize = 10

func (r repo) Root(v TreeVersion, l Read) tree.Root {
	return r.getRoot_(v, l)
}

func (r repo) getRoot_(v TreeVersion, l Read) root {
	return root{repoId: r.id, l_: l, rootVersion: v, cache: fifo.New[string, treev.TreeDataV](cacheSize)}
}

func (root root) Tree(relativePath string) (tree.Tree, error) {
	_, tr, err := root.getTree(relativePath)
	return tr, err
}

func (r root) getTree(treePath string) (TreeVersion, tree_, error) {
	if treePath == tree.RootPath {
		parentTreeData, isNotFoundErr, err := r.getTreeData(
			tree.RootPath, r.rootVersion)
		if isNotFoundErr {
			return 0, tree_{}, tree.ErrTreeNotFound
		}
		if err != nil {
			return 0, tree_{}, err
		}
		return r.rootVersion, tree_{
			repoId: r.repoId,
			d:      parentTreeData,
			l:      r.l_,
		}, nil
	}
	pathDepth := uint32(strings.Count(treePath, "/") + 1)

	// To find the tree we must traverse from a known parent to find the
	// version of each child.
	// In the worse case, we can always start from the root, but that will
	// require the whole tree to be traversed until we find the desired tree.
	// For better performance, we check the cached trees from the bottom up;
	// i.e. we look for the deepest path and trim it and try again
	var err error
	var isNotFoundErr bool
	var parentTreeData treev.TreeDataV
	foundParentTreeData := false
	parentPath := path.Dir(treePath)
	for parentPath != tree.RootPath {
		parentTreeData, foundParentTreeData = r.cache.Get(parentPath)
		if foundParentTreeData {
			break
		}
		parentPath = path.Dir(parentPath)
	}
	if !foundParentTreeData {
		parentTreeData, isNotFoundErr, err = r.getTreeData(
			tree.RootPath, r.rootVersion)
		if isNotFoundErr {
			return 0, tree_{}, tree.ErrTreeNotFound
		}
		if err != nil {
			return 0, tree_{}, err
		}
	}
	// At this point, the parent has to be of a smallest depth
	if parentTreeData.Data.Depth >= pathDepth {
		panic("parentTree is same depth")
	}

	// Now we iterate child by child to find the path.
	// We put the chunks in a queue just to get which one we're looking for next
	// We just need to first adujst that queue based on the path we're starting
	// from.
	currentPath := parentPath
	if currentPath == tree.RootPath {
		currentPath = ""
	}
	pathChunks := strings.Split(treePath, "/")
	chunks := queue.New[string]()
	for _, pathChunk := range pathChunks {
		chunks.Push(pathChunk)
	}
	currentPathChunks := strings.Split(currentPath, "/")
	for _, currentPathChunk := range currentPathChunks {
		if *chunks.Peek() == currentPathChunk {
			chunks.Pop()
			continue
		}
		break
	}

	currentTreeData := parentTreeData
	var currentTreeVersion TreeVersion
	for currentPath != treePath {
		nextChunk := chunks.Pop()
		found := false
		for i := 0; i < len(currentTreeData.ChildrenVersions); i++ {
			childName := currentTreeData.Data.ChildrenBaseNames[i]
			childVersion := currentTreeData.ChildrenVersions[i]
			if childName == nextChunk {
				found = true
				currentPath = path.Join(currentPath, childName)
				currentTreeData, isNotFoundErr, err = r.getTreeData(currentPath, childVersion)
				if isNotFoundErr {
					return 0, tree_{}, tree.ErrTreeNotFound
				}
				if err != nil {
					return 0, tree_{}, err
				}
				currentTreeVersion = childVersion
				break
			}
		}
		if !found {
			break
		}
	}
	if currentPath == treePath {
		return currentTreeVersion, tree_{
			treePath: currentPath,
			repoId:   r.repoId,
			d:        currentTreeData,
			l:        r.l_,
		}, nil
	}
	return 0, tree_{}, errors.Join(
		fmt.Errorf("could not find subtree %s in root tree version %d", treePath,
			r.rootVersion),
		tree.ErrTreeNotFound)
}

func (root root) getTreeData(treePath string, v uint64) (treev.TreeDataV, bool, error) {
	d, ok := root.cache.Get(treePath)
	if ok {
		return d, false, nil
	}
	td, isNotFoundErr, err := root.l_.GetTreeData(root.repoId, treePath, v)
	if err != nil {
		return td, isNotFoundErr, err
	}
	root.cache.Put(treePath, td)
	return td, false, nil
}