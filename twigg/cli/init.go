package cli

// Initializes the repository in the current working directory
// This should only be executed once, as it creates the neccessary folders.
func (a *app) fistTimeInit(args commandArgs) {
	if a.ag.IsInit() {
		a.logWarning(repoAlreadyInitialized)
		return
	}
	// Create the root commit
	root, err := a.ag.Init(a.wl)
	if err != nil {
		a.logError(err.Error())
		return
	}
	// Set the current commit and save the state
	a.s.Current = root
	err = a.saveState()
	if err != nil {
		a.logError(err.Error())
		return
	}

	a.logSuccess(repoCreated)
}
