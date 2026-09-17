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

func Test_ParseQuery_FailsOnASearchItCanNotRun(t *testing.T) {
	cases := []string{
		// text can not be excluded from a search
		`-queue`,
		`-"a b"`,
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
