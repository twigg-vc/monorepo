package bugsearch

import (
	"fmt"
	"monorepo/twigg-web/bug"
	"strings"
)

func parseQuery(repoId uint64, q string) (f Filter, err error) {
	return Filter{RepoId: repoId}, nil
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