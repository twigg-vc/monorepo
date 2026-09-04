package cli

import (
	"io"
	"testing"
)

// Parse command line args and execute.
// If out==nil, uses Stdout. If in==nil, uses Stdin
// systemRootPath, if provided, is the "Top-most" directory of the system.
// The CLI won't go above that path.
// twiggWebUrl specifies the root url of the server to which the CLI
// will communicate (e.g. `https://twigg.vc`)
// Returns the exit status code (0 = ok, anything else is an error)
func Run(systemRootPath *string, twiggWebUrl string, out io.Writer, in io.Reader) int {
	return run(systemRootPath, twiggWebUrl, out, in)
}

// This interface helps the test become less verbose and easier to follow.
// All the "Check" methods call t.Fatal if they don't succeed.
type TestHelper interface {
	// Sets the root url of the server. This must be done before running
	// commands that communicate with a server (pull/push).
	SetServerRootUrl(rootUrl string)
	// Runs the cli for the specified args
	Run(args ...string)
	// Returns the output
	Out() string
	// Mocks the user will type input to the terminal
	PrepareInput(input string)
	// Changes to a child directory
	Cd(path string)
	// Write a file to the workdir with the provided content
	WriteFile(filename string, content string)
	// Writes an executable file
	WriteExecutable(filename string, content string)
	// Create a symlink file
	WriteSymlink(filename, target string)
	// Create a symlink to an arbitrary target path
	WriteSymlinkWithAbsPath(filename, absTargetPath string)
	// Delete a folder from the workdir
	DeleteFolder(name string)
	// Delete a file from the workdir
	DeleteFile(filename string)
	// Checks that the output of the latest run contains the substring
	CheckOutContains(substring string)
	// Checks that the output of the latest run contains the substring nTimes
	CheckOutContainsN(substring string, nTimes int)
	// Checks that the output of the latest run doesn't contain the substring
	CheckOutDoesntContain(substring string)
	// Checks that the output of the latest run is empty
	CheckOutIsEmpty()
	// Checks the output has the expected []JsonDiffFile
	CheckOutJsonDiffFile(expected []JsonDiffFile)
	// Check that the workdir contains the file with the provided content
	CheckFile(filename string, expected string)
	// Check that the workdir contains the file with an provided line
	CheckFileLine(filename string, line int, expected string)
	// Check that the workdir contains the symlink `filename`
	// pointing to `expectedTarget`
	CheckSymlink(filename string, expectedTarget string)
	// Check that the workdir doesn't have a file
	CheckHasNoFile(filename string)
	// Check that the workdir has a directory
	CheckDirectoryExists(relativePath string)
	// Check that the workdir doesnt have a directory
	CheckDirectoryDoesntExist(relativePath string)
	// Calls `log` command and reads the active commit.
	// Fails the test if it can't be read.
	ActiveCommit() LoggedCommit
	// Helper to check active commit.
	CheckActiveCommit(args CheckCommitArg)
	// Helper to check the local id of the active commit
	CheckActiveCommitLocalId(i int)
	// Calls `log` command and reads the commits logged
	Log() []LoggedCommit
	// Calls `log` with "--all" flag and reads the commits logged
	LogAll() []LoggedCommit
	// Similar to Log(), but filters out the obsolete commits
	NonObsoletLog() []LoggedCommit
	// Checks that mentioned commits appear in the log.
	// The order is not considered.
	CheckLog(expectedIds ...int)
	// Same as CheckLog, but using --all. Note that you need to provide
	// the repeated values. If you expect commit1-v0 and commit1-v1; use [1, 1].
	CheckLogAll(expectedIds ...int)
	// Similar to CheckLogAll, but it'll check the versions.
	// Order doesn't matter.
	CheckLogAllVersions(expectedVersions ...IdVersionAndConflict)
	// Same as CheckLog, but using "number" as argument. The order is not
	// considered
	CheckLogN(number int, expectedIds []int)
}

type CheckCommitArg struct {
	Id             int
	Version        int
	HasConflicts   bool
	IsSubmitted    bool
	HasServerId    bool
	ServerId       int
	HasServerV     bool
	ServerV        int
	ObsoleteReason string // amend/pull/manual-rebase/auto-rebase/submit/restore
}

type LoggedCommit struct {
	Id             int
	IdString       string
	Version        int
	HasServerId    bool
	ServerId       int
	HasServerV     bool
	ServerVersion  int
	IsSubmitted    bool
	IsObsolete     bool
	HasConflicts   bool
	IsActive       bool
	IsUploaded     bool
	IsNotUploaded  bool
	ObsoleteReason string // amend/pull/manual-rebase/auto-rebase/submit/restore
}

type IdVersionAndConflict struct {
	Id           int
	Version      int
	HasConflicts bool
}

// Returns a TestHelper.
func NewTestHelper(t testing.TB) TestHelper {
	return newTestHelperAt("test-client-1", true, t)
}

// Returns a TestHelper inside a specific folder
func NewTestHelperAt(dir string, t testing.TB) TestHelper {
	return newTestHelperAt(dir, true, t)
}

// Returns a TestHelper which is different instance of that returned
// by NewTestHelper.
// This is equivalent to having another client and is usefull to test
// push/pull of different clients to the server.
func NewTestHelper2(t *testing.T) TestHelper {
	return newTestHelperAt("test-client-2", true, t)
}

// Returns a TestHelper that doesnt cleanup the files
func NewTestHelperNoCleanup(t testing.TB) TestHelper {
	return newTestHelperAt("test-client-1", false, t)
}
