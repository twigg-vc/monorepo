package cli

func (a *app) clean(args commandArgs) {
	isClean, _, err := a.workdirIsClean()
	if err != nil {
		a.logError(err.Error())
		return
	}
	if isClean {
		a.logInfo(nothingToClean)
		return
	}

	if !args.force {
		diffs, err := a.ag.DiffWorkdir(a.wd, a.s.Current.TreeVersion, a.wl)
		if err != nil {
			a.logError(err.Error())
			return
		}
		a.logInfo(workdirStatus)
		a.logDiff(diffs)
		if !a.askConfirmation(cleanConfirm) {
			a.logWarning(cleanAborted)
			return
		}
	}

	err = a.ag.Load(a.s.Current.TreeVersion, a.wd, a.wl)
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.logSuccess(cleanOk)
}
