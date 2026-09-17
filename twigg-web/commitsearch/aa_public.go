// Package commitsearch holds the entities of a commit search.
package commitsearch

import "monorepo/twigg-web/review"

// Whether the search covers pending commits, submitted commits or both.
type State string

const (
	StateAny       State = ""
	StatePending   State = "pending"
	StateSubmitted State = "submitted"
)

// Whether a search requires, excludes or ignores a commit trait.
type Presence string

const (
	PresenceIgnore  Presence = ""
	PresenceRequire Presence = "require"
	PresenceExclude Presence = "exclude"
)

// Filter of a commit search. Each optional field is ignored when it holds
// its zero value.
type Filter struct {
	RepoId  uint64
	Message string
	// Username of the commit author. Empty matches any author, and a
	// username nobody has matches no commit.
	AuthorUsername string
	// Username of a user in the commit reviewers. Empty matches any
	// reviewer, and a username nobody has matches no commit.
	ReviewerUsername string
	// Only applies when HasReviewStatus is true. A submitted commit has no
	// review status, so it only ever matches pending commits.
	HasReviewStatus bool
	ReviewStatus    review.ReviewStatus
	State           State
	Wip             Presence
	Archived        Presence
}

// Returns a filter that matches every non-archived commit of a repo.
func NewFilter(repoId uint64) Filter {
	return Filter{
		RepoId:   repoId,
		Archived: PresenceExclude,
	}
}