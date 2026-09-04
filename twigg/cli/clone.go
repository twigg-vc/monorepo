package cli

import (
	"fmt"
)

func (a *app) clone(args commandArgs) {
	if a.ag.IsInit() {
		a.logError(repoAlreadyInitialized)
		return
	}
	if args.apiKey == "" && a.s.ApiKey == "" {
		ok := a.askApiKey(&args)
		if !ok {
			return
		}
	}

	a.onlyLogDebugOrError = true
	a.fistTimeInit(args)
	if a.logErrorWasCalled {
		return
	}

	a.s.ApiKey = args.apiKey

	a.s.ServerUrl = a.twiggWebUrl + args.serverPath
	args.commit0InServerSyntax = topCommitAlias
	a.pullCommit(args)
	if a.logErrorWasCalled {
		return
	}
	a.top(args)
	if a.logErrorWasCalled {
		return
	}

	err := a.saveState()
	if err != nil {
		a.logError(fmt.Sprintf("failed to save CLI state: %s", err))
		return
	}

	a.onlyLogDebugOrError = false
	a.logSuccess(cloneOk)
}