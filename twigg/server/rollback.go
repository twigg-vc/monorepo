package server

import (
	"errors"
	"fmt"
	"monorepo/twigg/client"
	"monorepo/twigg/commit"
	"monorepo/twigg/repo"
	"monorepo/twigg/tree"
)

func (s *srv) CreateRollback(aId commit.LocalId, authorUserId int64, l Write) (commit.Commit, error) {
	if !s.WasInit() {
		return commit.Commit{}, errors.New("not initialized")
	}
	A, err := s.GetLatest(aId, l)
	if err != nil {
		return commit.Commit{}, err
	}
	if !A.IsSubmitted {
		return commit.Commit{}, errors.New("cant rollback non-submitted commits")
	}
	P, err := s.GetVersion(A.ParentL, A.ParentV, l)
	if err != nil {
		return commit.Commit{}, err
	}

	v, hash, conflict, err := s.r.Rebase(
		P.TreeVersion, commitLabelOnRollback(P),
		s.Top_.TreeVersion, commitLabelOnRollback(s.Top_),
		A.TreeVersion, l)
	if err != nil && !errors.Is(err, repo.ErrNoChange) {
		return commit.Commit{}, err
	}
	diffCounts, err := tree.CountDiffs(
		s.r.Root(v, l),
		s.r.Root(s.Top_.TreeVersion, l))
	if err != nil {
		return commit.Commit{}, err
	}
	message := trimEllipsis(
		fmt.Sprintf("Rollback c/%d: %s", aId, A.Message),
		client.MaxMsgLen)
	const attachToParent = false
	rollback := commit.NewRollback(A, conflict,
		/*isOnServer=*/ true,
		s.NextLocalId,
		v,
		hash,
		message,
		attachToParent,
		/*parent*/ &s.Top_,
		diffCounts,
	)
	rollback.AuthorUserId = authorUserId
	err = l.SetCommit(s.QuotaOwner, s.RepoId, rollback)
	if err != nil {
		return commit.Commit{}, err
	}
	s.NextLocalId += 1
	err = s.save(l)
	if err != nil {
		s.NextLocalId -= 1
		return commit.Commit{}, err
	}
	return rollback, nil
}

func commitLabelOnRollback(c commit.Commit) string {
	return fmt.Sprintf("#%dv%d", c.L, c.Version)
}

// Trim string up to a lenght and adds "..."
func trimEllipsis(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}
