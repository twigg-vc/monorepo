package repository

import (
	"reflect"
	"testing"
)

func TestParseGolinksFile(t *testing.T) {
	got, err := ParseGolinksFile([]byte(
		"# team links\n" +
			"docs https://example.com/docs\n" +
			"\n" +
			"  ci \t http://ci.example.com/a?b=c  \n"))
	if err != nil {
		t.Fatalf("ParseGolinksFile() err=%s", err)
	}
	want := map[string]string{
		"docs": "https://example.com/docs",
		"ci":   "http://ci.example.com/a?b=c",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseGolinksFile()=%v, want %v", got, want)
	}
}

func TestParseGolinksFileEmpty(t *testing.T) {
	got, err := ParseGolinksFile([]byte("# nothing yet\n\n"))
	if err != nil {
		t.Fatalf("ParseGolinksFile() err=%s", err)
	}
	if len(got) != 0 {
		t.Errorf("ParseGolinksFile()=%v, want empty", got)
	}
}

func TestParseGolinksFileRejectsBadFiles(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		wantErr string
	}{
		{"missing url", "docs\n", `line 1: expected "<alias> <url>"`},
		{"extra field", "docs https://a.com x\n", `line 1: expected "<alias> <url>"`},
		{"relative url", "docs /docs\n", `line 1: "/docs" is not an http(s) url`},
		{"javascript url", "x javascript:alert(1)\n", `line 1: "javascript:alert(1)" is not an http(s) url`},
		{"duplicate alias", "# c\ndocs https://a.com\ndocs https://b.com\n", `line 3: duplicate alias "docs"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseGolinksFile([]byte(tc.file))
			if err == nil {
				t.Fatalf("ParseGolinksFile()=%v, want err %q", got, tc.wantErr)
			}
			if err.Error() != tc.wantErr {
				t.Errorf("ParseGolinksFile() err=%q, want %q", err, tc.wantErr)
			}
		})
	}
}
