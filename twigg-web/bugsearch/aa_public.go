package bugsearch

import (
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/commitsearch"
)

type Filter struct {
	RepoId uint64
	// "" matches a bug of any status.
	Status bug.Status
	// Username of the bug's assignee. Ignored when AssigneeUnassigned is
	// true. Empty matches a bug with any assignee (or none), and a username
	// nobody has matches no bug.
	AssigneeUsername string
	// When true, only unassigned bugs match, and AssigneeUsername is ignored.
	AssigneeUnassigned bool
}

// Returns a filter that matches every bug of a repo.
func NewFilter(repoId uint64) Filter {
	return Filter{RepoId: repoId}
}

// The username that stands for whoever is searching. The caller replaces it
// with their own, since only it knows who is logged in.
const MeUsername = commitsearch.MeUsername

// Parses what was typed in a search bar, such as `is:open assignee:me`.
// Returns an error whose text is meant to be shown to the user.
func ParseQuery(repoId uint64, q string) (Filter, error) {
	return parseQuery(repoId, q)
}
