// Package commitsearch holds the entities of a commit search.
package commitsearch

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
	RepoId   uint64
	Message  string
	State    State
	Wip      Presence
	Archived Presence
}

// Returns a filter that matches every non-archived commit of a repo.
func NewFilter(repoId uint64) Filter {
	return Filter{
		RepoId:   repoId,
		Archived: PresenceExclude,
	}
}
