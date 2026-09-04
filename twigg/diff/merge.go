package diff

import (
	"bytes"
	"fmt"
	diff3 "monorepo/twigg/diff/epiclabs-io"
	"net/http"
	"strings"
)

func isText(b []byte) bool {
	if strings.Contains(http.DetectContentType(b), "text") || len(b) == 0 {
		return true
	}
	return false
}

func merge(
	base []byte,
	v1 []byte,
	v1Label string,
	v2 []byte,
	v2Label string) (v1v2 []byte, conflict bool) {
	if bytes.Equal(v1, v2) {
		return v1, false
	}

	res, err := diff3.Merge(
		bytes.NewReader(v1),
		bytes.NewReader(base),
		bytes.NewReader(v2),
		/*detailed=*/ false,
		v1Label,
		v2Label)
	if err != nil {
		panic(fmt.Sprintf("failed to merge %s and %s", v1Label, v2Label))
	}

	b := bytes.NewBuffer(nil)
	_, err = b.ReadFrom(res.Result)
	if err != nil {
		panic("failed to read merge results to buffer")
	}
	return b.Bytes(), res.Conflicts
}

func fileHasConflicts(file []byte) bool {
	return bytes.Contains(file, []byte(diff3.ConflictStart)) ||
		bytes.Contains(file, []byte(diff3.ConflictMid)) ||
		bytes.Contains(file, []byte(diff3.ConflictEnd))
}
