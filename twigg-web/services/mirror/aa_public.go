package mirror

import (
	"monorepo/twigg/server"
)

type GitMirrorService struct {
	s *gitMirror
}

func (g GitMirrorService) PushTopCommit(serverRead server.Read, srv server.Server,
	gitRepoUrl string,
	maxWorkdirSizeAllowed int64) error {
	return g.s.PushTopCommit(serverRead, srv, gitRepoUrl, maxWorkdirSizeAllowed)
}

func New(absPathToEmptyFolder string) (GitMirrorService, error) {
	s, err := newService(absPathToEmptyFolder)
	return GitMirrorService{s}, err
}
