package commitparser

import (
	"regexp"
	"strconv"
	"strings"
)

const maxDescriptionLenToParse = 10_000

// Matches a whole line (after trimming) like "b/1234" or "BUG=1234"
var bugTagRe = regexp.MustCompile(`(?i)^(?:b/|bug=)(\d+)$`)

func parseCommitDescription(desc string) Tags {
	if len(desc) > maxDescriptionLenToParse {
		return Tags{}
	}
	var bugs []int64
	seen := map[int64]bool{}
	for _, line := range strings.Split(desc, "\n") {
		match := bugTagRe.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil {
			continue
		}
		bugNumber, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil || seen[bugNumber] {
			continue
		}
		seen[bugNumber] = true
		bugs = append(bugs, bugNumber)
	}
	return Tags{Bugs: bugs}
}