package cli

import (
	"errors"
	"fmt"
	"io"
	"monorepo/twigg/cli/links"
	"regexp"
	"strconv"
	"strings"
)

// Command names
const (
	initCmd                = "init"
	logIsInitCmd           = "is-init"
	cloneCmd               = "clone"
	logRootPathCmd         = "root"
	logCmd                 = "log"
	statusCmd              = "status"
	statusShortCmd         = "st"
	dumpCmd                = "dump"
	diffCmd                = "diff"
	commitCmd              = "commit"
	amendCmd               = "amend"
	restoreCmd             = "restore"
	gotoCmd                = "goto"
	upCmd                  = "up"
	downCmd                = "down"
	topCmd                 = "top"
	warpCmd                = "warp"
	loadCmd                = "load"
	cleanCmd               = "clean"
	hideCmd                = "hide"
	unhideCmd              = "unhide"
	rebaseCmd              = "rebase"
	pullCmd                = "pull"
	pushCmd                = "push"
	setServerUrlCmd        = "server"
	setUnsafeServerUrlCmd  = "unsafe-server"
	setApiKeyCmd           = "key"
	setServerIdCmd         = "set-server-id"
	dumpApiKeyCmd          = "key-dump"
	idCmd                  = "id"
	ciListCmd              = "ci-list"
	ciListShortCmd         = "cil"
	enableUnsafeDevModeCmd = "enable-unsafe-dev-mode"
)

// Flags
const (
	forceFlag        = "--force"
	forceShortFlag   = "-f"
	yoloFlag         = "--yolo"
	dryRunFlag       = "--dry-run"
	dryRunShortFlag  = "-d"
	filterFlag       = "--filter"
	filterShortFlag  = "-fi"
	allFlag          = "--all"
	allShortFlag     = "-a"
	jsonFlag         = "--json"
	versionFlag      = "--version"
	versionShortFlag = "-v"
	helpFlag         = "--help"
	helpShortFlag    = "-h"
	debugModeFlag    = "--debug"
	stayFlag         = "--stay"
)

// Commit aliases
const (
	parentCommitAlias0      = "parent"
	parentCommitShortAlias0 = "p"
	parentCommitAlias1      = "down"
	childCommitAlias        = "up"
	topCommitAlias          = "top"
)

const maxNumberAllowedInCommandArgs = 100

func isParentCommitAlias(arg string) bool {
	return arg == parentCommitAlias0 ||
		arg == parentCommitShortAlias0 ||
		arg == parentCommitAlias1
}

func isTopCommitAlias(arg string) bool {
	return arg == topCommitAlias
}

// Regex for inline filter syntax
var inlineFilterRegex = regexp.MustCompile(
	fmt.Sprintf(`^(%s|%s)=(.+)$`, filterFlag, filterShortFlag))

// Regex for inline version syntax (v=<number>, version=<number>)
var inlineVersionRegex = regexp.MustCompile(
	fmt.Sprintf(`^(%s|%s)=(.+)$`, versionFlag, versionShortFlag))

// Regex for inlineV syntax (v<number>)
var inlineVRegex = regexp.MustCompile(
	fmt.Sprintf(`^(%s|%s)([^=]+)$`, versionFlag, versionShortFlag))

// If not provided, the arguments are set to ""
type commandArgs struct {
	commit0               string // commitId or one of the aliases (parent, up, down)
	commit0InServerSyntax string // commitId in server syntax (top, c/X, cX, cXvY, etc)
	commit1               string // commitId or one of the aliases (parent, up, down)
	message               string
	filename              string
	// if not pass set to -1. Limit by maxNumberAllowedInCommandArgs
	number int
	// Path under twiggWebUrl
	serverPath      string
	unsafeServerUrl string
	apiKey          string
	boolInFirstArg  bool
	serverId        uint64

	dryRun        bool
	force         bool
	all           bool
	json          bool
	version       uint64
	hasVersion    bool
	pathsToFilter []string
	stay          bool

	// used to show debug messages
	debug bool
}

