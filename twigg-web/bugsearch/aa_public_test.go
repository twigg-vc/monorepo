package bugsearch_test

import (
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/bugsearch"
	"testing"
)

const parsedBugRepoId = 7

func Test_ParseQuery_ReadsTheRepoId(t *testing.T) {
	f, err := bugsearch.ParseQuery(parsedBugRepoId, "")
	if err != nil {
		t.Fatal(err)
	}
	if f.RepoId != parsedBugRepoId {
		t.Fatalf("read the repo %d, want %d", f.RepoId, parsedBugRepoId)
	}
}

func Test_ParseQuery_ReadsTheStatusTerms(t *testing.T) {
	cases := map[string]bug.Status{
		"is:open":   bug.Status_Open,
		"is:closed": bug.Status_Closed,
		"is:OPEN":   bug.Status_Open,
	}
	for query, want := range cases {
		f, err := bugsearch.ParseQuery(parsedBugRepoId, query)
		if err != nil {
			t.Fatalf("parsing %q failed: %s", query, err)
		}
		if f.Status != want {
			t.Fatalf("parsing %q read the status %q, want %q",
				query, f.Status, want)
		}
	}
}

func Test_ParseQuery_ReadsTheAssignee(t *testing.T) {
	f, err := bugsearch.ParseQuery(parsedBugRepoId, "assignee:aang")

	if err != nil {
		t.Fatalf("parsing failed: %s", err)
	}
	if f.AssigneeUsername != "aang" || f.AssigneeUnassigned {
		t.Fatalf("read the assignee %q (unassigned=%v), want aang (false)",
			f.AssigneeUsername, f.AssigneeUnassigned)
	}
}

func Test_ParseQuery_ReadsTheMeAssignee(t *testing.T) {
	f, err := bugsearch.ParseQuery(parsedBugRepoId, "assignee:me")

	if err != nil {
		t.Fatalf("parsing failed: %s", err)
	}
	if f.AssigneeUsername != bugsearch.MeUsername {
		t.Fatalf("read the assignee %q, want %q", f.AssigneeUsername,
			bugsearch.MeUsername)
	}
}

func Test_ParseQuery_ReadsTheUnassignedTerm(t *testing.T) {
	for _, query := range []string{"assignee:unassigned", "assignee:none", "assignee:UNASSIGNED"} {
		f, err := bugsearch.ParseQuery(parsedBugRepoId, query)
		if err != nil {
			t.Fatalf("parsing %q failed: %s", query, err)
		}
		if !f.AssigneeUnassigned {
			t.Fatalf("parsing %q did not read AssigneeUnassigned", query)
		}
		if f.AssigneeUsername != "" {
			t.Fatalf("parsing %q read the assignee %q, want empty",
				query, f.AssigneeUsername)
		}
	}
}

func Test_ParseQuery_ReadsThePriority(t *testing.T) {
	cases := map[string]bug.Priority{
		"priority:P0": bug.Priority_P0,
		"priority:p1": bug.Priority_P1,
		"priority:P2": bug.Priority_P2,
	}
	for query, want := range cases {
		f, err := bugsearch.ParseQuery(parsedBugRepoId, query)
		if err != nil {
			t.Fatalf("parsing %q failed: %s", query, err)
		}
		if !f.HasPriority || f.Priority != want {
			t.Fatalf("parsing %q read the priority %q (has=%v), want %q (true)",
				query, f.Priority, f.HasPriority, want)
		}
	}
}

func Test_ParseQuery_ReadsTheNonePriorityTerm(t *testing.T) {
	f, err := bugsearch.ParseQuery(parsedBugRepoId, "priority:P0 priority:NONE")

	if err != nil {
		t.Fatalf("parsing failed: %s", err)
	}
	if !f.HasPriority || f.Priority != bug.Priority_None {
		t.Fatalf("read the priority %q (has=%v), want empty (true)",
			f.Priority, f.HasPriority)
	}
}

func Test_ParseQuery_CombinesStatusAndAssignee(t *testing.T) {
	f, err := bugsearch.ParseQuery(parsedBugRepoId, "is:open assignee:aang")
	if err != nil {
		t.Fatalf("parsing failed: %s", err)
	}
	if f.Status != bug.Status_Open || f.AssigneeUsername != "aang" {
		t.Fatalf("read status=%q assignee=%q, want open/aang",
			f.Status, f.AssigneeUsername)
	}
}

// A search bar appends a term to what is already written, so the last one
// wins instead of the search contradicting itself.
func Test_ParseQuery_KeepsTheLastValueOfARepeatedTerm(t *testing.T) {
	f, err := bugsearch.ParseQuery(parsedBugRepoId,
		"is:open is:closed assignee:aang assignee:unassigned")

	if err != nil {
		t.Fatalf("parsing failed: %s", err)
	}
	if f.Status != bug.Status_Closed {
		t.Fatalf("read the status %q, want closed", f.Status)
	}
	if !f.AssigneeUnassigned || f.AssigneeUsername != "" {
		t.Fatalf("read the assignee %q (unassigned=%v), want empty (true)",
			f.AssigneeUsername, f.AssigneeUnassigned)
	}
}

func Test_ParseQuery_FailsOnASearchItCanNotRun(t *testing.T) {
	cases := []string{
		// there is no free text search yet
		`queue`,
		`"a b"`,
		// negation isn't supported for any key
		`-is:open`,
		`-assignee:aang`,
		`-priority:P0`,
		// a status/priority/key the parser doesn't know
		`is:nope`,
		`priority:P3`,
		`priority:`,
		`nope:1`,
		// the tokenizer refuses this before the words are read
		`an "unclosed quote`,
	}
	for _, query := range cases {
		_, err := bugsearch.ParseQuery(parsedBugRepoId, query)
		if err == nil {
			t.Fatalf("parsing %q did not fail", query)
		}
	}
}
