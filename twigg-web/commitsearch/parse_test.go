package commitsearch_test

import (
	"monorepo/twigg-web/commitsearch"
	"monorepo/twigg-web/review"
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

func Test_ParseQuery_HidesArchivedCommitsUnlessAsked(t *testing.T) {
	cases := map[string]commitsearch.Presence{
		"queue":        commitsearch.PresenceExclude,
		"is:archived":  commitsearch.PresenceRequire,
		"-is:archived": commitsearch.PresenceExclude,
	}
	for query, want := range cases {
		f, err := commitsearch.ParseQuery(parsedRepoId, query)
		if err != nil {
			t.Fatalf("parsing %q failed: %s", query, err)
		}
		if f.Archived != want {
			t.Fatalf("parsing %q read archived %q, want %q",
				query, f.Archived, want)
		}
	}
}

func Test_ParseQuery_ReadsTheStateTerms(t *testing.T) {
	cases := map[string]commitsearch.State{
		"is:pending":   commitsearch.StatePending,
		"is:submitted": commitsearch.StateSubmitted,
		"is:PENDING":   commitsearch.StatePending,
	}
	for query, want := range cases {
		f, err := commitsearch.ParseQuery(parsedRepoId, query)
		if err != nil {
			t.Fatalf("parsing %q failed: %s", query, err)
		}
		if f.State != want {
			t.Fatalf("parsing %q read the state %q, want %q",
				query, f.State, want)
		}
	}
}

func Test_ParseQuery_ReadsTheReviewStatusTerms(t *testing.T) {
	cases := map[string]review.ReviewStatus{
		"is:ready":        review.ReviewStatus_Ready,
		"is:missing-lgtm": review.ReviewStatus_MissingLgtm,
		"is:unresolved":   review.ReviewStatus_Unresolved,
	}
	for query, want := range cases {
		f, err := commitsearch.ParseQuery(parsedRepoId, query)
		if err != nil {
			t.Fatalf("parsing %q failed: %s", query, err)
		}
		if !f.HasReviewStatus || f.ReviewStatus != want {
			t.Fatalf("parsing %q read the status %v (set %v), want %v",
				query, f.ReviewStatus, f.HasReviewStatus, want)
		}
	}
}

func Test_ParseQuery_ExcludesWipCommits(t *testing.T) {
	f, err := commitsearch.ParseQuery(parsedRepoId, "-is:wip")

	if err != nil {
		t.Fatal(err)
	}
	if f.Wip != commitsearch.PresenceExclude {
		t.Fatalf("wip is %q, want %q", f.Wip, commitsearch.PresenceExclude)
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
		"author:aang author:katara is:pending is:submitted is:wip -is:wip")

	if err != nil {
		t.Fatalf("parsing failed: %s", err)
	}
	if f.AuthorUsername != "katara" {
		t.Fatalf("read the author %q, want katara", f.AuthorUsername)
	}
	if f.State != commitsearch.StateSubmitted {
		t.Fatalf("read the state %q, want submitted", f.State)
	}
	if f.Wip != commitsearch.PresenceExclude {
		t.Fatalf("read wip %q, want excluded", f.Wip)
	}
}

func Test_ParseQuery_FailsOnASearchItCanNotRun(t *testing.T) {
	cases := []string{
		// text can not be excluded from a search
		`-queue`,
		`-"a b"`,
		`-author:aang`,
		`-message:queue`,
		`-is:pending`,
		`-is:ready`,
		`is:nope`,
		// a status the index does not hold is refused, not silently empty
		`is:missing-owners-approval`,
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