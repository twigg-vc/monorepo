package filter

import (
	"errors"
	"io"
	"monorepo/twigg/tree"
	"path"
	"strings"
)

func filter(tr tree.Root, paths []string, base tree.Root) tree.Root {
	return &filteredRoot{tr: tr, base: base, paths: paths}
}

// filtered is a Root implementation that delegates to tr or base
// depending on whether the path matches one of the given prefixes.
type filteredRoot struct {
	tr    tree.Root
	base  tree.Root
	paths []string
}

func (f *filteredRoot) Tree(path string) (tr tree.Tree, err error) {
	defer func() {
		tr = tree.Flatten(tr)
	}()
	// First check it tr exist
	tr, err = f.tr.Tree(path)
	if err != nil && !errors.Is(err, tree.ErrTreeNotFound) {
		return
	}
	hasTr := !errors.Is(err, tree.ErrTreeNotFound) && !tr.IsRemovedChild()
	if hasTr {
		isIncludedInFilter := false
		for _, filterPath := range f.paths {
			if hasCommonPrefixOrIsRoot(path, filterPath) {
				isIncludedInFilter = true
				break
			}
		}
		hasTr = isIncludedInFilter
	}
	if !hasTr {
		// If it doesn't exist, just return base
		tr, err = f.base.Tree(path)
		return
	}
	// If it exists, filter its children
	var filteredTr tree.Tree
	trChildren := tr.Data().ChildrenBaseNames
	filteredTrChildren, removedSomething := filterChildren(path, trChildren, f.paths)
	if removedSomething {
		filteredTr = filteredDir{
			t: tr,
			d: tree.NewUnknownDirData(
				tr.Data().BaseName,
				tr.Data().Depth,
				filteredTrChildren),
		}
	} else {
		filteredTr = tr
	}

	// Now check the base
	base, err := f.base.Tree(path)
	if err != nil && !errors.Is(err, tree.ErrTreeNotFound) {
		return
	}
	hasBase := !errors.Is(err, tree.ErrTreeNotFound) && !base.IsRemovedChild()
	if !hasBase {
		// If there is no base, return the filtered tr
		tr = filteredTr
		err = nil
		return
	}

	// In the case that both exist, we must merge the trees
	tr = newMergeTree(filteredTr, base)
	err = nil
	return
}

func hasCommonPrefixOrIsRoot(a, b string) bool {
	if a == tree.RootPath || b == tree.RootPath {
		return true
	}
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}

func newMergeTree(ft tree.Tree, base tree.Tree) tree.Tree {
	if !ft.Data().IsDir {
		return ft
	}
	if !base.Data().IsDir {
		return base
	}
	// If they're the same, just return any
	if ft.DataIsComplete() && base.DataIsComplete() && tree.IsEqual(ft, base) {
		return ft
	}

	nFc := len(ft.Data().ChildrenBaseNames)
	fChildren := ft.Data().ChildrenBaseNames
	nBc := len(base.Data().ChildrenBaseNames)
	baseChildren := base.Data().ChildrenBaseNames
	unionOfChildren := make([]string, 0, max(nFc, nBc))

	f := 0
	canGetF := func() bool { return f < len(fChildren) }
	b := 0
	canGetB := func() bool { return b < len(baseChildren) }
	fChild := ""
	baseChild := ""
	for canGetF() || canGetB() {
		if canGetF() && fChild == "" {
			fChild = fChildren[f]
		}
		if canGetB() && baseChild == "" {
			baseChild = baseChildren[b]
		}

		if fChild == "" {
			// Check to avoid adding duplicated entry
			if len(unionOfChildren) == 0 ||
				unionOfChildren[len(unionOfChildren)-1] != baseChild {
				unionOfChildren = append(unionOfChildren, baseChild)
			}
			b++
			baseChild = ""
			continue
		}
		if baseChild == "" {
			// Check to avoid adding duplicated entry
			if len(unionOfChildren) == 0 ||
				unionOfChildren[len(unionOfChildren)-1] != fChild {
				unionOfChildren = append(unionOfChildren, fChild)
			}
			f++
			fChild = ""
			continue
		}
		if baseChild == fChild {
			unionOfChildren = append(unionOfChildren, baseChild)
			f++
			fChild = ""
			b++
			baseChild = ""
			continue
		}

		if baseChild < fChild {
			unionOfChildren = append(unionOfChildren, baseChild)
			b++
			baseChild = ""
			continue
		}
		unionOfChildren = append(unionOfChildren, fChild)
		f++
		fChild = ""
	}

	return mergedDir{baseName: ft.Data().BaseName, children: unionOfChildren, depth: ft.Data().Depth}
}

type mergedDir struct {
	baseName string
	depth    uint32
	children []string
}

func (m mergedDir) IsRemovedChild() bool {
	return false
}
func (m mergedDir) DataIsComplete() bool {
	return false
}
func (m mergedDir) Data() tree.Data {
	return tree.NewUnknownDirData(m.baseName, m.depth, m.children)
}
func (m mergedDir) GetFile() (wt io.WriterTo, err error) {
	panic("called GetFile on directory tree")
}

// Filters a tree children based on the filter paths.
// Returns true if something was removed
func filterChildren(treePath string, children []string, filterPaths []string) ([]string, bool) {
	removed := false
	filtered := make([]string, 0, len(children))
	for _, childName := range children {
		addChild := false
		for _, filterPath := range filterPaths {
			if hasCommonPrefixOrIsRoot(filterPath, path.Join(treePath, childName)) {
				addChild = true
				break
			}
		}
		if !addChild {
			removed = true
			continue
		}

		filtered = append(filtered, childName)
	}
	return filtered, removed
}

type filteredDir struct {
	t tree.Tree
	d tree.Data
}

func (f filteredDir) IsRemovedChild() bool {
	return false
}
func (f filteredDir) DataIsComplete() bool {
	return false
}
func (f filteredDir) Data() tree.Data {
	return f.d
}
func (f filteredDir) GetFile() (wt io.WriterTo, err error) {
	return f.t.GetFile()
}
