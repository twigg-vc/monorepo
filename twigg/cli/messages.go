package cli

import (
	"fmt"
	"monorepo/twigg/ansi"
	"monorepo/twigg/cli/links"
	"monorepo/twigg/commit"
	"monorepo/twigg/tree"
)

const (
	repoNotInitialized         = "repository not initialized. Call `init` first"
	isInitLogMessage           = "ok"
	repoAlreadyInitialized     = "repository already initialized"
	repoCreated                = "repository created :)"
	cloneOk                    = "repository sucessfully cloned :)"
	cloneDirectoryIsNotEmpty   = "clone directory is not empty"
	rebaseOk                   = "rebase succeeded"
	rebaseIntoParent           = "can't rebase into parent"
	cantRebaseIntoConflicts    = "can't rebase into commit with conflicts"
	rebaseRoot                 = "can't rebase the root commit"
	rebaseIntoSelf             = "can't rebase into itself"
	rebaseWillSucceed          = "rebase will have no conflicts :)"
	rebaseWillConflict         = "rebase will cause conflicts"
	rebaseWithObsolete         = "can't perform rebase with obsolete commits"
	invalidCommand             = "invalid command"
	workdirIsDirty             = "workdir is dirty. Create a commit or use -f to discard"
	nothingToClean             = "workdir is already clean"
	cleanConfirm               = "this will discard the changes above. Continue? [y/N]"
	cleanAborted               = "clean aborted"
	cleanOk                    = "workdir cleaned"
	conflictsStatus            = "conflicts:"
	workdirStatus              = "changes in working directory:"
	commitTreeStatus           = "current commit tree:"
	noChanges                  = "no changes"
	allNotSupportedWithJson    = "--all is not supported with --json"
	commitNotFound             = "commit not found"
	gotConflict                = "got conflict"
	parentNotFound             = "parent not found"
	commitIsDetached           = "commit is detached"
	alreadyAtTop               = "you're already at the top"
	nothingToCommit            = "nothing to commit"
	nothingToPull              = "already up to date"
	childNotFound              = "child not found"
	pushOk                     = "push succeeded"
	pullOk                     = "pull succeeded"
	currentCommitAlreadyPushed = "current commit was already pushed"
	commitVersionNotProvided   = "commit version not provided"
	serverUrlNotSet            = "server url not set. Use `server https://<...>` to set it"
	apiKeyNotSet               = "API Key not configured. Use `key <your key>` to set it"
	badApiKey                  = "API Key is outdated. Use `key <your key>` to updated it"
	updateRequired             = "Twigg is outdated. Install the newest version"
	apiKeyConfiguredOk         = "API Key sucessfully configured"
	messageNotProvided         = "message wasn't provided"
	messageCantHaveFlagPrefix  = "message can't start with `-`"
	targerCommitNotProvided    = "target commit wasn't provided"
	fileNotProvided            = "filename wasn't provided"
	serverUrlNotProvided       = "server URL wasn't provided"
	badServerPath              = "invalid server Path"
	apiKeyNotProvided          = "API Key wasn't provided"
	boolNotProvided            = "bool value wasn't provided"
	invalidBoolValue           = "invalid bool value"
	serverIdNotProvided        = "server id wasn't provided"
	invalidServerId            = "invalid server id"
	alreadyHidden              = "already hidden"
	cantHideSubmitted          = "submitted commits can't be hidden"
	notHidden                  = "is not hidden"
	cantPushWithObsParent      = "can't push commit with obsolete parent. Rebase the commit to the new version of the parent first"
	firstCommitMsg             = "[Initialize]"
	cantCommitOnTopOfConflicts = "can't create commit on top of commit with conflicts. Did you mean to `amend`?"
	cantRestoreSubmitted       = "can't restore old version of submitted commit"
	badCommitServerSyntax      = "invalid server commit syntax"
	cantAmendWithConflicts     = "can't amend with unresolved conflicts"
	noCiWillRun                = "no CI job affected"
	badNumberMsgPrefix         = "bad number:"
)

