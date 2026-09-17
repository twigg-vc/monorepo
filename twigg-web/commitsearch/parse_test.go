package commitsearch_test

import (
	"monorepo/twigg-web/commitsearch"
	"testing"
)

const parsedRepoId = 7

func Test_ParseQuery_ReadsTheWordsAsTheMessage(t *testing.T) {
	cases := []struct{ query, message string }{
		{"", ""},
		{"queue", "queue"},
		{"  refactor the   queue ", "refactor the queue"},
		{`"refactor the queue"`, "refactor the queue"},
		{`refactor "the queue"`, "refactor the queue"},
		// A quoted token is text, which is how a colon is searched for
		{`"is: refactor"`, "is: refactor"},
		// message: adds to the words, so a phrase and words can be mixed
		{`message:"a b" c`, "a b c"},
	}
	for _, c := range cases {
		f, err := commitsearch.ParseQuery(parsedRepoId, c.query)
		if err != nil {
			t.Fatalf("parsing %q failed: %s", c.query, err)
		}
		if f.Message != c.message {
			t.Fatalf("parsing %q read the message %q, want %q",
				c.query, f.Message, c.message)
		}
		if f.RepoId != parsedRepoId {
			t.Fatalf("parsing %q read the repo %d, want %d",
				c.query, f.RepoId, parsedRepoId)
		}
	}
}

func Test_ParseQuery_HidesArchivedCommits(t *testing.T) {
	f, err := commitsearch.ParseQuery(parsedRepoId, "queue")

	if err != nil {
		t.Fatal(err)
	}
	if f.Archived != commitsearch.PresenceExclude {
		t.Fatalf("archived is %q, want %q", f.Archived,
			commitsearch.PresenceExclude)
	}
}

func Test_ParseQuery_ReadsTheAuthorAndTheReviewer(t *testing.T) {
	f, err := commitsearch.ParseQuery(parsedRepoId,
		"author:me reviewer:aang queue")

	if err != nil {
		t.Fatalf("parsing failed: %s", err)
	}
	if f.AuthorUsername != commitsearch.MeUsername {
		t.Fatalf("read the author %q, want %q", f.AuthorUsername,
			commitsearch.MeUsername)
	}
	if f.ReviewerUsername != "aang" {
		t.Fatalf("read the reviewer %q, want aang", f.ReviewerUsername)
	}
	if f.Message != "queue" {
		t.Fatalf("read the message %q, want queue", f.Message)
	}
}

// A search bar appends a term to what is already written, so the last one
// wins instead of the search contradicting itself.
func Test_ParseQuery_KeepsTheLastValueOfARepeatedTerm(t *testing.T) {
	f, err := commitsearch.ParseQuery(parsedRepoId,
		"author:aang author:katara")

	if err != nil {
		t.Fatalf("parsing failed: %s", err)
	}
	if f.AuthorUsername != "katara" {
		t.Fatalf("read the author %q, want katara", f.AuthorUsername)
	}
}

func Test_ParseQuery_FailsOnASearchItCanNotRun(t *testing.T) {
	cases := []string{
		// text can not be excluded from a search
		`-queue`,
		`-"a b"`,
		`-author:aang`,
		`-message:queue`,
		// the tokenizer refuses these before the words are read
		`nope:1`,
		`an "unclosed quote`,
	}
	for _, query := range cases {
		if _, err := commitsearch.ParseQuery(parsedRepoId, query); err == nil {
			t.Fatalf("parsing %q did not fail", query)
		}
	}
}