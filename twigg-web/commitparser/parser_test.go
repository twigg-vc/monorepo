package commitparser_test

import (
	"monorepo/twigg-web/commitparser"
	"reflect"
	"strings"
	"testing"
)

func TestParseCommitDescription(t *testing.T) {
	tests := []struct {
		name string
		desc string
		want commitparser.Tags
	}{
		{
			name: "no tag",
			desc: "fix the tea kettle",
			want: commitparser.Tags{},
		},
		{
			name: "b/<number>",
			desc: "fix the tea kettle\n\nb/42",
			want: commitparser.Tags{Bugs: []int64{42}},
		},
		{
			name: "B/<number> uppercase",
			desc: "fix the tea kettle\n\nB/42",
			want: commitparser.Tags{Bugs: []int64{42}},
		},
		{
			name: "multiple tags on separate lines",
			desc: "fix the tea kettle\n\nb/42\nb/17",
			want: commitparser.Tags{Bugs: []int64{42, 17}},
		},
		{
			name: "duplicate tag is deduplicated",
			desc: "fix the tea kettle\n\nb/42\nb/42",
			want: commitparser.Tags{Bugs: []int64{42}},
		},
		{
			name: "tag embedded mid-line does not match",
			desc: "see b/42 for context",
			want: commitparser.Tags{},
		},
		{
			name: "bug=<number>",
			desc: "fix the tea kettle\n\nbug=42",
			want: commitparser.Tags{Bugs: []int64{42}},
		},
		{
			name: "BUG=<number> uppercase",
			desc: "fix the tea kettle\n\nBUG=42",
			want: commitparser.Tags{Bugs: []int64{42}},
		},
		{
			name: "bug= <number> with a space",
			desc: "fix the tea kettle\n\nbug= 42",
			want: commitparser.Tags{Bugs: []int64{42}},
		},
		{
			name: "BUG:<number>",
			desc: "fix the tea kettle\n\nBUG:42",
			want: commitparser.Tags{Bugs: []int64{42}},
		},
		{
			name: "bug: <number> with a space",
			desc: "fix the tea kettle\n\nbug: 42",
			want: commitparser.Tags{Bugs: []int64{42}},
		},
		{
			name: "b/ and BUG= tags for the same bug are deduplicated",
			desc: "fix the tea kettle\n\nb/42\nBUG=42\nBUG=17",
			want: commitparser.Tags{Bugs: []int64{42, 17}},
		},
		{
			name: "BUG= embedded mid-line does not match",
			desc: "see BUG=42 for context",
			want: commitparser.Tags{},
		},
		{
			name: "malformed BUG= with no number does not match",
			desc: "BUG=",
			want: commitparser.Tags{},
		},
		{
			name: "malformed tag with no number does not match",
			desc: "b/",
			want: commitparser.Tags{},
		},
		{
			name: "malformed tag with trailing letters does not match",
			desc: "b/12x",
			want: commitparser.Tags{},
		},
		{
			name: "FIXES=<number>",
			desc: "fix the tea kettle\n\nFIXES=42",
			want: commitparser.Tags{Fixes: []int64{42}},
		},
		{
			name: "fixes: <number> lowercase with a space",
			desc: "fix the tea kettle\n\nfixes: 42",
			want: commitparser.Tags{Fixes: []int64{42}},
		},
		{
			name: "duplicate FIXES tag is deduplicated",
			desc: "fix the tea kettle\n\nFIXES=42\nfixes:42\nFIXES=17",
			want: commitparser.Tags{Fixes: []int64{42, 17}},
		},
		{
			name: "FIXES and BUG tags for the same bug fill both lists",
			desc: "fix the tea kettle\n\nBUG=42\nFIXES=42",
			want: commitparser.Tags{Bugs: []int64{42}, Fixes: []int64{42}},
		},
		{
			name: "FIXES= embedded mid-line does not match",
			desc: "see FIXES=42 for context",
			want: commitparser.Tags{},
		},
		{
			name: "every fix and close variant",
			desc: "fix the tea kettle\n\nFIX=1\nfixed: 2\nCLOSE=3\ncloses:4\nClosed= 5",
			want: commitparser.Tags{Fixes: []int64{1, 2, 3, 4, 5}},
		},
		{
			name: "fix and close variants for the same bug are deduplicated",
			desc: "fix the tea kettle\n\nFIXED=42\nCLOSES=42",
			want: commitparser.Tags{Fixes: []int64{42}},
		},
		{
			name: "misspelled variant does not match",
			desc: "fix the tea kettle\n\nFIXD=42\nCLOSING=42",
			want: commitparser.Tags{},
		},
		{
			name: "FIXES b/<number> does not match",
			desc: "FIXES b/42",
			want: commitparser.Tags{},
		},
		{
			name: "description longer than the max is ignored even with a well-formed tag",
			desc: strings.Repeat("a", 10_000) + "\nb/42",
			want: commitparser.Tags{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := commitparser.ParseCommitDescription(tt.desc)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseCommitDescription(%q): expected %+v, got %+v", tt.desc, tt.want, got)
			}
		})
	}
}