// Struct that declarativelly registers a command
type command struct {
	// Actual name of the command
	name string
	// first argument might be a number. Else, no other arg is provided.
	firstArgMightBeNumber bool
	// first argument might be a message. Else, no other arg is provided.
	firstArgMightBeMessage bool
	// first argument must be a message.
	firstArgIsMessage bool
	// requires a commit identifier as first argument
	firstArgIsCommit bool
	// first argument might be a commit identifier in server syntax
	// (including `top` alias). Else, no other arg is provided.
	firstArgMightBeServerCommit bool
	// the first argument is either a commit or isn't provided.
	// if not provided, the current commit is used.
	firstArgIsCommitOrCurrent bool
	// a first argument that is a commit might be provided
	firstArgMightBeCommit bool
	// a second argument that is a commit might be provided
	secondArgMightBeCommit bool
	// first argument must be a filename
	firstArgIsFile bool
	// first argument must be the server path
	firstArgIsServerPath bool
	// first argument is unsafe url
	firstArgIsUnsafeURL bool
	// first argument must be the api key
	firstArgIsApiKey bool
	// first argument must be a server id (a positive number)
	firstArgIsServerId bool
	// first argument must be bool
	firstArgIsBool bool
	// a second argument might be provided. If so, it's the api key.
	secondArgMightBeApiKey bool
	// supports "--yolo"
	supportsForceFlag bool
	// supports "--dry-run"
	supportsDryRunFlag bool
	// supports "--filter=<...>"
	supportsFilterFlag bool
	// supports "--all"
	supportsAllFlag bool
	// supports "--json"
	supportsJsonFlag bool
	// supports "--version"
	supportsVersionFlag bool
	// supports "--stay" flag
	supportsStayFlag bool

	// Function that is executed.
	callback func(c commandArgs)
	// If set to true, will skip setting the ignores
	skipIgnoreSetup bool
	// If set to true, doesn't first check that the repository was initialized
	skipInitCheck bool
	// If true, logs an error if the workdir is dirty and --force is not used.
	// --force skips this check.
	checkCleanWorkdir bool
}

// Sets the command and command args in the app by parsing the
// command line arguments. cmdLineArgs should not include the binary name (
// os.Args by default contain the binary name followed by the args.)
// Logs err and returns false on any error.
func (a *app) initCommand(cmdLineArgs []string) (ok bool) {
	// Runs `log` by default if no command is provided
	if len(cmdLineArgs) == 0 {
		cmdLineArgs = []string{logCmd}
	}
	// Runs `log <flags>` by default if only a flag is provided
	if strings.HasPrefix(cmdLineArgs[0], "-") {
		cmdLineArgs = append([]string{logCmd}, cmdLineArgs...)
	}
	c := command{}
	switch cmdLineArgs[0] {
	case initCmd:
		c = command{
			callback:        a.fistTimeInit,
			skipIgnoreSetup: true,
			skipInitCheck:   true,
		}
	case cloneCmd:
		c = command{
			callback:               a.clone,
			firstArgIsServerPath:   true,
			secondArgMightBeApiKey: true,
			skipInitCheck:          true,
		}
	case logIsInitCmd:
		// IsInit is an edge case as we handle it before initializing anything
		// else, so we just need to populate the command name (which is
		// done after this switch-case statement).
		c = command{}
	case logCmd:
		c = command{
			callback:              a.log,
			firstArgMightBeNumber: true,
			supportsAllFlag:       true,
			supportsJsonFlag:      true,
		}
	case commitCmd:
		c = command{
			firstArgIsMessage:  true,
			supportsFilterFlag: true,
			callback:           a.commit,
		}
	case upCmd:
		c = command{
			supportsForceFlag:     true,
			checkCleanWorkdir:     true,
			firstArgMightBeNumber: true,
			callback:              a.up,
		}
	case downCmd:
		c = command{
			supportsForceFlag:     true,
			checkCleanWorkdir:     true,
			firstArgMightBeNumber: true,
			callback:              a.down,
		}
	case gotoCmd:
		c = command{
			firstArgIsCommit:  true,
			supportsForceFlag: true,
			checkCleanWorkdir: true,
			callback:          a.goto_,
		}
	case warpCmd:
		c = command{
			firstArgIsCommit:  true,
			checkCleanWorkdir: false,
			callback:          a.warp,
		}
	case hideCmd:
		c = command{
			firstArgIsCommit: true,
			callback:         a.hide,
		}
	case unhideCmd:
		c = command{
			firstArgIsCommit: true,
			callback:         a.unhide,
		}
	case loadCmd:
		c = command{
			firstArgIsCommit:  true,
			supportsForceFlag: true,
			checkCleanWorkdir: true,
			callback:          a.load,
		}
	case topCmd:
		c = command{
			supportsForceFlag: true,
			checkCleanWorkdir: true,
			callback:          a.top,
		}
	case statusCmd, statusShortCmd:
		c = command{
			callback: a.status,
		}
	case cleanCmd:
		c = command{
			supportsForceFlag: true,
			callback:          a.clean,
		}
	case diffCmd:
		c = command{
			firstArgMightBeCommit:  true,
			secondArgMightBeCommit: true,
			supportsAllFlag:        true,
			supportsJsonFlag:       true,
			callback:               a.diff,
		}
	case amendCmd:
		c = command{
			firstArgMightBeMessage: true,
			supportsForceFlag:      true,
			supportsFilterFlag:     true,
			callback:               a.amend,
		}
	case restoreCmd:
		c = command{
			firstArgIsCommit: true,
			callback:         a.restore,
		}
	case rebaseCmd:
		c = command{
			// If one arg is provided: the arg must be the `target` commit
			// If two args are provided: first is `source` second is `target`
			firstArgIsCommit:       true,
			secondArgMightBeCommit: true,
			supportsDryRunFlag:     true,
			checkCleanWorkdir:      true,
			callback:               a.rebase,
		}
	case dumpCmd:
		c = command{
			firstArgIsFile:         true,
			secondArgMightBeCommit: true,
			callback:               a.dump,
		}
	case logRootPathCmd:
		c = command{
			callback: a.logRootPath,
		}
	case pushCmd:
		c = command{
			callback: a.push,
		}
	case pullCmd:
		c = command{
			callback:                    a.pull,
			firstArgMightBeServerCommit: true,
			supportsStayFlag:            true,
		}
	case setServerUrlCmd:
		c = command{
			firstArgIsServerPath: true,
			callback:             a.setServerUrl,
		}
	case setUnsafeServerUrlCmd:
		c = command{
			firstArgIsUnsafeURL: true,
			callback:            a.setUnsafeServerUrl,
		}
	case setApiKeyCmd:
		c = command{
			firstArgIsApiKey: true,
			callback:         a.setApiKey,
		}
	case dumpApiKeyCmd:
		c = command{
			callback: a.dumpApiKey,
		}
	case setServerIdCmd:
		c = command{
			firstArgIsServerId: true,
			callback:           a.setServerId,
		}
	case idCmd:
		c = command{
			callback: a.logId,
		}
	case enableUnsafeDevModeCmd:
		c = command{
			firstArgIsBool: true,
			callback:       a.setEnableUnsafeDevModeCmd,
		}
	case ciListCmd, ciListShortCmd:
		c = command{
			firstArgIsCommitOrCurrent: true,
			callback:                  a.ciList,
		}
	default:
		a.logError(invalidCommand)
		return
	}
	c.name = cmdLineArgs[0]
	checkCommandOrDie(c)
	// Remove the first argument, as that's the command itself
	args, err := parseCmdLineArgs(c, cmdLineArgs[1:])
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.c = c
	a.args = args
	ok = true
	return
}

