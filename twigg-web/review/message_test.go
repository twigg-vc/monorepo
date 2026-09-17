package review_test

import (
	"monorepo/twigg-web/review"
	"testing"
)

func Test_MessageIsWip(t *testing.T) {
	cases := []struct {
		message string
		isWip   bool
	}{
		{"wip", true},
		{"WIP", true},
		{"wip: refactor the queue", true},
		{"  wip refactor  ", true},
		{"wip1", false},
		{"wipFix", false},
		{"swip", false},
		{"   ", false},
		{"Define user entity", false},
	}
	for _, c := range cases {
		if got := review.MessageIsWip(c.message); got != c.isWip {
			t.Fatalf("MessageIsWip(%q)=%v, want %v", c.message, got, c.isWip)
		}
	}
}

func Test_MessageIsArchived(t *testing.T) {
	cases := []struct {
		message    string
		isArchived bool
	}{
		{review.ArchivedMessagePrefix, true},
		{review.ArchivedMessagePrefix + " old attempt", true},
		{" #ARCHIVED leading space", false},
		{"Define user entity", false},
		{"", false},
	}
	for _, c := range cases {
		if got := review.MessageIsArchived(c.message); got != c.isArchived {
			t.Fatalf("MessageIsArchived(%q)=%v, want %v",
				c.message, got, c.isArchived)
		}
	}
}
