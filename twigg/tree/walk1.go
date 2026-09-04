package tree

import (
	"errors"
	"fmt"
	"io"
	"monorepo/base/stack"
	"path"
	"strings"
)

func walk(root Root) (Iterator, error) {
	st := stack.New[*treeIterNode]()
	rootTree, err := root.Tree(RootPath)
	if err != nil {
		return nil, err
	}
	if !rootTree.Data().IsDir {
		return nil, errors.New("root path must be a directory")
	}
	checkTreeOrDie(rootTree)
	st.Push(newTreeIterNode(RootPath, 0,
		newNullFileGetter(), rootTree.Data(),
		rootTree.DataIsComplete(), false))
	return &iter1{
		root:             root,
		stack:            st,
		current:          (*st.Peek()),
		willSkipChildren: false,
	}, nil
}

type iter1 struct {
	root             Root
	stack            stack.Stack[*treeIterNode]
	current          *treeIterNode
	willSkipChildren bool
}

func (iter iter1) CanGet() bool {
	return !iter.stack.IsEmpty()
}
func (iter iter1) Get() (p string, depth uint32, vs VisitStatus, t Tree) {
	if !iter.CanGet() {
		panic("Get called with CanGet() = false")
	}
	t = iter.current
	p = iter.current.treePath
	depth = iter.current.treePathDepth
	vs = iter.current.s
	return
}
func (iter *iter1) SkipChildrenOnNext() {
	iter.willSkipChildren = true
}

// doesnt recurse, performs one step.
func (iter *iter1) Next() (err error) {
	defer func() {
		if err == nil {
			iter.willSkipChildren = false
		}
	}()
	_, _, st, _ := iter.Get()

	// If there are no children, or this is the second visit, or we're
	// skipping them on purpose, just pop from the stack, update the current
	// node and compute its data if possible
	nChildren := len(iter.current.data.ChildrenBaseNames)
	if nChildren == 0 || iter.willSkipChildren || st == SecondVisit {
		iter.stack.Pop()
		if iter.stack.IsEmpty() {
			iter.current = nil
			return nil
		}
		iter.current = *iter.stack.Peek()
		if !iter.current.data.IsDir || iter.current.s == SecondVisit {
			iter.current.computeDataIfPossibleAndNeeded()
		}
		return nil
	}
	// Else, add the children to the execution stack and update the current
	err = iter.current.addChildren(iter.root, iter.stack)
	if err != nil {
		return
	}
	iter.current = *iter.stack.Peek()
	return nil
}

type treeIterNode struct {
	treePath              string
	treePathDepth         uint32
	fg                    fileGetter
	s                     VisitStatus
	children              []*treeIterNode // Populated lazily
	isRemovedChild        bool
	data                  Data
	dataIsComplete        bool
	dataWasReadFromParent bool

	addChildrenWasCalled bool // just used for panics
}

func newTreeIterNode(treePath string,
	treePathDepth uint32,
	fg fileGetter,
	d Data, dataIsComplete, dataWasReadFromParent bool) *treeIterNode {
	node := &treeIterNode{
		treePath:              treePath,
		treePathDepth:         treePathDepth,
		fg:                    fg,
		s:                     FirstVisit,
		addChildrenWasCalled:  false,
		isRemovedChild:        false,
		data:                  d,
		dataIsComplete:        dataIsComplete,
		dataWasReadFromParent: dataWasReadFromParent,
	}
	if dataIsComplete || len(d.ChildrenBaseNames) == 0 {
		node.computeDataIfPossibleAndNeeded()
	}
	return node
}

func newRemovedChildNode(treePath string) *treeIterNode {
	var depth uint32
	if treePath == RootPath {
		depth = 0
	} else {
		depth = uint32(strings.Count(treePath, "/") + 1)
	}
	return &treeIterNode{
		treePath:       treePath,
		treePathDepth:  depth,
		isRemovedChild: true,
		dataIsComplete: true}
}

func (node *treeIterNode) addChildren(root Root,
	stack stack.Stack[*treeIterNode]) error {
	if node.s != FirstVisit {
		panic("tried to add children on second visit")
	}
	if node.addChildrenWasCalled {
		panic("addChildrenWasCalled already called")
	}
	if !node.data.IsDir {
		panic("tried to add children from file")
	}
	// If the node has children, we'll need to read the children from disk
	// or from its ChildrenData if available.
	// If the node's data was read from the parent but the node itself doesn't
	// have the children data, there's a high change that the node only doesn't
	// have the children data because it was manually removed to prevent the
	// struct from growing ubounded. Thus, it's probably a good idea to fetch
	// whole node from disk again as there's a high change that it will contain
	// all the children, which is better than reading the children one by one.
	if len(node.data.ChildrenBaseNames) > 0 &&
		node.dataWasReadFromParent &&
		!node.data.HasChildrenData {
		// The root must be able to provide all the trees, including the ones
		// pre-fetched. However, we can handle this case in which it doesn't
		// and returns ErrTreeNotFound. We can just keep on going and we'll
		// read the child trees
		tr, err := root.Tree(node.treePath)
		if err != nil && !errors.Is(err, ErrTreeNotFound) {
			return err
		}
		if !errors.Is(err, ErrTreeNotFound) {
			node.data = tr.Data()
		}
	}

	node.children = make([]*treeIterNode, 0, len(node.data.ChildrenBaseNames))
	if node.data.HasChildrenData {
		for i := range node.data.ChildrenData {
			childTreePath := path.Join(node.treePath, node.data.ChildrenData[i].BaseName)
			node.children = append(node.children,
				newTreeIterNode(childTreePath,
					node.data.ChildrenData[i].Depth,
					newFileGetterFromRootAndPath(root, childTreePath),
					node.data.ChildrenData[i],
					node.data.ChildrenDataIsComplete[i], true))
		}
	} else {
		for _, childName := range node.data.ChildrenBaseNames {
			childTreePath := path.Join(node.treePath, childName)
			childTree, err := root.Tree(childTreePath)
			if err != nil {
				return err
			}
			if childTree.IsRemovedChild() {
				node.children = append(node.children, newRemovedChildNode(
					childTreePath,
				))
				continue
			}
			checkTreeOrDie(childTree)
			if childTree.Data().Depth != node.data.Depth+1 {
				panic(fmt.Sprintf(
					"child %q of %q has depth %d but should have %d",
					childTreePath, node.treePath,
					childTree.Data().Depth, node.data.Depth+1))
			}
			node.children = append(node.children,
				newTreeIterNode(childTreePath,
					childTree.Data().Depth, newFileGetterFromTree(childTree),
					childTree.Data(), childTree.DataIsComplete(),
					false))
		}
	}
	// Since we use a stack, push in reverse order
	for i := len(node.children) - 1; i >= 0; i-- {
		stack.Push(node.children[i])
	}
	// Next time this node appears, it'll be its second visit
	node.s = SecondVisit
	node.addChildrenWasCalled = true
	return nil
}

