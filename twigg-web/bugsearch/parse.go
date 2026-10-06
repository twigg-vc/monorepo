package bugsearch

import (
	"fmt"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/commitsearch"
	"strings"
)

const (
	keyIs       = "is"
	keyAssignee = "assignee"
)

func parseQuery(repoId uint64, q string) (f Filter, err error) {
	return Filter{RepoId: repoId}, nil
}

// A term given twice keeps the last value, which is what a search bar that
// appends a term to what is already written needs.
func applyFilter(f *Filter, t commitsearch.Token) error {
	if t.IsNegated {
		return notExcludableErr(t.Key)
	}
	switch t.Key {
	case keyIs:
		return applyIsFilter(f, t.Value)
	case keyAssignee:
		applyAssigneeFilter(f, t.Value)
	default:
		panic(fmt.Sprintf("the tokenizer read the unknown key %q", t.Key))
	}
	return nil
}

func applyIsFilter(f *Filter, value string) error {
	switch strings.ToLower(value) {
	case "open":
		f.Status = bug.Status_Open
		return nil
	case "closed":
		f.Status = bug.Status_Closed
		return nil
	}
	return fmt.Errorf("%q is not something a bug can be", value)
}

func applyAssigneeFilter(f *Filter, value string) {
	switch strings.ToLower(value) {
	case "unassigned", "none":
		f.AssigneeUnassigned = true
		f.AssigneeUsername = ""
	default:
		f.AssigneeUnassigned = false
		f.AssigneeUsername = value
	}
}

func notExcludableErr(what string) error {
	return fmt.Errorf("%q can not be excluded from a search", what)
}