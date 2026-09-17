package commitsearch_test

import (
	"monorepo/twigg-web/commitsearch"
	"reflect"
	"strings"
	"testing"
)

const tokenizedMaxLen = 500

func newTestTokenizer() commitsearch.Tokenizer {
	return commitsearch.NewTokenizer(
		[]string{"is", "author", "message"}, tokenizedMaxLen)
}

func Test_Tokenizer_ReadsTheTokensOfAQuery(t *testing.T) {
	cases := []struct {
		query string
		want  []commitsearch.Token
	}{
		{"", nil},
		{"   ", nil},
		{`""`, nil},
		{"refactor the queue", []commitsearch.Token{
			{Value: "refactor"}, {Value: "the"}, {Value: "queue"},
		}},
		{"  refactor \t the \n queue ", []commitsearch.Token{
			{Value: "refactor"}, {Value: "the"}, {Value: "queue"},
		}},
		{`"refactor the queue"`, []commitsearch.Token{
			{Value: "refactor the queue", IsQuoted: true},
		}},
		// a token that opens with a quote is text, colon and all
		{`"is: refactor"`, []commitsearch.Token{
			{Value: "is: refactor", IsQuoted: true},
		}},
		{`"a b" c`, []commitsearch.Token{
			{Value: "a b", IsQuoted: true}, {Value: "c"},
		}},
		{"is:pending", []commitsearch.Token{{Key: "is", Value: "pending"}}},
		{"IS:Pending", []commitsearch.Token{{Key: "is", Value: "Pending"}}},
		{"-is:wip", []commitsearch.Token{
			{Key: "is", Value: "wip", IsNegated: true},
		}},
		// a quote right after the colon quotes only the value
		{`message:"a b"`, []commitsearch.Token{
			{Key: "message", Value: "a b", IsQuoted: true},
		}},
		{"author:me queue", []commitsearch.Token{
			{Key: "author", Value: "me"}, {Value: "queue"},
		}},
		// a - before a quote negates the quoted token
		{`-"a b"`, []commitsearch.Token{
			{Value: "a b", IsQuoted: true, IsNegated: true},
		}},
		{"- foo", []commitsearch.Token{{Value: "foo"}}},
		{"--foo", []commitsearch.Token{{Value: "-foo", IsNegated: true}}},
		// a colon inside a value is part of it
		{"message:a:b", []commitsearch.Token{{Key: "message", Value: "a:b"}}},
		// a quote is only special where a value can start or end, so one in
		// the middle of a value is part of it
		{`a"b`, []commitsearch.Token{{Value: `a"b`}}},
		{`is:a"b`, []commitsearch.Token{{Key: "is", Value: `a"b`}}},
		{`don"t worry`, []commitsearch.Token{
			{Value: `don"t`}, {Value: "worry"},
		}},
		// which is why a quote meant to open a value, written after the value
		// already started, is read as part of it
		{`is:a"b c"`, []commitsearch.Token{
			{Key: "is", Value: `a"b`}, {Value: `c"`},
		}},
	}
	tk := newTestTokenizer()
	for _, c := range cases {
		got, err := tk.Parse(c.query)
		if err != nil {
			t.Fatalf("parsing %q failed: %s", c.query, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Fatalf("parsing %q gave %+v, want %+v", c.query, got, c.want)
		}
	}
}

func Test_Tokenizer_FailsOnAKeyItDoesNotKnow(t *testing.T) {
	cases := []string{"nope:1", "NOPE:1", ":1", "-:1", "author:me nope:1"}
	tk := newTestTokenizer()
	for _, c := range cases {
		if _, err := tk.Parse(c); err == nil {
			t.Fatalf("parsing %q did not fail", c)
		}
	}
}

func Test_Tokenizer_FailsOnAValueItCanNotRead(t *testing.T) {
	cases := []string{
		// a key with nothing after it
		"is:",
		"-is:",
		`is:""`,
		"is: pending",
		// a quote that is never closed
		`"`,
		`an "unclosed quote`,
		`is:"unclosed`,
		// text glued to a value that a quote already closed
		`"a b"c`,
	}
	tk := newTestTokenizer()
	for _, c := range cases {
		if _, err := tk.Parse(c); err == nil {
			t.Fatalf("parsing %q did not fail", c)
		}
	}
}

func Test_Tokenizer_FailsOnAQueryLongerThanTheMax(t *testing.T) {
	tk := newTestTokenizer()

	_, err := tk.Parse(strings.Repeat("a", tokenizedMaxLen+1))

	if err == nil {
		t.Fatal("parsing a query over the max length did not fail")
	}
}

func Test_Tokenizer_CountsTheLengthInCharactersNotBytes(t *testing.T) {
	// characters of 2 bytes each: over the byte limit, within the character one
	query := strings.Repeat("ç", tokenizedMaxLen)
	tk := newTestTokenizer()

	got, err := tk.Parse(query)

	if err != nil {
		t.Fatalf("parsing %d two byte characters failed: %s", tokenizedMaxLen, err)
	}
	want := []commitsearch.Token{{Value: query}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsing %d two byte characters gave %+v, want %+v",
			tokenizedMaxLen, got, want)
	}
}

func Test_Tokenizer_HasNoMaxLengthWhenTheMaxIsNotPositive(t *testing.T) {
	tk := commitsearch.NewTokenizer([]string{"is"}, 0)
	query := strings.Repeat("a", 10_000)

	got, err := tk.Parse(query)

	if err != nil {
		t.Fatalf("parsing a long query failed: %s", err)
	}
	want := []commitsearch.Token{{Value: query}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsing a long query gave %d tokens, want 1", len(got))
	}
}
