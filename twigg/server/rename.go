package server

import (
	"errors"
	"fmt"
	"monorepo/twigg/client"
	"monorepo/twigg/commit"
	"monorepo/twigg/tree"
	"unicode/utf8"
)

func (s *srv) RenameCommit(cId commit.LocalId, newMessage string, authorUserId int64, l Write) (commit.Commit, error) {
	if !s.WasInit() {
		return commit.Commit{}, errors.New("not initialized")
	}
	if newMessage == "" {
		return commit.Commit{}, errors.New("message cannot be empty")
	}
	if utf8.RuneCountInString(newMessage) > client.MaxMsgLen {
		return commit.Commit{}, fmt.Errorf("message too long (max %d chars)", client.MaxMsgLen)
	}
	c, err := s.GetLatest(cId, l)
	if err != nil {
		return commit.Commit{}, err
	}
	if c.IsSubmitted {
		return commit.Commit{}, errors.New("cannot rename a submitted commit")
	}
	if c.Message == newMessage {
		return commit.Commit{}, errors.New("new message is the same as the current one")
	}
	parent, err := s.GetVersion(c.ParentL, c.ParentV, l)
	if err != nil {
		return commit.Commit{}, err
	}
	diffCounts := tree.TotalDiffCounts{
		LinesCreated:  c.DiffDataLinesCreated,
		LinesDeleted:  c.DiffDataLinesDeleted,
		LinesModified: c.DiffDataLinesModified,
		FilesCreated:  c.DiffDataFilesCreated,
		FilesDeleted:  c.DiffDataFilesDeleted,
		FilesModified: c.DiffDataFilesModified,
	}
	amended := commit.NewAmend(
		c.TreeVersion, c.RootDirHash, newMessage, &c, &parent, true, &authorUserId,
		diffCounts,
	)

	// c is now obsolete (SetSuccessor was called by NewAmend), so we must save it
	if err = l.SetCommit(s.QuotaOwner, s.RepoId, c); err != nil {
		return commit.Commit{}, err
	}

	// save the new commit
	if err = l.SetCommit(s.QuotaOwner, s.RepoId, amended); err != nil {
		return commit.Commit{}, err
	}

	// Note that we purposely don't save the parent
	// because on the server children are only attached on submit.

	return amended, nil
}
