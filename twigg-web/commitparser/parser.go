package commitparser

import (
	"regexp"
	"strconv"
	"strings"
)

const maxDescriptionLenToParse = 10_000

// Matches a whole line (after trimming) like "b/1234", "BUG=1234", "bug= 1234" or "BUG: 1234"
var bugTagRe = regexp.MustCompile(`(?i)^(?:b/|bug[=:]\s*)(\d+)$`)

func parseCommitDescription(desc string) Tags {
	if len(desc) > maxDescriptionLenToParse {
		return Tags{}
	}
	bugs := newBugNumbers()
	for _, line := range strings.Split(desc, "\n") {
		line = strings.TrimSpace(line)
		bugs.addMatch(bugTagRe.FindStringSubmatch(line))
	}
	return Tags{Bugs: bugs.numbers}
}

type bugNumbers struct {
	numbers []int64
	seen    map[int64]bool
}

func newBugNumbers() *bugNumbers {
	return &bugNumbers{seen: map[int64]bool{}}
}

func (b *bugNumbers) addMatch(match []string) {
	if match == nil {
		return
	}
	bugNumber, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil || b.seen[bugNumber] {
		return
	}
	b.seen[bugNumber] = true
	b.numbers = append(b.numbers, bugNumber)
}
