package cli

import (
	"encoding/json"
	"monorepo/twigg/tree"
)

// A file changed by a diff, as printed by `tw diff --json`.
type JsonDiffFile struct {
	Path string
	// "created", "deleted" or "modified"
	Status string
}

type JsonDiff struct {
	Files []JsonDiffFile
}

// Prints the files changed by the diff as json.
func (a *app) logDiffAsJson(diffs tree.ParallelIterator) {
	d := JsonDiff{Files: []JsonDiffFile{}}
	ok := a.walkDiffs(diffs, func(path string, diffType tree.DiffType) {
		d.Files = append(d.Files, JsonDiffFile{
			Path:   path,
			Status: diffTypeText(diffType),
		})
	})
	if !ok {
		return
	}
	b, err := json.Marshal(d)
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.logInfo(string(b))
}
