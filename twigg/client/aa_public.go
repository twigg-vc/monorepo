package client

import (
	"errors"
	"io"
	"monorepo/twigg/commit"
	"monorepo/twigg/repo"
	"monorepo/twigg/tree"
	"monorepo/twigg/workdir"
	"testing"
)

type Read interface {
	repo.Read
	commit.Read
	GetRepoNextLocalId(repoId uint64) (n uint64, isErrNotFound bool, err error)
}

type Write interface {
	Read
	repo.Write
	commit.Write
	SetRepoNextLocalId(repoId uint64, n uint64) error
}

// Twigg client.
// It only provides interfaces which receive Locks because this code
// should be embedded into a client (CLI or a server) which may
// want to read/write more data to the lock to decide if they want to commit
// it or not. No methods commit the write locks.
type Client interface {
	// Returns true if already initialized.
	IsInit() bool
	// Create the root commit. Doesn't commit the lock (caller must commit).
	Init(l Write) (commit.Commit, error)
	// Get commit by the CL number.
	// Returns ErrCommitNotFound if not found.
	GetLatest(n commit.LocalId, l Read) (commit.Commit, error)
	// Get commit by CL and version
	// Returns ErrCommitNotFound if not found.
	GetVersion(n commit.LocalId, v uint64, l Read) (commit.Commit, error)
	// Returns the root of the provided commit
	GetRoot(c commit.Commit, l Read) tree.Root
	// Returns a commit by its server (global) id
	GetLatestByServerId(serverId commit.LocalId, l Read) (commit.Commit, error)
	// Returns a commit by its server id and version
	GetVersionByServerId(serverId commit.LocalId, serverV uint64, l Read) (commit.Commit, error)

	// Create a commit given a workdir, a message, and its parent.
	// If there's nothing to commit, returns ErrNothingToCommit.
	// Doesn't commit the lock (caller must commit).
	Commit(wd tree.Root, message string, parent *commit.Commit, l Write) (commit.Commit, error)
	// Amend a previous commit by reading a workdir and a message.
	// Returns ErrNothingToCommit if the workdir and the message didn't change.
	// Doesn't commit the lock (caller must commit).
	Amend(c *commit.Commit, rebaseChildren bool, wd tree.Root, message string, l Write) (commit.Commit, error)
	// Create a new comit that is a rebase of a into b. Must be called with the
	// latest versions of both commits.
	// Also marks A as obsolete.
	// Doesn't commit the lock (caller must commit).
	Rebase(A, B *commit.Commit, isAutoRebaseOfChildren bool, rebaseChildren bool, l Write) (commit.Commit, error)
	// Rebases the children of an old version of a commit into the new version.
	RebaseChildren(oldCommit commit.Commit, newParent *commit.Commit, l Write) error
	// Create a new commit that is the "restauration" of the old one.
	Restore(newCommit *commit.Commit, oldCommit commit.Commit, l Write) (commit.Commit, error)

	// Load a commit into a workdir by writing/deleting files from it
	Load(m repo.TreeVersion, wd workdir.Workdir, l Read) error
	// Hide commit
	Hide(c *commit.Commit, lock Write) error
	// Unhide commit
	Unhide(c *commit.Commit, lock Write) error
	// Returns Diff betwen two commits
	// NoChange diffs are filtered out.
	Diff(a, b repo.TreeVersion, l Read) (it tree.ParallelIterator, err error)
	// Returns all files with the given name that are located
	// in directories whose contents changed
	SearchFileInChangedDirs(A, B commit.Commit, l Read, filename string) (repo.FileInChangedDirsIter, error)
	// Writes the (a-b) textual diff of filename to the Writer .
	// If the file is too big or not textual, a fake diff is written.
	WriteDiff(a, b repo.TreeVersion, filename string, w io.Writer, l Read) error
	// Writes the (a-b) textual diff of all changed files
	WriteDiffAll(a, b repo.TreeVersion, w io.Writer, l Read) error
	// Returns the Diff between the workdir and the specified commit.
	// NoChange diffs are filtered out.
	DiffWorkdir(w workdir.Workdir, a repo.TreeVersion, l Read) (it tree.ParallelIterator, err error)

	// Writes the content of a file to a Writer.
	// Returns ErrFileNotFound if not found.
	Read(c repo.TreeVersion, filename string, w io.Writer, l Read) error

	// Pushes all the non-pushed ancestors and then c.
	// Uses the parent of each commit as the base for pushing
	// Panics if commit is obsolete or has already been pushed.
	// isObsParentErr=true when the commit has an ancestor which is obsolete
	// and has never been pushed before. We return an error because the server
	// needs to know the parent commit. Since we don't push obsolete commits (
	// else we might override a new version with an old one), commits with
	// non-pushed obsolete parents must be rebased into parents which
	// have been pushed before or can be pushed not for not being obsolete.
	Push(c *commit.Commit, url string, apiKey string, l Write) (
		isObsParentErr bool, isBadApiKeyErr bool, err error)
	// Pulls all the submitted children after `base`(not including) from
	// another instance. base must be a commit that was pushed, else panics.
	// onPull is called before saving the pulled commit (it can be nil).
	// Note that `base` won't be modified by this function
	// (its passed as value, not ref); so you should read `base` again after this
	// function call if you plan on using it as it might have new children
	// added to it.
	// If it returns an error, pulling stops.
	// Doesn't commit the lock.
	// Returns ErrNothingToPull if already up to date.
	PullAllSubmittedAfter(base commit.Commit, url string, apiKey string,
		onPull func(
			pulledCommit commit.Commit,
			hasLocalCommit bool,
			localCommit commit.Commit) error,
		l Write) (isBadApiKeyErr bool, isOldProtocolErr bool, err error)

	// Pulls the specified commit.
	// if !hasServerVersion, the latest version is fetched.
	// `base` should be as "close" to the expected commit as possible, as the
	// server will only transfer the differences from the target commit to base.
	PullCommit(commitServerId uint64,
		hasServerVersion bool, commitServerVersion uint64,
		base commit.Commit,
		url string, apiKey string,
		onPull func(
			pulledCommit commit.Commit,
			hasLocalCommit bool,
			localCommit commit.Commit) error,
		l Write) (isBadApiKeyErr bool, isOldProtocolErr bool, err error)
	// Pulls the last submitted commit from the server.
	// `base` should be as "close" to the expected commit as possible, as the
	// server will only transfer the differences from the target commit to base.
	PullTopCommit(base commit.Commit, url string, apiKey string,
		onPull func(
			pulledCommit commit.Commit,
			hasLocalCommit bool,
			localCommit commit.Commit) error,
		l Write) (isBadApiKeyErr bool, isOldProtocolErr bool, err error)

	// Asks the server to set the id that the next commit created on it will
	// get. notOkMsg is set when the server refuses the request (e.g. bad id
	// or permission denied); err is set on unexpected failures.
	SetNextServerId(url string, apiKey string, id uint64) (
		notOkMsg string, err error)
}

