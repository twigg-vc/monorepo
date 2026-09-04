package cli

import (
	"encoding/json"
	"fmt"
	"monorepo/twigg/commit"
)

// A commit as printed by `tw log --json`.
type JsonCommit struct {
	// Local commit syntax (e.g. "1549v17")
	Id string
	// Server commit syntax (e.g. "c/1529")
	// Empty when the commit has no server id
	ServerId string
	// Id of the parent in local commit syntax (e.g. "1549v17")
	// Empty for the root commit and for detached commits
	ParentId              string
	Message               string
	IsCurrent             bool
	IsSubmitted           bool
	IsPushed              bool
	HasConflicts          bool
	IsHidden              bool
	IsObsolete            bool
	HasDiffData           bool
	DiffDataLinesCreated  int64
	DiffDataLinesDeleted  int64
	DiffDataLinesModified int64
}

type JsonLog struct {
	Commits []JsonCommit
}

func jsonCommitId(l commit.LocalId, version uint64) string {
	return fmt.Sprintf("%dv%d", l, version)
}

func jsonServerId(serverId commit.LocalId, hasServerVersion bool, serverVersion uint64) string {
	if !hasServerVersion {
		return fmt.Sprintf("c/%d", serverId)
	}
	return fmt.Sprintf("c/%dv%d", serverId, serverVersion)
}

// Prints the logged tree as json. A commit is always printed before its
// parents, so the newest commits come first.
func (a *app) logTreeAsJson(root treeNode) {
	l := JsonLog{Commits: []JsonCommit{}}
	appendJsonCommits(&l.Commits, root)
	b, err := json.Marshal(l)
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.logInfo(string(b))
}

// Appends the children of n and lastly n itself
func appendJsonCommits(out *[]JsonCommit, n treeNode) {
	// Recurse on children
	for _, child := range n.visibleChildren {
		childNode, ok := child.(treeNode)
		if !ok {
			panic("visibleChildren is not treeNode")
		}
		appendJsonCommits(out, childNode)
	}
	// Add this commit
	*out = append(*out, treeNodeToJson(n))
}

func treeNodeToJson(n treeNode) JsonCommit {
	var parentId string
	if n.hasParent {
		parentId = jsonCommitId(n.parentL, n.parentV)
	}
	var serverId string
	if n.hasServerL {
		serverId = jsonServerId(n.serverL, n.hasServerV, n.serverV)
	}
	msg := n.msg
	if n.l == 0 {
		msg = firstCommitMsg
	}
	return JsonCommit{
		Id:                    jsonCommitId(n.l, n.version),
		ServerId:              serverId,
		ParentId:              parentId,
		Message:               msg,
		IsCurrent:             n.isActive,
		IsSubmitted:           n.isSubmitted,
		IsPushed:              n.isOnServer,
		HasConflicts:          n.hasConflicts,
		IsHidden:              n.isManuallyHidden,
		IsObsolete:            n.status == commit.StatusObsolete,
		HasDiffData:           n.hasDiffData,
		DiffDataLinesCreated:  n.diffDataLinesCreated,
		DiffDataLinesDeleted:  n.diffDataLinesDeleted,
		DiffDataLinesModified: n.diffDataLinesModified,
	}
}
