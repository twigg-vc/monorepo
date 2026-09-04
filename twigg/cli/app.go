package cli

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"monorepo/buildmeta"
	"monorepo/twigg/ansi"
	"monorepo/twigg/cli/clidb"
	"monorepo/twigg/cli/links"
	"monorepo/twigg/client"
	"monorepo/twigg/clistate"
	"monorepo/twigg/commit"
	"monorepo/twigg/tree"
	"monorepo/twigg/treev"
	"monorepo/twigg/workdir"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	okExitCode     = 0
	anyErrExitCode = 1
)

func run(systemRootPath *string, twiggWebUrl string, out io.Writer, in io.Reader) (exitCode int) {
	gob.Register(commit.Commit{})
	gob.Register(treev.TreeDataV{})
	gob.Register(tree.Data{})

	if out == nil {
		out = os.Stdout
	}
	if in == nil {
		in = os.Stdin
	}
	// Create an instance just to use the logError methods
	app := &app{out: out, in: in}
	defer func() {
		if app.logErrorWasCalled {
			exitCode = anyErrExitCode
		} else {
			exitCode = okExitCode
		}
	}()
	cliWorkdir, err := os.Getwd()
	if err != nil {
		app.logError("failed to get current directory: " + err.Error())
		return
	}
	executionPath := getExecutionPath(cliWorkdir, systemRootPath)
	// Create the minimal instance. The log methods already works; everything
	// else doesnt.
	app = newApp(twiggWebUrl, filepath.ToSlash(executionPath), out, in)
	// First arg is the binary name, so we remove it
	cmdLineArgs := os.Args[1:]

	// Print twigg version
	if len(cmdLineArgs) == 1 {
		if cmdLineArgs[0] == "version" ||
			cmdLineArgs[0] == "--version" ||
			cmdLineArgs[0] == "-version" ||
			cmdLineArgs[0] == "--v" ||
			cmdLineArgs[0] == "-v" {
			app.logInfo(buildmeta.Version)
			return
		}
		if cmdLineArgs[0] == helpFlag ||
			cmdLineArgs[0] == helpShortFlag {
			app.logInfo(readTheDocs(twiggWebUrl, app.supportsHyperlinks))
			return
		}
	}

	ok := app.initCommand(cmdLineArgs)
	if !ok {
		return
	}
	// Validate the args are ok
	ok = app.checkCommandArgs()
	if !ok {
		return
	}
	// Check if already initialized
	isInit := app.isInitialized()
	// The "isInit" command is an edge case. We just need to check if initialize
	// and can then return.
	if app.c.name == logIsInitCmd {
		if isInit {
			app.logSuccess(isInitLogMessage)
		} else {
			app.logWarning(repoNotInitialized)
		}
		return
	}
	if !isInit && !app.c.skipInitCheck {
		app.logError(repoNotInitialized)
		return
	}
	app.setupAndRun()
	return
}

type app struct {
	// ## Fields that are saved ##
	s clistate.State

	// ## Fields that are not saved ##
	db                        clidb.CliDb
	dbWrite                   context.Context
	twiggWebUrl               string
	forwardSlashExecutionPath string
	c                         command
	args                      commandArgs
	wl                        clidb.Ctx
	ag                        client.Client
	wd                        workdir.Workdir
	out                       io.Writer
	in                        io.Reader
	supportsHyperlinks        bool // indicates the terminal supports hyperlinks
	noColor                   bool // use to disable showing colors with ANSI codes

	onlyLogDebugOrError bool // If set, only "debug" and "error" logs are shown
	logErrorWasCalled   bool
}

// Creates an instance at the provided execution path.
// Init must be called before actually using it.
// "logError" fuctions already work
func newApp(twiggWebUrl, forwardSlashExecutionPath string, out io.Writer, in io.Reader) (ap *app) {
	return &app{
		twiggWebUrl:               twiggWebUrl,
		forwardSlashExecutionPath: forwardSlashExecutionPath,
		out:                       out,
		in:                        in,
		supportsHyperlinks:        links.Supports(),
		noColor:                   os.Getenv("NO_COLOR") != "",
	}
}

// Check if the app was already initialized in the specified directory or not
func (ap *app) isInitialized() bool {
	return hasFolder(ap.forwardSlashExecutionPath, storageFolder)
}

