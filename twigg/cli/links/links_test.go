package links

import (
	"testing"
)

func TestBasicLink(t *testing.T) {
	gotHyperlink := GetHyperlink("Hello", "https://google.com")
	expectedHyperlink := "\x1b]8;;https://google.com\x07Hello\x1b]8;;\x07"
	if gotHyperlink != expectedHyperlink {
		t.Fatalf("expected %q got %q", expectedHyperlink, gotHyperlink)
	}
}

func TestRemoveHyperlinks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic hyperlink",
			input:    GetHyperlink("Hello", "https://twigg.vc/"),
			expected: "Hello",
		},
		{
			name:     "string without hyperlinks",
			input:    "Just plain text",
			expected: "Just plain text",
		},
		{
			name: "long hyperlinks",
			input: GetHyperlink("long",
				"http://twigg.vc/test/c/14?left=0&right=1&tab=changes"),
			expected: "long",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name: "multiple hyperlinks",
			input: GetHyperlink("First", "https://a.com") +
				" and " +
				GetHyperlink("Second", "https://b.com"),
			expected: "First and Second",
		},
	}

	for _, tc := range tests {
		got := RemoveHyperlinks(tc.input)
		if got != tc.expected {
			t.Fatalf("tets %q failed expected %q got %q",
				tc.name, tc.expected, got)
		}
	}
}