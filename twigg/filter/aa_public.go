package filter

import "monorepo/twigg/tree"

// The filtered tree has the values from `tr` for paths and subpaths provided.
// It has the values of base in all others. This method flattens the trees
// provided by the Root, which might cause Root to be queried more often.
func Filter(tr tree.Root, paths []string, base tree.Root) tree.Root {
	return filter(tr, paths, base)
}
