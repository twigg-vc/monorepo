package docusaurus

import (
	"monorepo/twigg-web/cacheheaders"
	"net/http"
	"path"
)

// Sets the correct headers based on which file is requested. Almost everything
// can be cached except for some base content like html
func setCacheHeaders(w http.ResponseWriter, filePath string) {
	if shouldNotCache(filePath) {
		cacheheaders.NoCache(w)
		return
	}
	cacheheaders.LongCache(w)
}

func shouldNotCache(filePath string) bool {
	ext := path.Ext(filePath)
	// Directories (served as index.html) and explicit HTML/XML files must never be cached
	if ext == "" || ext == ".html" || ext == ".xml" {
		return true
	}
	// Some specific jsons should never be cached
	if ext == ".json" {
		fileName := path.Base(filePath)
		if fileName == "siteManifest.json" ||
			fileName == "manifest.json" {
			return true
		}
		return false
	}
	return false
}
