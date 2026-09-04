package server

import (
	"errors"
	"monorepo/twigg/commit"
	"monorepo/twigg/repo"
	"monorepo/twigg/tree"
)

func (s *srv) Submit(input commit.Commit, l Write) (err error) {
	if !s.WasInit() {
		return errors.New("not initialized")
	}
	if len(s.Top_.Children) != 0 {
		panic("top has children")
	}
	if input.IsSubmitted {
		err = errAlreadySubmitted
		return
	}
	if input.HasRebaseConflicts {
		err = errCantSubmitWithConflict
		return
	}
	if input.Status != commit.StatusLatest {
		err = errors.New("cant submit obsolete commit")
		return
	}

	// This will be the commit that is actually submitted
	var submitted commit.Commit
	// Cleanup if something goes wrong
	defer func() {
		// If the submit worked
		if err == nil {
			// Update the new top
			topCopy := s.Top_
			s.Top_ = submitted

			// Save the server state
			err = s.save(l)
			// If saving fails, reset the server state
			if err != nil {
				s.Top_ = topCopy
				return
			}
		}
	}()

	latestVersionOfParent, err := s.GetLatest(input.ParentL, l)
	if err != nil {
		return
	}
	if !latestVersionOfParent.IsSubmitted {
		err = errParentNotSubmitted
		return
	}

	var parent commit.Commit
	if input.ParentV != latestVersionOfParent.Version {
		parent, err = s.GetVersion(input.ParentL, input.ParentV, l)
		if err != nil {
			return
		}
		if parent.Status != commit.StatusObsolete {
			panic("parent was not latest version but is not obsolete")
		}
	} else {
		parent = latestVersionOfParent
	}

	// If the top is the parent, we just create a trivial rebase commit
	if s.Top_.L == parent.L && s.Top_.Version == parent.Version {
		submitted = commit.NewTrivialSubmit(&input, &parent)
		err = l.SetCommit(s.QuotaOwner, s.RepoId, submitted)
		if err != nil {
			return
		}
		err = l.SetCommit(s.QuotaOwner, s.RepoId, parent)
		if err != nil {
			return
		}
		err = l.SetCommit(s.QuotaOwner, s.RepoId, input)
		if err != nil {
			return
		}
		return
	}

	// Else, actually perform the rebase.
	// We can use empty labels bc those are only used to show conflicts
	// in files.
	const labels = ""
	v, hash, conflict, err := s.r.Rebase(
		input.TreeVersion,
		labels,
		s.Top_.TreeVersion,
		labels,
		input.ParentTreeVersion, l)
	if err != nil && !errors.Is(err, repo.ErrNoChange) {
		return err
	}
	if conflict {
		err = errSubmitWouldConflict
		return
	}
	diffCounts, err := tree.CountDiffs(
		s.r.Root(v, l),
		s.r.Root(s.Top_.TreeVersion, l))
	if err != nil {
		return err
	}
	submitted = commit.NewRebaseSubmit(
		v,
		hash,
		&input,
		&s.Top_,
		diffCounts,
	)
	err = l.SetCommit(s.QuotaOwner, s.RepoId, submitted)
	if err != nil {
		return err
	}
	err = l.SetCommit(s.QuotaOwner, s.RepoId, input)
	if err != nil {
		return err
	}
	err = l.SetCommit(s.QuotaOwner, s.RepoId, s.Top_)
	if err != nil {
		return err
	}
	return
}

func (s srv) CanSubmit(c commit.Commit, l Read) (bool, CantSubmitReason, error) {
	if !s.WasInit() {
		return false, CantSubmitReasonNone, errors.New("not initialized")
	}
	if len(s.Top_.Children) != 0 {
		panic("top has children")
	}
	if c.Status != commit.StatusLatest {
		return false, CantSubmitObsoleteCommit, nil
	}
	if c.IsSubmitted {
		return false, CantSubmitAlreadySubmitted, nil
	}
	if c.HasRebaseConflicts {
		return false, CantSubmitWithConflict, nil
	}
	if c.ParentL == s.Top_.L && c.ParentV == s.Top_.Version {
		// This is a trivial submit
		return true, CantSubmitReasonNone, nil
	}
	latestVersionOfParent, err := s.GetLatest(c.ParentL, l)
	if err != nil {
		return false, CantSubmitReasonNone, err
	}
	if !latestVersionOfParent.IsSubmitted {
		return false, CantSubmitBeforeParent, nil
	}
	canRebase, err := s.r.CanRebaseWithoutConflict(c.TreeVersion,
		s.Top_.TreeVersion,
		c.ParentTreeVersion, l)
	if err != nil {
		return false, CantSubmitReasonNone, err
	}
	if !canRebase {
		return false, CantSubmitWouldCauseRebaseConflict, nil
	}
	return true, CantSubmitReasonNone, nil
}

var (
	errAlreadySubmitted       = errors.New("already submitted")
	errCantSubmitWithConflict = errors.New("can't submit commit with conflicts")
	errSubmitWouldConflict    = errors.New("submit would conflict")
	errParentNotSubmitted     = errors.New("parent not submitted")
)