// Setup all the dependencies (load db, etc) and run.
func (ap *app) setupAndRun() {
	isClone := ap.c.name == cloneCmd
	if isClone {
		if ap.isInitialized() {
			ap.logError(repoAlreadyInitialized)
			return
		}
		cloneDir := path.Base(ap.args.serverPath)
		if cloneDir == "." || cloneDir == "/" {
			ap.logError(badServerPath)
			return
		}
		ap.forwardSlashExecutionPath = ap.forwardSlashExecutionPath + "/" + cloneDir
	}
	// Initialize the dependencies (db, workdir)
	db, closeDb, err := clidb.New(
		filepath.Join(ap.forwardSlashExecutionPath, storageFolder), "sqlarge.db")
	defer closeDb()
	if err != nil {
		ap.logError(err.Error())
		return
	}
	wl_, ul, commitTx, err := db.BeginWrite()
	defer ul()
	if err != nil {
		ap.logError(err.Error())
		return
	}
	ap.db = db
	ap.dbWrite = wl_
	wl := db.Bind(wl_)
	defer func() {
		if db.ShouldCommit(wl_) {
			err = commitTx()
			if err != nil {
				ap.logError("failed to commit tx: " + err.Error())
				return
			}
		}
	}()
	if err != nil {
		ap.logError(err.Error())
		return
	}
	const minSizeToCacheFile = 100 * 1024 // cache hashes of files >=100kB
	wd, err := workdir.New(ap.forwardSlashExecutionPath, wl, minSizeToCacheFile)
	if err != nil {
		ap.logError("failed to create working directory: " + err.Error())
		return
	}
	if isClone {
		// should be empty except for storage folder
		wd.Ignore(storageFolder + "/")
		cloneDirIsEmpty, err := wd.IsEmpty()
		wd.ClearIgnores()
		if err != nil {
			ap.logError("failed to check if clone dir is empty " + err.Error())
			return
		}
		if !cloneDirIsEmpty {
			ap.logError(cloneDirectoryIsNotEmpty)
			return
		}
	}
	// Set the dependencies
	ap.wd = wd
	ap.wl = wl
	ap.s = clistate.State{
		ServerUrl: ap.twiggWebUrl,
	}
	const repoOwner = "o"
	const repoId = 1
	ap.ag, err = client.New(repoOwner, repoId, wl)
	if err != nil {
		ap.logError(err.Error())
		return
	}
	// Load the previous state
	if ap.c.name != initCmd && ap.c.name != cloneCmd {
		err = ap.loadState()
		if err != nil {
			ap.logError(fmt.Sprintf("error loading state: %s", err))
			return
		}
	}
	// Perform all checks
	if !ap.c.skipIgnoreSetup {
		ap.wd.ClearIgnores()
		err := errors.Join(
			ap.wd.Ignore(storageFolder+"/"),
			ap.wd.Ignore(".sl/"),
			ap.wd.Ignore(".git/"),
		)
		if err != nil {
			ap.logError(
				fmt.Sprintf("error when setting storage folder ignore: %s", err))
			return
		}
	}
	if ap.c.checkCleanWorkdir && !ap.args.force {
		ok := ap.checkCleanWorkdir()
		if !ok {
			return
		}
	}
	// Finally, run the callback
	ap.logDebug(fmt.Sprintf("will run callback of: %s", ap.c.name))
	ap.c.callback(ap.args)
	ap.logDebug(fmt.Sprintf("done running callback of: %s", ap.c.name))
}

const storageFolder = ".twigg"

func removeAnsiIfNeeded(shouldRemove bool, s string) string {
	if !shouldRemove {
		return s
	}
	return ansi.Remove(s)
}

// When called, the ongoing db write lock will be informed that it should not
// be committed.
func (a *app) logError(msg string) {
	a.logErrorWasCalled = true
	if a.dbWrite != nil {
		a.db.PreventCommit(a.dbWrite)
	}
	a.out.Write([]byte(removeAnsiIfNeeded(
		a.noColor, ansi.Red.S()+msg+ansi.Reset.S()+"\n")))
}
func (a app) logSuccess(s string) {
	if a.onlyLogDebugOrError {
		return
	}
	a.out.Write([]byte(removeAnsiIfNeeded(
		a.noColor, ansi.Green.S()+s+ansi.Reset.S()+"\n")))
}
func (a app) logWarning(s string) {
	if a.onlyLogDebugOrError {
		return
	}
	a.out.Write([]byte(removeAnsiIfNeeded(
		a.noColor, ansi.Yellow.S()+s+ansi.Reset.S()+"\n")))
}
func (a app) logInfo(s string) {
	if a.onlyLogDebugOrError {
		return
	}
	a.out.Write([]byte(removeAnsiIfNeeded(a.noColor, s+"\n")))
}

// Only logs when running in debug mode (i.e. when `--debug` is used)
func (a app) logDebug(s string) {
	if !a.args.debug {
		return
	}
	a.out.Write([]byte(removeAnsiIfNeeded(a.noColor, "[DEBUG]: "+s+"\n")))
}

// Save the state of the app. Doesnt commit lock.
func (a app) saveState() error {
	return a.wl.SetCliState(a.s)
}

// Load the saved state of the app.
func (a *app) loadState() error {
	st, _, err := a.wl.GetCliState()
	if err != nil {
		return err
	}
	a.s = st
	return nil
}

// Finds the directory in which the command should be executed based on the
// directory in which the command line command was executed.
// It wont go up above "systemRoot" (use it for tests)
func getExecutionPath(osWd string, systemRoot *string) string {
	if hasFolder(osWd, storageFolder) {
		return osWd
	}

	child := osWd
	parent := getParentPath(child, systemRoot)
	for parent != child {
		if hasFolder(parent, storageFolder) {
			return parent
		}
		child = parent
		parent = getParentPath(parent, systemRoot)
	}

	return osWd
}