func parseCmdLineArgs(c command, cmdLineArgs []string) (cArgs commandArgs, err error) {
	argsWithoutFlags, err := parseFlags(c, &cArgs, cmdLineArgs)
	if err != nil {
		return
	}
	err = parseArgs(c, &cArgs, argsWithoutFlags)
	return
}

// Sets the flags in the command and removes the flag arguments from the
// cmdLineArgs
func parseFlags(cmd command, c *commandArgs, args []string) (cmdLineArgsWithoutFlags []string, err error) {
	cmdLineArgsWithoutFlags = make([]string, 0, len(args))
	i := 0
	argIndex := 0
	for argIndex < len(args) {
		if args[argIndex] == debugModeFlag {
			c.debug = true
			argIndex += 1
			continue
		}
		if args[argIndex] == dryRunFlag || args[argIndex] == dryRunShortFlag {
			if !cmd.supportsDryRunFlag {
				err = fmt.Errorf("%s not supported by this command", dryRunFlag)
				return
			}
			c.dryRun = true
			argIndex += 1
			continue
		}
		if args[argIndex] == stayFlag {
			if !cmd.supportsStayFlag {
				err = fmt.Errorf("%s not supported by this command", stayFlag)
				return
			}
			c.stay = true
			argIndex += 1
			continue
		}
		if args[argIndex] == forceFlag ||
			args[argIndex] == forceShortFlag ||
			args[argIndex] == yoloFlag {
			if !cmd.supportsForceFlag {
				err = fmt.Errorf("%s not supported by this command", forceFlag)
				return
			}
			c.force = true
			argIndex += 1
			continue
		}
		if args[argIndex] == allFlag || args[argIndex] == allShortFlag {
			if !cmd.supportsAllFlag {
				err = fmt.Errorf("%s not supported by this command", allFlag)
				return
			}
			c.all = true
			argIndex += 1
			continue
		}
		if args[argIndex] == jsonFlag {
			if !cmd.supportsJsonFlag {
				err = fmt.Errorf("%s not supported by this command", jsonFlag)
				return
			}
			c.json = true
			argIndex += 1
			continue
		}
		if strings.HasPrefix(args[argIndex], filterFlag) ||
			strings.HasPrefix(args[argIndex], filterShortFlag) {
			if !cmd.supportsFilterFlag {
				err = fmt.Errorf("%s not supported by this command", filterFlag)
				return
			}

			if len(c.pathsToFilter) > 0 {
				err = fmt.Errorf("filter was specified twice")
				return
			}
			// If it's an inline format (filter=<filter string>), just parse it
			if matches := inlineFilterRegex.FindStringSubmatch(args[argIndex]); matches != nil {
				c.pathsToFilter, err = parseFilter(matches[2])
				if err != nil {
					return
				}
				argIndex += 1
				continue
			}
			// Else, we must read the following argument
			if argIndex == len(args)-1 {
				err = fmt.Errorf("filter not provided")
				return
			}
			c.pathsToFilter, err = parseFilter(args[argIndex+1])
			if err != nil {
				return
			}
			argIndex += 2
			continue
		}
		if strings.HasPrefix(args[argIndex], versionFlag) ||
			strings.HasPrefix(args[argIndex], versionShortFlag) {
			if !cmd.supportsVersionFlag {
				err = fmt.Errorf("%s not supported by this command", versionFlag)
				return
			}
			// Check if its an inline version
			if matches := inlineVersionRegex.FindStringSubmatch(args[argIndex]); matches != nil {
				c.version, err = parseVersion(matches[2])
				c.hasVersion = true
				if err != nil {
					return
				}
				argIndex += 1
				continue
			}
			// Check if its an inline-v (-v1, etc)
			if matches := inlineVRegex.FindStringSubmatch(args[argIndex]); matches != nil {
				c.version, err = parseVersion(matches[2])
				c.hasVersion = true
				if err != nil {
					return
				}
				argIndex += 1
				continue
			}

			// Else, we must read the following argument
			if argIndex == len(args)-1 {
				err = fmt.Errorf("version not provided")
				return
			}
			c.version, err = parseVersion(args[argIndex+1])
			c.hasVersion = true
			if err != nil {
				return
			}
			argIndex += 2
			continue
		}
		if strings.HasPrefix(args[argIndex], "-") {
			err = fmt.Errorf("%s is not a supported flag", args[argIndex])
			return
		}
		cmdLineArgsWithoutFlags = append(cmdLineArgsWithoutFlags, args[argIndex])
		argIndex++
		i++
	}
	return
}

