package server

import (
	"errors"
	"monorepo/twigg/commit"
	"monorepo/twigg/workdir"
)

func (s *srv) Load(cId commit.LocalId, wd workdir.Workdir, l Read) error {
	if !s.WasInit() {
		return errors.New("not initialized")
	}

	c, err := s.GetLatest(cId, l)
	if err != nil {
		return err
	}

	return s.r.Load(c.TreeVersion, wd, l)
}
