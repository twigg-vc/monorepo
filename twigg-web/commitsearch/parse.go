package commitsearch

import (
	"fmt"
	"strings"
)

const maxQueryLength = 500

var queryTokenizer = NewTokenizer(nil, maxQueryLength)

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
		words = append(words, t.Value)
	}
	f.Message = strings.Join(words, " ")
	return f, nil
}

func notExcludableErr(what string) error {
	return fmt.Errorf("%q can not be excluded from a search", what)
}
