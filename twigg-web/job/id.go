package job

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const pipelineIdPrefix = "p-"
const pipelineStageIdSuffix = ".s"

var pipelineStageIdRegexp = regexp.MustCompile(fmt.Sprintf(`^%s.*\%s[0-9]+$`, pipelineIdPrefix, pipelineStageIdSuffix))

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

func parsePipelineStageId(id string) (RepoId uint64, Commit uint64, CommitVersion uint64,
	Path string, Name string, RunNumber int64, Stage int32, ok bool) {
	i := strings.LastIndex(id, pipelineStageIdSuffix)
	if i < 0 {
		return
	}
	stageStr := id[i+2:]
	stage, err := strconv.ParseInt(stageStr, 10, 32)
	if err != nil {
		return
	}
	Stage = int32(stage)
	RepoId, Commit, CommitVersion, Path, Name, RunNumber, ok =
		ParsePipelineId(id[:i])
	return
}
