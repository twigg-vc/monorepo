package workdir

import (
	"bufio"
	"bytes"
	"monorepo/twigg/gitignore"
	"path"
	"strings"
)

func (f *wd) Ignore(pattern string) error {
	f.hasIgnCache = false
	f.ignoreRules = append(
		f.ignoreRules,
		pattern)
	return nil
}

func (w *wd) ClearIgnores() {
	w.ignoreRules = []string{}
	w.hasIgnCache = false
}

func (f *wd) isIgnored(path_ string) bool {
	if !f.hasIgnCache {
		f.ign = gitignore.CompileIgnoreLines(f.ignoreRules...)
		f.hasIgnCache = true
	}
	return f.ign.MatchesPath(path_)
}

func isIgnoreFile(baseName string) bool {
	return baseName == IgnoreFileName
}

// Parse an ignore file and add it to the ignored list.
// If the list already contains it it's ok; as appearing twice on the list
// is fine.
func (w *wd) parseIgnoreFile(path_ string) error {
	buff := bytes.NewBuffer(nil)

	if w.isIgnored(path_) {
		return ErrNotFound
	}
	wt := newFileWt(w.cleanAbsPath(path_))
	_, err := wt.WriteTo(buff)
	if err != nil {
		return err
	}

	pathDir := path.Dir(path_)
	scanner := bufio.NewScanner(buff)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "#") {
			continue
		}
		pattern := scanner.Text()
		if len(pattern) == 0 {
			continue
		}

		// Before calling path.Join, do some adjustments to the pattern
		hasLeadingSlash := strings.HasPrefix(pattern, "/")
		isNegate := false
		if strings.HasPrefix(pattern, "!") {
			isNegate = true
			pattern = strings.TrimPrefix(pattern, "!")
		}
		// Call path.Join and "undo" the adjustments
		relativePattern := path.Join(pathDir, pattern)
		if hasLeadingSlash {
			relativePattern = "/" + relativePattern
		}
		if isNegate {
			relativePattern = "!" + relativePattern
		}

		w.Ignore(relativePattern)
	}
	return nil
}