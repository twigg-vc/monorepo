package job

import (
	"encoding/base64"
	"strconv"
	"strings"
)

func parseJobId(id string) (repoId uint64, commit uint64, commitVersion uint64,
	path string, name string, runNumber int64, ok bool) {
	parts := strings.Split(id, ".")
	if len(parts) != 6 {
		return
	}
	var err error
	if repoId, err = strconv.ParseUint(parts[0], 10, 64); err != nil {
		return
	}
	if commit, err = strconv.ParseUint(parts[1], 10, 64); err != nil {
		return
	}
	if commitVersion, err = strconv.ParseUint(parts[2], 10, 64); err != nil {
		return
	}
	var b []byte
	if b, err = base64.RawURLEncoding.DecodeString(parts[3]); err != nil {
		return
	}
	path = string(b)
	if b, err = base64.RawURLEncoding.DecodeString(parts[4]); err != nil {
		return
	}
	name = string(b)
	if runNumber, err = strconv.ParseInt(parts[5], 10, 64); err != nil {
		return
	}
	ok = true
	return
}
