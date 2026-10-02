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
			name: "bug= prefix is not (yet) recognized",
			desc: "BUG=42",
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
