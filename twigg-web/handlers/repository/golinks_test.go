package repository

import (
	"errors"
	"io"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/wrappers"
	"monorepo/twigg/commit"
	"monorepo/twigg/tree"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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
		{"bad alias", "docs/api https://a.com\n", `line 1: bad alias "docs/api" - only letters, numbers, "_" and "-" are allowed`},
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

func TestHandleGetGoLinkRedirects(t *testing.T) {
	h := newGoLinkHandler(func(filename string) (tree.Tree, bool, error) {
		if filename != GoLinksFileName {
			t.Fatalf("GetRepoTree(%q), want %q", filename, GoLinksFileName)
		}
		return newGoLinksFileTree("docs https://example.com/docs\n"), false, nil
	})
	w := httptest.NewRecorder()

	h.handleGetGoLink(w, newGoLinkReq("docs"), nil)

	if w.Code != http.StatusFound {
		t.Fatalf("code=%d, want %d. body=%s", w.Code, http.StatusFound, w.Body)
	}
	if got := w.Header().Get("Location"); got != "https://example.com/docs" {
		t.Errorf("Location=%q, want %q", got, "https://example.com/docs")
	}
}

func TestHandleGetGoLinkUnknownAlias(t *testing.T) {
	h := newGoLinkHandler(func(string) (tree.Tree, bool, error) {
		return newGoLinksFileTree("docs https://example.com/docs\n"), false, nil
	})
	w := httptest.NewRecorder()

	h.handleGetGoLink(w, newGoLinkReq("ci"), nil)

	if w.Code != http.StatusNotFound {
		t.Fatalf("code=%d, want %d. body=%s", w.Code, http.StatusNotFound, w.Body)
	}
}

func TestHandleGetGoLinkBadGoLinksFile(t *testing.T) {
	tests := []struct {
		name          string
		tr            tree.Tree
		isNotFoundErr bool
		err           error
		wantCode      int
	}{
		{"isNotFoundErr", nil, true, errors.New("not found"), http.StatusNotFound},
		{"read error", nil, false, errors.New("boom"), http.StatusInternalServerError},
		{"directory", treeMock{data: tree.Data{IsDir: true}}, false, nil, http.StatusBadRequest},
		{"too large", treeMock{data: tree.Data{Size: GoLinksFileMaxSize + 1}}, false, nil, http.StatusBadRequest},
		{"malformed", newGoLinksFileTree("docs\n"), false, nil, http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newGoLinkHandler(func(string) (tree.Tree, bool, error) {
				return tc.tr, tc.isNotFoundErr, tc.err
			})
			w := httptest.NewRecorder()

			h.handleGetGoLink(w, newGoLinkReq("docs"), nil)

			if w.Code != tc.wantCode {
				t.Errorf("code=%d, want %d. body=%s", w.Code, tc.wantCode, w.Body)
			}
		})
	}
}

func newGoLinkHandler(getRepoTree func(filename string) (tree.Tree, bool, error)) handler {
	repoS := repoServiceMock{
		getRepoTopCommit: func() (commit.Commit, error) {
			return commit.Commit{}, nil
		},
		getRepoTree: getRepoTree,
	}
	return NewHandler(repoS, reviewServiceMock{}, userServiceMock{}, &commitSearchDbMock{})
}

func newGoLinkReq(alias string) wrappers.UserWithReadPermissionMuxRequest {
	return newMockReq(map[string]string{routes.GoLinksAliasPathParamName: alias}, nil)
}

type treeMock struct {
	data    tree.Data
	content string
}

func newGoLinksFileTree(content string) treeMock {
	return treeMock{
		data:    tree.Data{BaseName: GoLinksFileName, IsText: true, Size: int64(len(content))},
		content: content,
	}
}

func (m treeMock) IsRemovedChild() bool { return false }
func (m treeMock) DataIsComplete() bool { return true }
func (m treeMock) Data() tree.Data      { return m.data }
func (m treeMock) GetFile() (io.WriterTo, error) {
	return strings.NewReader(m.content), nil
}
