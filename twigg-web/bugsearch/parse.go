package bugsearch

import (
	"fmt"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/commitsearch"
	"strings"
)

const maxQueryLength = 500

const (
	keyIs       = "is"
	keyAssignee = "assignee"
	keyPriority = "priority"
)

var queryTokenizer = commitsearch.NewTokenizer(
	[]string{keyIs, keyAssignee, keyPriority},
	maxQueryLength)

func parseQuery(repoId uint64, q string) (f Filter, err error) {
	f = Filter{RepoId: repoId}
	tokens, err := queryTokenizer.Parse(q)
	if err != nil {
		return f, err
	}
	for _, t := range tokens {
		// There is no free-text search yet, so a bare word has nothing to do.
		if t.Key == "" {
			return f, fmt.Errorf(
				`%q is free text, which can not be searched yet: use "is:", "assignee:" or "priority:"`,
				t.Value)
		}
		err = applyFilter(&f, t)
		if err != nil {
			return f, err
		}
	}
	return f, nil
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
	case keyPriority:
		return applyPriorityFilter(f, t.Value)
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

func applyPriorityFilter(f *Filter, value string) error {
	p := bug.Priority(strings.ToUpper(value))
	if strings.EqualFold(value, "none") {
		p = bug.Priority_None
	} else if p == bug.Priority_None || !p.IsValid() {
		return fmt.Errorf("%q is not a priority", value)
	}
	f.HasPriority = true
	f.Priority = p
	return nil
}

func notExcludableErr(what string) error {
	return fmt.Errorf("%q can not be excluded from a search", what)
}