func setEnableUnsafeDevModeOk(enableUnsafeDevMode bool) string {
	return fmt.Sprintf("enable unsafe dev mode = %v", enableUnsafeDevMode)
}
func setServerUrlOk(url string) string {
	return fmt.Sprintf("server url updated to %s", url)
}
func setServerIdOk(id uint64) string {
	return fmt.Sprintf("next server id set to %d", id)
}
func commitString(c commit.Commit, colored bool,
	supportsHyperlink bool, serverUrl string, onlyShowServerId bool) string {
	return commitStringByLAndV(c.L, c.Version, c.HasServerL,
		c.ServerL, c.HasServerV, c.ServerV, colored,
		supportsHyperlink, serverUrl, onlyShowServerId)
}
func commitStringByLAndV(
	l commit.LocalId, version uint64,
	hasServerL bool, serverL commit.LocalId,
	hasServerV bool, serverV uint64, colored bool,
	supportsHyperlink bool, serverUrl string,
	onlyShowServerId bool) string {
	if onlyShowServerId && (!hasServerL || !hasServerV) {
		panic("used onlyShowServerId without server id or version")
	}
	localIdColor := ansi.Blue
	serverIdColor := ansi.Magenta
	resetColor := ansi.Reset
	if !colored {
		localIdColor = ""
		serverIdColor = ""
		resetColor = ""
	}
	var localId string
	if onlyShowServerId {
		localId = ""
	} else {
		localId = fmt.Sprintf("%s#%dv%d%s", localIdColor, l, version, resetColor)
	}
	if !hasServerL && hasServerV {
		panic("somehow got serverV but not serverL")
	}
	if !hasServerL {
		return localId
	}

	if localId != "" {
		localId += " "
	}
	if !hasServerV {
		if supportsHyperlink {
			return fmt.Sprintf("%s%s%s%s",
				localId, serverIdColor,
				links.GetHyperlink(
					fmt.Sprintf("c/%d", serverL),
					fmt.Sprintf("%s/c/%d", serverUrl, serverL),
				),
				resetColor)
		}
		return fmt.Sprintf("%s%sc/%d%s",
			localId, serverIdColor, serverL, resetColor)
	}
	if supportsHyperlink {
		return fmt.Sprintf("%s%s%s%s",
			localId, serverIdColor,
			links.GetHyperlink(
				fmt.Sprintf("c/%dv%d", serverL, serverV),
				fmt.Sprintf("%s/c/%d?tab=changes&right=%d",
					serverUrl, serverL, serverV),
			),
			resetColor)
	}
	return fmt.Sprintf("%s%sc/%dv%d%s",
		localId, serverIdColor, serverL, serverV, resetColor)
}
func switchedToCommit(c commit.Commit, supportsHyperlink bool, serverUrl string) string {
	return fmt.Sprintf("%vswitched to commit %v%v",
		ansi.Green, commitString(c, true, supportsHyperlink, serverUrl,
			/*onlyShowServerId*/ false), ansi.Reset)
}
func loadedCommit(c commit.Commit, supportsHyperlink bool, serverUrl string) string {
	return fmt.Sprintf("%vloaded commit %s into the working directory%v",
		ansi.Green, commitString(c, true, supportsHyperlink, serverUrl,
			/*onlyShowServerId*/ false), ansi.Reset)
}
func warpedToCommit(c commit.Commit, supportsHyperlink bool, serverUrl string) string {
	return fmt.Sprintf("%vswitched to commit %v (workding directory was not modified)%v",
		ansi.Green, commitString(c, true, supportsHyperlink, serverUrl,
			/*onlyShowServerId*/ false), ansi.Reset)
}
func hidCommit(c commit.Commit, supportsHyperlink bool, serverUrl string) string {
	return fmt.Sprintf("%v %s is now hidden%v",
		ansi.Green, commitString(c, false, supportsHyperlink, serverUrl,
			/*onlyShowServerId*/ false), ansi.Reset)
}
func unhideCommit(c commit.Commit, supportsHyperlink bool, serverUrl string) string {
	return fmt.Sprintf("%vcommit %s is now unhidden%v",
		ansi.Green, commitString(c, true, supportsHyperlink, serverUrl,
			/*onlyShowServerId*/ false), ansi.Reset)
}
func createdCommit(c commit.Commit, supportsHyperlink bool, serverUrl string) string {
	return fmt.Sprintf("created commit %v",
		commitString(c, true, supportsHyperlink, serverUrl,
			/*onlyShowServerId*/ false))
}
func ammendedCommit(old, new commit.Commit, supportsHyperlink bool, serverUrl string) string {
	return fmt.Sprintf("ammended commit %s -> %s",
		commitString(old, true, supportsHyperlink, serverUrl,
			/*onlyShowServerId*/ false),
		commitString(new, true, supportsHyperlink, serverUrl,
			/*onlyShowServerId*/ false))
}
func pulledCommit(c commit.Commit, supportsHyperlink bool, serverUrl string) string {
	return fmt.Sprintf("pulled commit %s",
		commitString(c, true, supportsHyperlink, serverUrl,
			/*onlyShowServerId*/ true))
}
func restoredCommit(restoredVersion uint64, restored commit.Commit,
	supportsHyperlink bool, serverUrl string) string {
	return fmt.Sprintf("restored version %d of commit %d -> %s",
		restoredVersion, restored.L,
		commitString(restored, true, supportsHyperlink, serverUrl,
			/*onlyShowServerId*/ false))
}
func instructToPullParent(detached commit.Commit) string {
	return fmt.Sprintf("parent not found. Run `pull c%dv%d --stay`",
		detached.ParentServerL, detached.ParentServerV)
}

func fileHasUnresolvedConflicts(path string) string {
	return fmt.Sprintf("%s has unresolved conflicts", path)
}
func fileHasResolvedConflicts(path string) string {
	return fmt.Sprintf("%s conflicts resolved :)", path)
}

func diffTypeText(d tree.DiffType) string {
	switch d {
	case tree.DiffTypeCreated:
		return "created"
	case tree.DiffTypeDeleted:
		return "deleted"
	case tree.DiffTypeAnyModified:
		return "modified"
	case tree.DiffTypeNoChange:
		return "no change"
	}
	panic("text not implemented")
}

func fileStatus(shortFileName string, d tree.DiffType) string {
	color := ""
	switch d {
	case tree.DiffTypeCreated:
		color = ansi.Green.String()
	case tree.DiffTypeDeleted:
		color = ansi.Red.String()
	case tree.DiffTypeAnyModified:
		color = ansi.Yellow.String()
	case tree.DiffTypeNoChange:
		color = ansi.White.String()
	default:
		panic("color not implemented")
	}
	return fmt.Sprintf("%v: %v%v%v",
		shortFileName, color, diffTypeText(d), ansi.Reset)
}

func readTheDocs(serverUrl string, supportsHyperlink bool) string {
	if supportsHyperlink {
		return fmt.Sprintf("read the docs at %s", links.GetHyperlink(serverUrl+"/docs/v/2", serverUrl+"/docs/v/2"))
	}
	return fmt.Sprintf("read the docs at %s", serverUrl+"/docs/v/2")
}

func ciListHeader(c commit.Commit, supportsHyperlink bool, serverUrl string) string {
	return fmt.Sprintf("%s will affects the following CI jobs:",
		commitString(c,
			/*colored*/ false, supportsHyperlink, serverUrl,
			/*onlyShowServerId*/ false))
}