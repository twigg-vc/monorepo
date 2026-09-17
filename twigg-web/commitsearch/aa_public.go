// Package commitsearch holds the entities of a commit search.
package commitsearch

import "monorepo/twigg/commit"

// How far an index sweep got. Repo ids are autoincremented, so no commit has
// repo id 0 and the zero value starts a new sweep.
type IndexCursor struct {
	RepoId   uint64
	CommitId commit.LocalId
}

// Returns the cursor of a sweep that starts at the first commit.
func NewIndexCursor() IndexCursor {
	return IndexCursor{}
}
