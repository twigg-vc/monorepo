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
