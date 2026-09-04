package treev

import "monorepo/twigg/tree"

// The version control stores versions of trees (files/directories).
// Thus, a tree is uniquely identified by it's path and a version.
// The TreeDataV (TreeData + Version) simply associates a treeData with a
// version for its blob (if it's a file) and children (it it's a directory).
type TreeDataV struct {
	Data             tree.Data
	ChildrenVersions []uint64
	BlobVersion      uint64
}