func (node *treeIterNode) computeDataIfPossibleAndNeeded() {
	if node.dataIsComplete {
		return
	}
	if !node.data.IsDir {
		panic("got file with non complete data")
	}
	// Check if all the children are complete. Else we can't compute this one.
	for _, child := range node.children {
		if !child.dataIsComplete {
			return
		}
	}
	defer func() {
		// Relese the children when done
		node.children = nil
	}()

	// Update the ChildrenNames in the metadata. In some cases (rebase),
	// the "non-complete" data will contain name of some childrens that we'll
	// later discover are not really children (see ErrChildTreeNotFound).
	// Also filter out child directories that have size 0.
	node.data.ChildrenBaseNames = node.data.ChildrenBaseNames[:0]
	node.data.HasChildrenData = false
	node.data.ChildrenData = node.data.ChildrenData[:0]
	node.data.ChildrenDataIsComplete = node.data.ChildrenDataIsComplete[:0]
	originalChildren := node.children
	filteredChildren := originalChildren[:0]
	allChildrenAreRemovedChild := true
	for _, child := range node.children {
		if isEmptyDir(child.data) {
			continue
		}
		if child.isRemovedChild {
			continue
		}
		if !node.isRemovedChild {
			allChildrenAreRemovedChild = false
		}
		node.data.ChildrenBaseNames = append(
			node.data.ChildrenBaseNames,
			child.data.BaseName)
		filteredChildren = append(filteredChildren, child)
		// Flatten the child data to prevent a directory struct from growing
		// indefinitely
		child.data.HasChildrenData = false
		child.data.ChildrenData = nil
		child.data.ChildrenDataIsComplete = nil
		// Add the flat child data to the node
		node.data.HasChildrenData = true
		node.data.ChildrenDataIsComplete = append(
			node.data.ChildrenDataIsComplete, child.dataIsComplete)
		node.data.ChildrenData = append(
			node.data.ChildrenData, child.data)
	}
	if allChildrenAreRemovedChild && node.treePathDepth != 0 {
		node.isRemovedChild = true
		node.dataIsComplete = true
		return
	}

	// Compute data from the children
	hash := NewHasher()
	for _, child := range filteredChildren {
		node.data.Size += child.data.Size
		hash.WriteString(child.data.BaseName)
		hash.WriteSum(child.data.ContentHash)
		node.data.LastModifiedUnixMillis = max(
			node.data.LastModifiedUnixMillis,
			child.data.LastModifiedUnixMillis)
		if child.data.HasChildWithConflicts ||
			child.data.HasConflicts {
			node.data.HasChildWithConflicts = true
		}
	}
	node.data.ContentHash = hash.Sum()
	node.dataIsComplete = true
}
func (node treeIterNode) IsRemovedChild() bool {
	return node.isRemovedChild
}
func (node treeIterNode) DataIsComplete() bool {
	if node.IsRemovedChild() {
		panic("called DataIsComplete() on IsRemovedChild")
	}
	return node.dataIsComplete
}
func (node treeIterNode) Data() Data {
	if node.IsRemovedChild() {
		panic("called Data() on IsRemovedChild")
	}
	return node.data
}
func (node treeIterNode) GetFile() (wt io.WriterTo, err error) {
	if node.IsRemovedChild() {
		panic("called GetFile() on IsRemovedChild")
	}
	return node.fg.GetFile()
}

var emptyHashVal = NewHasher().Sum()

func isEmptyDir(d Data) bool {
	return d.IsDir && d.Size == 0 && d.ContentHash == emptyHashVal
}

// Function to validate that the tree implementation satifies the requirements
// that we expect
func checkTreeOrDie(tr Tree) {
	if !tr.Data().IsDir && !tr.DataIsComplete() {
		panic("got non fully-computed file")
	}
}

type fileGetter struct {
	r    Root
	path string
	tr   Tree
}

func newNullFileGetter() fileGetter {
	return fileGetter{}
}
func newFileGetterFromRootAndPath(r Root, filePath string) fileGetter {
	return fileGetter{r: r, path: filePath}
}
func newFileGetterFromTree(tr Tree) fileGetter {
	return fileGetter{tr: tr}
}
func (fg fileGetter) GetFile() (io.WriterTo, error) {
	if fg.tr != nil {
		return fg.tr.GetFile()
	}
	if fg.r == nil {
		return nil, errors.New("tried to get null file getter")
	}
	tr, err := fg.r.Tree(fg.path)
	if err != nil {
		return nil, err
	}
	return tr.GetFile()
}