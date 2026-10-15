package termsandprivacy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChromeExtPrivacy(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/chrome-ext-privacy", nil)
	handleGetChromeExtPrivacy(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want plain UTF-8 text", got)
	}
	for _, disclosure := range []string{
		"Twigg Links Chrome Extension Privacy Policy",
		"Chrome Sync",
		"Clear the repository field and click Save",
		"andre@twigg.vc",
	} {
		if !strings.Contains(w.Body.String(), disclosure) {
			t.Errorf("policy missing %q", disclosure)
		}
	}
}
