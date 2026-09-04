package docusaurus

import (
	"net/http/httptest"
	"testing"
)

func Test_setCacheHeaders(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		wantControl string
	}{
		{
			name:        "page is not cached",
			filePath:    "build/blog/twigg-is-open-source/index.html",
			wantControl: "no-cache",
		},
		{
			name:        "directory is not cached",
			filePath:    "build/commands",
			wantControl: "no-cache",
		},
		{
			name:        "feed is not cached",
			filePath:    "build/blog/rss.xml",
			wantControl: "no-cache",
		},
		{
			// It is requested with a hash of its contents in the query, so a
			// new build is already requested under a different url.
			name:        "search index is cached",
			filePath:    "build/search-index.json",
			wantControl: "public, max-age=86400",
		},
		{
			name:        "script is cached",
			filePath:    "build/assets/js/main.deadbeef.js",
			wantControl: "public, max-age=86400",
		},
		{
			name:        "image is cached",
			filePath:    "build/img/twigg-og-card.png",
			wantControl: "public, max-age=86400",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			setCacheHeaders(w, tt.filePath)

			got := w.Header().Get("Cache-Control")
			if got != tt.wantControl {
				t.Errorf("got cache-control %q, want %q", got, tt.wantControl)
			}
		})
	}
}