var (
	ErrNothingToCommit     = errors.New("nothing to commit")
	ErrNothingToPull       = errors.New("nothing to pull")
	ErrCommitNotFound      = errors.New("commit not found")
	ErrFileNotFound        = errors.New("file not found")
	ErrNotTwiggServer      = errors.New("server is not speaking twigg")
	ErrFailedToReachServer = errors.New("failed to reach server")
)

func New(quotaOwner string, repoId uint64, l Read) (Client, error) {
	return newTw(quotaOwner, repoId, l)
}

func NewTest(quotaOwner string, repoId uint64, t *testing.T) (commit.Commit, Client, workdir.TestWorkdir, Write) {
	return newTestClient(quotaOwner, repoId, t)
}

// Max length for commit messages
const MaxMsgLen = 50

// Constants related to the server-client comunication
const (
	BaseCommitServerIdQueryParam          = "id"
	BaseCommitServerVersionQueryParam     = "v"
	IsDetachedPullQueryParamName          = "d"
	IsDetachedPullQueryParamValue         = "yes"
	DetachedCommitServerIdQueryParam      = "did"
	DetachedCommitServerVersionQueryParam = "dv"
	DetachedLastSubmittedCommitAlias      = "top"
	PushEndpoint                          = "/push"
	PushMethod                            = "POST"
	PullEndpoint                          = "/pull"
	PullMethod                            = "GET"
	SetServerIdEndpoint                   = "/set-server-id"
	SetServerIdMethod                     = "POST"
	SetServerIdQueryParam                 = "id"
)