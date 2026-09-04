package routes

import (
	"fmt"
	"monorepo/twigg/commit"
	"net/http"
	"net/url"
	"strconv"
)

type router struct{}

func (r router) repo(owner, repoDisplayName string) string {
	return "/" + owner + "/" + repoDisplayName
}
func (r router) Commit(repoOwnerName string, repoDisplayName string, cId commit.LocalId,
	hasLeftVersion bool, leftVersion uint64,
	hasRightVersion bool, rightVersion uint64) string {
	params := url.Values{}
	if hasLeftVersion {
		params.Add(leftVersionParamName,
			strconv.FormatInt(int64(leftVersion), 10))
	}
	if hasRightVersion {
		params.Add(rightVersionParamName,
			strconv.FormatInt(int64(rightVersion), 10))
	}
	var query string
	if len(params) != 0 {
		query = "?" + params.Encode()
	}
	return fmt.Sprintf("%s/c/%v%s", r.repo(repoOwnerName, repoDisplayName), cId, query)
}
func (r router) GetCommitParameter(req *http.Request) (commit.LocalId, error) {
	return strconv.ParseUint(req.PathValue(CommitParamName), 10, 64)
}
func (r router) Submit(repoOwnerName, repoDisplayName string, cId commit.LocalId) string {
	return fmt.Sprintf("%s/c/%v/submit", r.repo(repoOwnerName, repoDisplayName), cId)
}
func (r router) Diff(repoOwnerName, repoDisplayName string, cId commit.LocalId,
	hasLeftVersion bool, leftVersion uint64,
	hasRightVersion bool, rightVersion uint64, file string) string {
	params := url.Values{}
	if hasLeftVersion {
		params.Add(leftVersionParamName,
			strconv.FormatInt(int64(leftVersion), 10))
	}
	if hasRightVersion {
		params.Add(rightVersionParamName,
			strconv.FormatInt(int64(rightVersion), 10))
	}
	params.Add(fileParamName, file)
	return fmt.Sprintf("%s/c/%v/diff?%s", r.repo(repoOwnerName, repoDisplayName), cId, params.Encode())
}
func (r router) NewThread(repoOwnerName, repoDisplayName string, cId commit.LocalId) string {
	return fmt.Sprintf("%s/c/%v/new-thread", r.repo(repoOwnerName, repoDisplayName), cId)
}
func (r router) Thread(repoOwnerName, repoDisplayName string, cId commit.LocalId, threadId int) string {
	return fmt.Sprintf("%s/c/%v/thread/%d", r.repo(repoOwnerName, repoDisplayName), cId, threadId)
}
func (r router) GetFileParam(req *http.Request) string {
	return getStringParameter(req, fileParamName)
}
func (r router) GetLineParameter(req *http.Request) (hasLine bool, line uint64) {
	return getUint64Parameter(req, lineParamName)
}
func (r router) GetLeftVersionParameter(req *http.Request) (hasV bool, v uint64) {
	return getUint64Parameter(req, leftVersionParamName)
}
func (r router) GetRightVersionParameter(req *http.Request) (hasV bool, v uint64) {
	return getUint64Parameter(req, rightVersionParamName)
}
func (r router) GetVersionParameter(req *http.Request) (hasV bool, v uint64) {
	return getUint64Parameter(req, versionParamName)
}
func (r router) Threads(repoOwnerName, repoDisplayName string, cId commit.LocalId) string {
	return fmt.Sprintf("%s/c/%v/threads", r.repo(repoOwnerName, repoDisplayName), cId)
}
func (r router) Comments(repoOwnerName, repoDisplayName string, cId commit.LocalId) string {
	return r.Threads(repoOwnerName, repoDisplayName, cId) + "/comments"
}

const leftVersionParamName = "left"
const rightVersionParamName = "right"
const fileParamName = "file"

// when a thread is posted we just need to know the version, as its tied
// to one of the versions necessarily. It can't be -1, as commenting on the
// base is not possible.
const versionParamName = "version"

// when a thread is posted, the line of the file it is anchored to. It is
// absent for threads anchored to the file as a whole.
const lineParamName = "line"

const threadIdParamName = "id"

func getStringParameter(r *http.Request, paramName string) string {
	val := r.FormValue(paramName)
	if val != "" {
		return val
	}
	return r.PathValue(paramName)
}

func getUint64Parameter(r *http.Request, paramName string) (hasV bool, n uint64) {
	var err error

	p := r.PathValue(paramName)
	n, err = strconv.ParseUint(p, 10, 64)
	if err == nil {
		hasV = true
		return
	}

	p = r.FormValue(paramName)
	n, err = strconv.ParseUint(p, 10, 64)
	if err == nil {
		hasV = true
		return
	}
	return
}