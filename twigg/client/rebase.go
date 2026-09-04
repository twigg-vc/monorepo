package client

import (
	"errors"
	"fmt"
	"monorepo/base/queue"
	"monorepo/twigg/commit"
	"monorepo/twigg/repo"
	"monorepo/twigg/tree"
)

func (ag *tw) Rebase(A, B *commit.Commit,
	isAutoRebaseOfChildren bool, rebaseChildren bool, l Write) (rebased commit.Commit, err error) {
	if A.IsSubmitted {
		err = errors.New("cant rebase submitted commit")
		return
	}
	if A.HasRebaseConflicts {
		err = errors.New("cant rebase commit with conflicts")
		return
	}
	if A.Status != commit.StatusLatest {
		err = errors.New("cant rebase obsolete commit")
		return
	}
	if A.IsDetached {
		err = errors.New("cant rebase detached commit")
		return
	}
	if B.HasRebaseConflicts {
		err = errors.New("cant rebase into commit with conflicts")
		return
	}
	latestA, _, err := l.GetLatestCommitByLocalId(ag.RepoId, A.L)
	if err != nil {
		return
	}
	if latestA.Version != A.Version {
		panic("rebase called with out-of-date commit")
	}

	// If B is a descendent of A, we manually set rebase rebaseChildren to false
	// to avoid an infinite recursion
	if rebaseChildren {
		var isGrampaIntoGrandchildRebase bool
		isGrampaIntoGrandchildRebase, err = ag.isGranchild(*A, *B, l)
		if err != nil {
			return
		}
		if isGrampaIntoGrandchildRebase {
			rebaseChildren = false
		}
	}

	var treeVersion uint64
	var hash [32]byte
	var conflict bool
	treeVersion, hash, conflict, err = ag.repo.Rebase(
		A.TreeVersion, commitLabelOnRebase(*A),
		B.TreeVersion, commitLabelOnRebase(*B), A.ParentTreeVersion,
		l)
	isNoChangeErr := errors.Is(err, repo.ErrNoChange)
	if err != nil && !isNoChangeErr {
		return
	}
	if isNoChangeErr {
		treeVersion = B.TreeVersion
		hash = B.RootDirHash
		conflict = false
	}
	diffCounts, err := tree.CountDiffs(
		ag.repo.Root(treeVersion, l),
		ag.repo.Root(B.TreeVersion, l))
	if err != nil {
		return
	}
	rebased = commit.NewRebase(
		/*isOnServer=*/ false,
		isAutoRebaseOfChildren,
		treeVersion,
		hash,
		conflict,
		A,
		B,
		diffCounts,
	)
	err = l.SetCommit(ag.QuotaOwner, ag.RepoId, *A)
	if err != nil {
		return
	}
	err = l.SetCommit(ag.QuotaOwner, ag.RepoId, *B)
	if err != nil {
		return
	}
	if rebaseChildren {
		err = ag.RebaseChildren(*A, &rebased, l)
		if err != nil {
			return
		}
	}
	err = l.SetCommit(ag.QuotaOwner, ag.RepoId, rebased)
	if err != nil {
		return
	}
	return
}

func commitLabelOnRebase(c commit.Commit) string {
	return fmt.Sprintf("#%dv%d", c.L, c.Version)
}

func (ag *tw) RebaseChildren(oldCommit commit.Commit, target *commit.Commit, l Write) error {
	if oldCommit.Status != commit.StatusObsolete {
		panic("RebaseChildren should be used on obsolete commits")
	}
	if target.L != oldCommit.L {
		panic("invalid commit L")
	}
	// Can't rebase the children if the new target got conflicts
	if target.HasRebaseConflicts {
		return nil
	}
	oldChildren := queue.New[commit.Commit]()
	for i, childId := range oldCommit.Children {
		child, err := ag.GetVersion(childId, oldCommit.ChildrenVersions[i], l)
		if err != nil {
			return err
		}
		if shouldSkipChild(child) {
			continue
		}
		oldChildren.Push(child)
	}
	newParents := queue.New[commit.Commit]()
	newParents.Push(*target)
	for !newParents.IsEmpty() {
		newParent := newParents.Pop()
		for !oldChildren.IsEmpty() && oldChildren.Peek().ParentL == newParent.L {
			oldChild := oldChildren.Pop()
			newChild, err := ag.Rebase(&oldChild, &newParent, true, false, l)
			if err != nil {
				return err
			}

			// if the rebase had no conflicts, keep rebasing the children
			if !newChild.HasRebaseConflicts && len(oldChild.Children) > 0 {
				newParents.Push(newChild)
				for i, childId := range oldChild.Children {
					child, err := ag.GetVersion(
						childId, oldChild.ChildrenVersions[i], l)
					if err != nil {
						return err
					}
					if shouldSkipChild(child) {
						continue
					}
					oldChildren.Push(child)
				}
			}

			// Update the input
			if newParent.L == target.L && newParent.Version == target.Version {
				(*target) = newParent
			}
		}

	}
	return nil
}

// Returns true if child is a (direct or transitive) child of grampa
func (ag *tw) isGranchild(grampa, child commit.Commit, l Read) (bool, error) {
	children := []commit.Commit{}
	for i := range grampa.Children {
		c, err := ag.GetVersion(grampa.Children[i], grampa.ChildrenVersions[i], l)
		if err != nil {
			return false, err
		}
		children = append(children, c)
		if c.L == child.L && c.Version == child.Version {
			return true, nil
		}
	}
	// Recurse on children
	for _, c := range children {
		cIsGrampa, err := ag.isGranchild(c, child, l)
		if err != nil {
			return false, err
		}
		if cIsGrampa {
			return true, nil
		}
	}
	return false, nil
}

// Some child commits should not be auto-rebased either because they can't be
// (such as those with conflicts) or because the user probably doesn't want
// them rebased (such as hidden ones). This method determines all the criteria.
func shouldSkipChild(child commit.Commit) bool {
	return child.Status == commit.StatusObsolete ||
		child.HasRebaseConflicts ||
		child.IsHidden
}
