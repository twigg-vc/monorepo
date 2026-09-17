package review

import (
	"regexp"
	"strings"
)

// Matches "wip", "WIP", "wip:" and "wip fix", but not "wip1" or "wipFix".
var wipMessageRegex = regexp.MustCompile(`(?i)^wip($|[^a-z0-9])`)

const archivedMessagePrefix = "#ARCHIVED"

func messageIsWip(message string) bool {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return false
	}
	return wipMessageRegex.MatchString(trimmed)
}

func messageIsArchived(message string) bool {
	return strings.HasPrefix(message, archivedMessagePrefix)
}
