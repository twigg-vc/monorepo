package commitsearch

import (
	"fmt"
	"strings"
)

const maxQueryLength = 500

const (
	keyAuthor   = "author"
	keyReviewer = "reviewer"
	keyMessage  = "message"
)

var queryTokenizer = NewTokenizer(
	[]string{keyAuthor, keyReviewer, keyMessage}, maxQueryLength)

func parseQuery(repoId uint64, q string) (f Filter, err error) {
	f = NewFilter(repoId)
	tokens, err := queryTokenizer.Parse(q)
	if err != nil {
		return f, err
	}
	words := []string{}
	for _, t := range tokens {
		if t.IsNegated {
			return f, notExcludableErr(t.Value)
		}
		if t.Key == "" {
			words = append(words, t.Value)
			continue
		}
		applyTerm(&f, &words, t)
	}
	f.Message = strings.Join(words, " ")
	return f, nil
}

// A term given twice keeps the last value, which is what a search bar that
// appends a term to what is already written needs.
func applyTerm(f *Filter, words *[]string, t Token) {
	switch t.Key {
	case keyMessage:
		*words = append(*words, t.Value)
	case keyAuthor:
		f.AuthorUsername = t.Value
	case keyReviewer:
		f.ReviewerUsername = t.Value
	default:
		panic(fmt.Sprintf("the tokenizer read the unknown key %q", t.Key))
	}
}

func notExcludableErr(what string) error {
	return fmt.Errorf("%q can not be excluded from a search", what)
}