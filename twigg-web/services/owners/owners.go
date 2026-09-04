package owners

import (
	"bufio"
	"context"
	"errors"
	"io"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg/commit"
	"monorepo/twigg/server"
	"monorepo/twigg/tree"
	"path"
	"slices"
	"strings"
)

type service struct {
	sp ServerProvider
}

func (s service) OwnersLgmtIsOk(repoId, commitId uint64,
	usersWhoLgtmd []string,
	commitIdToReadOwners uint64,
	supremeLeaders []string,
	sr context.Context) (bool, error) {

	if len(supremeLeaders) > permissions.MaxNumberOfOwnersInOrg {
		return false, errors.New("too many supreme leaders")
	}

	if len(supremeLeaders) > 0 {
		allLgtmd := true
		for _, leader := range supremeLeaders {
			if !slices.Contains(usersWhoLgtmd, leader) {
				allLgtmd = false
				break
			}
		}
		if allLgtmd {
			return true, nil
		}
	}
	r := s.sp.GetServerRead(sr)
	srv, err := s.sp.GetServerByRepoId(sr, repoId)
	if err != nil {
		return false, err
	}
	c, err := srv.GetLatest(commitId, r)
	if err != nil {
		return false, err
	}
	if c.IsSubmitted {
		return true, err
	}
	parent, err := srv.GetVersion(c.ParentL, c.ParentV, r)
	if err != nil {
		return false, err
	}
	cOwners, err := srv.GetLatest(commitIdToReadOwners, r)
	if err != nil {
		return false, err
	}
	diffs, err := srv.Diff(c, parent, r)
	if err != nil {
		return false, err
	}
	for diffs.CanGet() {
		modifedTreePath, _, visit, modifedTree := diffs.Get()
		if visit == tree.SecondVisit {
			err := diffs.Next()
			if err != nil {
				return false, err
			}
			continue
		}
		if diffs.GetDiff().Type == tree.DiffTypeUndefined {
			// trees in commits are stored with all data known even for dirs
			panic("got undefined diff")
		}
		if diffs.GetDiff().Type == tree.DiffTypeNoChange {
			diffs.SkipChildrenOnNext()
			err := diffs.Next()
			if err != nil {
				return false, err
			}
			continue
		}
		isApproved, childPathsAreApproved, err := isApprovedPath(
			modifedTreePath, usersWhoLgtmd, srv, cOwners, r)
		if err != nil {
			return false, err
		}
		// If a file is not approved, we already return false.
		// If a directory is not approved, however, we don't return false;
		// because ultimatelly the approval must be of files; else we'd
		// be requiring the root OWNER to approve all changes for example.
		if !modifedTree.Data().IsDir {
			if !isApproved {
				return false, nil
			}
		} else {
			if isApproved && childPathsAreApproved {
				diffs.SkipChildrenOnNext()
			}
		}
		err = diffs.Next()
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

// If there are no OWNERS in the path from the root until treePath, returns
// `true, false`. Returns `true, true` if there is some OWNERS file in the path;
// which implies `treePath` is approved and also any child path from it.
func isApprovedPath(
	treePath string,
	usersWhoLgtmd []string,
	srv server.Server,
	cOwners commit.Commit,
	r server.Read) (isApproved bool, childPathsAreApproved bool, err error) {
	hasAnyOwnersFile := false
	shouldBreak := func() bool {
		return (treePath == "." || treePath == "/")
	}
	for {
		// Get the directory/file at the owners commit. We'll use it to
		// read the OWNERS file
		var treeAtOwners tree.Tree
		treeAtOwners, err = srv.GetTree(cOwners, treePath, r)
		if err != nil && !errors.Is(err, tree.ErrTreeNotFound) {
			return
		}
		// If not found, "walk up"
		if errors.Is(err, tree.ErrTreeNotFound) || !treeAtOwners.Data().IsDir {
			if shouldBreak() {
				break
			}
			treePath = path.Dir(treePath)
			continue
		}
		// If found, check if the directory has some OWNERS file
		var hasOwnersFile bool
		var lgtmsSatisfyOwnersFile bool
		hasOwnersFile, lgtmsSatisfyOwnersFile, err = checkDir(
			treePath, treeAtOwners, usersWhoLgtmd, srv, cOwners, r)
		if err != nil {
			return
		}
		if hasOwnersFile {
			hasAnyOwnersFile = true
		}
		if hasOwnersFile && lgtmsSatisfyOwnersFile {
			isApproved = true
			childPathsAreApproved = true
			return
		}
		if shouldBreak() {
			break
		}
		treePath = path.Dir(treePath)
	}
	isApproved = !hasAnyOwnersFile
	childPathsAreApproved = false
	err = nil
	return
}

func checkDir(
	dirTreePath string,
	dirTree tree.Tree,
	usersWhoLgtmd []string,
	srv server.Server,
	cOwners commit.Commit,
	r server.Read) (dirHasOwnersFile bool, lgtmsSatisfyOwners bool, err error) {
	if !dirTree.Data().IsDir {
		panic("isDirApproved called for non dir tree")
	}
	// If no OWNERS file exists in this specific directory
	if !directoryTreeHasOwnersChild(dirTree.Data()) {
		dirHasOwnersFile = false
		err = nil
		return
	}
	// Read the actual OWNERS file
	ownersTr, err := srv.GetTree(cOwners, path.Join(dirTreePath, OwnersFileName), r)
	if err != nil {
		return
	}
	// Edge cases: a directory named "OWNERS" or a file that is too large
	// This is handled as if there were no OWNERS file
	if ownersTr.Data().IsDir || ownersTr.Data().Size > MaxOwnersFileSize {
		dirHasOwnersFile = false
		err = nil
		return
	}
	file, err := ownersTr.GetFile()
	if err != nil {
		return
	}
	dirHasOwnersFile = true
	parser := ownersFileParser{file: file}
	lgtmsSatisfyOwners, err = parser.isApproved(usersWhoLgtmd)
	return
}

func directoryTreeHasOwnersChild(trData tree.Data) bool {
	if !trData.IsDir {
		panic("got non dir tree")
	}
	for _, childName := range trData.ChildrenBaseNames {
		if childName == OwnersFileName {
			return true
		}
	}
	return false
}

type ownersFileParser struct {
	file io.WriterTo
}

func (o ownersFileParser) isApproved(approvers []string) (bool, error) {
	pr, pw := io.Pipe()
	go func() {
		_, err := o.file.WriteTo(pw)
		_ = pw.CloseWithError(err)
	}()
	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if slices.Contains(approvers, line) {
			_ = pr.Close()
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, nil
}
