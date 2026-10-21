package commitparser

import (
	"regexp"
	"strconv"
	"strings"
)

const maxDescriptionLenToParse = 10_000

// Matches a whole line (after trimming), case-insensitively:
//   - "b/1234"
//   - "BUG=1234" or "BUG:1234"
//   - "BUG=b/1234" or "BUG:b/1234"
//
// Whitespace is allowed after "=" or ":".
var bugTagRe = regexp.MustCompile(`(?i)^(?:b/|bug[=:]\s*(?:b/)?)(\d+)$`)

// Matches a whole line (after trimming), case-insensitively:
//   - "<KEYWORD>=1234" or "<KEYWORD>:1234"
//   - "<KEYWORD>=b/1234" or "<KEYWORD>:b/1234"
//
// where <KEYWORD> is FIX, FIXES, FIXED, CLOSE, CLOSES or CLOSED. Whitespace is
// allowed after "=" or ":".
var fixesTagRe = regexp.MustCompile(`(?i)^(?:fix(?:es|ed)?|close[sd]?)[=:]\s*(?:b/)?(\d+)$`)

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
