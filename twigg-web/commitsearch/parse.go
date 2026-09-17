package commitsearch

import (
	"fmt"
	"monorepo/twigg-web/review"
	"strings"
)

const maxQueryLength = 500

const (
	keyIs       = "is"
	keyAuthor   = "author"
	keyReviewer = "reviewer"
	keyMessage  = "message"
)

var queryTokenizer = NewTokenizer(
	[]string{keyIs, keyAuthor, keyReviewer, keyMessage},
	maxQueryLength)

// The review statuses a commit can be searched by.
var searchedReviewStatuses = map[string]review.ReviewStatus{
	"ready":                   review.ReviewStatus_Ready,
	"lgtm":                    review.ReviewStatus_Ready,
	"missing-lgtm":            review.ReviewStatus_MissingLgtm,
	"no-lgtm":                 review.ReviewStatus_MissingLgtm,
	"unresolved":              review.ReviewStatus_Unresolved,
	"missing-owners-approval": review.ReviewStatus_MissingOwnersApproval,
}

func parseQuery(repoId uint64, q string) (f Filter, err error) {
	f = Filter{RepoId: repoId}
	tokens, err := queryTokenizer.Parse(q)
	if err != nil {
		return f, err
	}
	messageValues := []string{}
	for _, t := range tokens {
		// empty keys are equivalent to message
		if t.Key == "" {
			if t.IsNegated {
				return f, notExcludableErr(t.Value)
			}
			t.Key = keyMessage
		}
		err = applyFilter(&f, &messageValues, t)
		if err != nil {
			return f, err
		}
	}
	f.Message = strings.Join(messageValues, " ")
	// Unless specified by the query, archived commits are hidden by default
	if f.Archived == PresenceIgnore {
		f.Archived = PresenceExclude
	}
	return f, nil
}

// A term given twice keeps the last value, which is what a search bar that
// appends a term to what is already written needs.
func applyFilter(f *Filter, messageValues *[]string, t Token) error {
	if t.Key == keyIs {
		return applyIsFilter(f, t)
	}
	// The following keys can't be negated
	if t.IsNegated {
		return notExcludableErr(t.Key)
	}
	switch t.Key {
	case keyMessage:
		*messageValues = append(*messageValues, t.Value)
	case keyAuthor:
		f.AuthorUsername = t.Value
	case keyReviewer:
		f.ReviewerUsername = t.Value
	default:
		panic(fmt.Sprintf("the tokenizer read the unknown key %q", t.Key))
	}
	return nil
}

func applyIsFilter(f *Filter, t Token) error {
	presence := PresenceRequire
	if t.IsNegated {
		presence = PresenceExclude
	}
	value := strings.ToLower(t.Value)

	// Handle "is:wip" and "is:archived"
	switch value {
	case "wip":
		f.Wip = presence
		return nil
	case "archived":
		f.Archived = presence
		return nil
	}

	// Handle review status: `is:ready`, `is:missing-lgtm`, etc
	if status, isStatus := searchedReviewStatuses[value]; isStatus {
		if t.IsNegated {
			return notExcludableErr(value)
		}
		f.HasReviewStatus = true
		f.ReviewStatus = status
		return nil
	}

	// Handle state: `is:pending`, `is:submitted`
	switch value {
	case "pending":
		if t.IsNegated {
			return notExcludableErr(value)
		}
		f.State = StatePending
		return nil
	case "submitted":
		if t.IsNegated {
			return notExcludableErr(value)
		}
		f.State = StateSubmitted
		return nil
	}

	// Handle bad value
	return fmt.Errorf("%q is not something a commit can be", t.Value)
}

func notExcludableErr(what string) error {
	return fmt.Errorf("%q can not be excluded from a search", what)
}