func hasFolder(path, folder string) bool {
	fileInfo, err := os.Stat(filepath.Join(path, folder))
	if err == nil {
		return fileInfo.IsDir()
	}
	return false
}

// Returns itself if it can't go up
func getParentPath(path string, systemRoot *string) string {
	if systemRoot != nil && path == *systemRoot {
		return path
	}
	return filepath.Dir(path)
}

// Returns true if everything is ok. Else, logs an error and returns false.
// Expects app.wl to be populated, otherwise panics.
func (a *app) parseCommit(s string) (commit.Commit, bool) {
	// Local commit syntax: C or CvV
	c, match, err := a.tryParseLocalCommit(s)
	if err != nil {
		a.logError(err.Error())
		return commit.Commit{}, false
	}
	if match {
		return c, true
	}

	// Global commit syntax: c/C or c/CvV
	c, match, err = a.tryParseServerCommit(s)
	if err != nil {
		a.logError(err.Error())
		return commit.Commit{}, false
	}
	if match {
		return c, true
	}

	// Alias commits: top, parent, etc.
	c, match, err = a.tryParseAliasCommit(s)
	if err != nil {
		a.logError(err.Error())
		return commit.Commit{}, false
	}
	if match {
		return c, true
	}

	a.logError(fmt.Sprintf("%q is not valid commit syntax", s))
	return commit.Commit{}, false
}

// (?i) → case-insensitive (v or V both match)
// ^#? → optional leading #
// (\d+) → captures the commit number C
// (?:v(\d+))? → optional v or V followed by the version number V
// $ → ensures it ends cleanly
var localCommitRegex = regexp.MustCompile(`(?i)^#?(\d+)(?:v(\d+))?$`)

// handles local syntax: "C", "CvV", "#C", "#CvV".
func (a *app) tryParseLocalCommit(s string) (c commit.Commit, match bool, err error) {
	// parses: C, CvV, #C, #CvV
	m := localCommitRegex.FindStringSubmatch(s)
	if m == nil {
		match = false
		return
	}
	match = true

	commitIdStr := m[1]
	commitId, err := strconv.ParseUint(commitIdStr, 10, 64)
	if err != nil {
		panic("regex failed to return valid number for commit id")
	}

	commitVersionStr := m[2]
	if commitVersionStr == "" {
		c, err = a.ag.GetLatest(commitId, a.wl)
		return
	} else {
		var commitVersion uint64
		commitVersion, err = strconv.ParseUint(commitVersionStr, 10, 64)
		if err != nil {
			panic("regex failed to return valid number for commit version")
		}
		c, err = a.ag.GetVersion(commitId, commitVersion, a.wl)
		return
	}
}

// (?i) → makes it case-insensitive (so C, c, V, v. all match).
// ^c/? → starts with c and an optional slash (/).
// (\d+) → captures the commit number C.
// (?:v(\d+))? → optionally matches v followed by a version number V.
// $ → ensures it ends right there.
var serverCommitRegex = regexp.MustCompile(`(?i)^c/?(\d+)(?:v(\d+))?$`)

func (a *app) tryParseServerCommitString(s string) (
	match bool, commitServerId uint64, hasVersion bool, commitServerVersion uint64) {
	m := serverCommitRegex.FindStringSubmatch(s)
	if m == nil {
		match = false
		return
	}
	match = true
	commitServerIdStr := m[1]
	commitServerId, err := strconv.ParseUint(commitServerIdStr, 10, 64)
	if err != nil {
		panic("regex failed to return valid number for server commit id")
	}
	commitServerVersionStr := m[2]
	if commitServerVersionStr == "" {
		return
	}
	hasVersion = true
	commitServerVersion, err = strconv.ParseUint(commitServerVersionStr, 10, 64)
	if err != nil {
		panic("regex failed to return valid number for server commit version")
	}
	return
}

// handles server syntax: "c/C", "c/CvV", "cC" and "cCvV".
func (a *app) tryParseServerCommit(s string) (c commit.Commit, match bool, err error) {
	// parses: c/C, c/CvV
	match, serverId, hasV, serverV := a.tryParseServerCommitString(s)
	if !match {
		return
	}
	if !hasV {
		c, err = a.ag.GetLatestByServerId(serverId, a.wl)
		return
	} else {
		c, err = a.ag.GetVersionByServerId(serverId, serverV, a.wl)
		return
	}
}

// handles aliases like "top" or "parent".
func (a *app) tryParseAliasCommit(s string) (commit.Commit, bool, error) {

	if isTopCommitAlias(s) {
		c, err := a.getTopCommit()
		return c, true, err
	}
	if isParentCommitAlias(s) {
		if a.s.Current.IsDetachedOrRoot() {
			return commit.Commit{}, false, errors.New(parentNotFound)
		}
		c, err := a.ag.GetVersion(a.s.Current.ParentL, a.s.Current.ParentV, a.wl)
		return c, true, err
	}
	if s == childCommitAlias {
		c, err := a.getNthChildCommit(1)
		return c, true, err
	}
	return commit.Commit{}, false, nil
}

func (a *app) commitHasExplicitVersion(s string) bool {
	return strings.Contains(s, "v") || strings.Contains(s, "V")
}