// args should be free of flags
func parseArgs(c command, cArgs *commandArgs, args []string) (err error) {
	if c.firstArgMightBeNumber {
		if len(args) > 0 {
			num, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("%q is not a number", args[0])
			}
			if num < 0 || num > maxNumberAllowedInCommandArgs {
				err = fmt.Errorf("number must be > 0 and < %d",
					maxNumberAllowedInCommandArgs)
				return err
			}
			cArgs.number = num
		} else {
			cArgs.number = -1
		}
	} else {
		cArgs.number = -1
	}
	if c.firstArgMightBeMessage {
		if len(args) > 0 {
			cArgs.message = args[0]
		}
	}
	if c.firstArgIsMessage {
		if len(args) == 0 {
			err = errors.New(messageNotProvided)
			return
		}
		cArgs.message = args[0]
	}
	if c.firstArgIsCommit {
		if len(args) == 0 {
			err = errors.New(targerCommitNotProvided)
			return
		}
		cArgs.commit0 = args[0]
	}
	if c.firstArgMightBeServerCommit {
		if len(args) > 0 {
			cArgs.commit0InServerSyntax = args[0]
		}
	}
	if c.firstArgIsCommitOrCurrent {
		if len(args) > 0 {
			cArgs.commit0 = args[0]
		}
	}
	if c.firstArgMightBeCommit {
		if len(args) > 0 {
			cArgs.commit0 = args[0]
		}
	}
	if c.secondArgMightBeCommit {
		if len(args) > 1 {
			cArgs.commit1 = args[1]
		}
	}
	if c.firstArgIsFile {
		if len(args) == 0 {
			err = errors.New(fileNotProvided)
			return
		}
		cArgs.filename = args[0]
	}
	if c.firstArgIsServerPath {
		if len(args) == 0 {
			err = errors.New(serverUrlNotProvided)
			return
		}
		cArgs.serverPath = "/" + args[0]
	}
	if c.firstArgIsUnsafeURL {
		if len(args) == 0 {
			err = errors.New(serverUrlNotProvided)
			return
		}
		cArgs.unsafeServerUrl = args[0]
	}
	if c.secondArgMightBeApiKey {
		if len(args) >= 2 {
			cArgs.apiKey = args[1]
		}
	}
	if c.firstArgIsApiKey {
		if len(args) == 0 {
			err = errors.New(apiKeyNotProvided)
			return
		}
		cArgs.apiKey = args[0]
	}
	if c.firstArgIsBool {
		if len(args) == 0 {
			err = errors.New(boolNotProvided)
			return
		}
		cArgs.boolInFirstArg, err = strconv.ParseBool(args[0])
		if err != nil {
			err = errors.New(invalidBoolValue)
		}
	}
	if c.firstArgIsServerId {
		if len(args) == 0 {
			err = errors.New(serverIdNotProvided)
			return
		}
		cArgs.serverId, err = strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			err = errors.New(invalidServerId)
			return
		}
	}
	return
}

