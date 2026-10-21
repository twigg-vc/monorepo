package commitparser

import (
	"regexp"
	"strconv"
	"strings"
)

const maxDescriptionLenToParse = 10_000

// Matches a whole line (after trimming) like "b/1234", "BUG=1234", "bug= 1234" or "BUG: 1234"
var bugTagRe = regexp.MustCompile(`(?i)^(?:b/|bug[=:]\s*)(\d+)$`)

// Matches a whole line (after trimming) like "FIXES=1234", "fixed: 1234" or "CLOSE=1234"
var fixesTagRe = regexp.MustCompile(`(?i)^(?:fix(?:es|ed)?|close[sd]?)[=:]\s*(\d+)$`)

func parseCommitDescription(desc string) Tags {
	if len(desc) > maxDescriptionLenToParse {
		return Tags{}
	}
	bugs := newBugNumbers()
	fixes := newBugNumbers()
	for _, line := range strings.Split(desc, "\n") {
		line = strings.TrimSpace(line)
		bugs.addMatch(bugTagRe.FindStringSubmatch(line))
		fixes.addMatch(fixesTagRe.FindStringSubmatch(line))
	}
	return Tags{Bugs: bugs.numbers, Fixes: fixes.numbers}
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
