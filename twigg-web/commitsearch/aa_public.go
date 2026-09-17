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

// Filter of a commit search. Each optional field is ignored when it holds
// its zero value.
type Filter struct {
	RepoId uint64
	// Only returns commits older than AfterCommitId. Used to paginate.
	HasAfterCommitId bool
	AfterCommitId    commit.LocalId
	// Max number of commits returned
	Limit int
}

// Returns a filter that matches every commit of a repo.
func NewFilter(repoId uint64, limit int) Filter {
	return Filter{RepoId: repoId, Limit: limit}
}
