package client

import (
	"monorepo/twigg/repo"
	"monorepo/twigg/workdir"
)

func (a tw) Load(A repo.TreeVersion, wd workdir.Workdir, l Read) error {
	return a.repo.Load(A, wd, l)
}