// Receives a csv filter of file names (e.g. "a/b.txt, c.txt") and returns
// a list of file names
func parseFilter(filter string) ([]string, error) {
	const maxFilterSize = 100
	// Trim leading/trailing spaces and quotes
	filter = strings.TrimSpace(filter)
	filter = strings.Trim(filter, `"'`) // remove " or ' if present
	if filter == "" {
		return nil, fmt.Errorf("`%s` is not a valid filter", filter)
	}
	parts := strings.Split(filter, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
		if len(result) >= maxFilterSize {
			return nil, fmt.Errorf(
				"`%s` is too long. you can filter at most %d files",
				filter, maxFilterSize)
		}
	}
	return result, nil
}
func parseVersion(v string) (uint64, error) {
	V, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s is not a valid version", v)
	}
	return V, nil
}

// Ask the user to input their api key and put it in the args.
// Logs err and returns false in any error
func (a app) askApiKey(args *commandArgs) bool {
	const pathToViewApiKey = "/user-settings"
	var hyperlink string
	if a.supportsHyperlinks {
		hyperlink = links.GetHyperlink(
			pathToViewApiKey,
			a.twiggWebUrl+"/"+pathToViewApiKey,
		)
	} else {
		hyperlink = pathToViewApiKey
	}
	a.logInfo(
		fmt.Sprintf("What's your CLI key? (get it under %s)",
			hyperlink,
		))
	_, err := fmt.Fscan(a.in, &args.apiKey)
	if err != nil {
		if err == io.EOF {
			a.logError(apiKeyNotProvided)
			return false
		}
		a.logError(fmt.Sprintf("failed to read api key: %s", err))
	}
	// Add a len check just in case
	if len(args.apiKey) > 300 {
		a.logError("invalid api key")
		return false
	}
	return err == nil
}

func (a app) askConfirmation(prompt string) bool {
	a.logInfo(prompt)
	var answer string
	_, err := fmt.Fscan(a.in, &answer)
	if err != nil {
		return false
	}
	answer = strings.ToLower(answer)
	return answer == "y" || answer == "yes"
}

// just a checker to panic if we configure a command badly
func checkCommandOrDie(c command) {
	nArgOneMustBe := 0
	if c.firstArgMightBeNumber {
		nArgOneMustBe += 1
	}
	if c.firstArgMightBeMessage {
		nArgOneMustBe += 1
	}
	if c.firstArgIsMessage {
		nArgOneMustBe += 1
	}
	if c.firstArgIsCommit {
		nArgOneMustBe += 1
	}
	if c.firstArgIsFile {
		nArgOneMustBe += 1
	}
	if c.firstArgIsServerPath {
		nArgOneMustBe += 1
	}
	if c.firstArgIsUnsafeURL {
		nArgOneMustBe += 1
	}
	if c.firstArgIsApiKey {
		nArgOneMustBe += 1
	}
	if c.firstArgIsBool {
		nArgOneMustBe += 1
	}
	if c.firstArgIsServerId {
		nArgOneMustBe += 1
	}
	if c.firstArgIsCommitOrCurrent {
		nArgOneMustBe += 1
	}
	if nArgOneMustBe > 1 {
		panic("too many requiremets for first arg")
	}

	if !isParentCommitAlias("parent") {
		panic("`parent` is used in the vscode extension, so remember to change it there as well")
	}
}