package cli

import (
	"errors"
	"fmt"
	"monorepo/twigg/client"
)

func (a *app) dump(args commandArgs) {
	filename := args.filename
	commit := a.s.Current
	if args.commit1 != "" {
		var parseCommitOk bool
		commit, parseCommitOk = a.parseCommit(args.commit1)
		if !parseCommitOk {
			return
		}
	}

	err := a.ag.Read(commit.TreeVersion, filename, a.out, a.wl)
	if errors.Is(err, client.ErrFileNotFound) {
		return
	}
	if err != nil {
		a.logError(err.Error())
		return
	}
}

func (a *app) logRootPath(args commandArgs) {
	a.logInfo(a.wd.Path())
}

func (a *app) logId(args commandArgs) {

	isClean, hash, err := a.workdirIsClean()
	if err != nil {
		a.logError(err.Error())
		return
	}
	const nHashBytesToLog = 6
	if !isClean {
		a.logInfo(fmt.Sprintf("dirty-workdir-%x", hash[:nHashBytesToLog]))
		return
	}
	if !a.s.Current.IsOnServer() {
		a.logInfo(fmt.Sprintf("not-pushed-%x", hash[:nHashBytesToLog]))
		return
	}
	a.logInfo(fmt.Sprintf("c%dv%d", a.s.Current.ServerL, a.s.Current.ServerV))
}