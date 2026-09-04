package server

import (
	"context"
	"errors"
	"io"
	"monorepo/base/iterator"
	"monorepo/twigg/commit"
	"monorepo/twigg/repo"
	"monorepo/twigg/tree"
	"monorepo/twigg/workdir"
	"net/http"
	"testing"
)

type Read interface {
	repo.Read
	commit.Read
	GetRepoNextLocalId(repoId uint64) (n uint64, isErrNotFound bool, err error)
	GetRepoTopCommit(repoId uint64) (commitLocalId uint64, isErrNotFound bool, err error)
	GetCommitChildren(repoId uint64, commitL uint64, commitV uint64,
		maxRowsToRead int) (
		children []commit.LocalId, childrenVersions []uint64, err error)
}

type Write interface {
	Read
	repo.Write
	commit.Write
	SetRepoNextLocalId(repoId uint64, n uint64) error
	SetRepoTopCommit(repoId uint64, commitLocalId uint64) (err error)
	PreventCommit()
}

type PushVerifier interface {
	// Only returns error if something unexpected happens and it can't determine
	// if the push is ok or not.
	PushIsOk(r *http.Request) (ok bool, notOkMsg string, err error)
}

type PullVerifier interface {
	// Only returns error if something unexpected happens and it can't determine
	// if the pull is ok or not.
	PullIsOk(r *http.Request) (ok bool, notOkMsg string, err error)
}

type PushObserver interface {
	// Called before commits are saved after they're pushed
	OnPush(newCommit *commit.Commit, hasOld bool, oldCommit commit.Commit)
}

// Server interface that runs on the server.
// All methods receive locks because the server which is embedding
// an instance (see TestServer for example) might want to perform
// other actions with the Lock before commiting.
type Server interface {
	// Returns wheather the server was initialized already or not.
	// Init must be called before all the other methods.
	WasInit() bool
	// Does all the necessary setup. Returns an error if already initialized.
	// It doesn't commit the lock (it's the caller's responsability).
	Init(l Write) error

	// Get commit.
	// Returns ErrNotFound if not found.
	GetLatest(n commit.LocalId, l Read) (commit.Commit, error)
	// Get a version of a commit.
	// Returns ErrNotFound if not found.
	GetVersion(n commit.LocalId, v uint64, l Read) (commit.Commit, error)

	// Panics if the server was not initialized.
	Pending(ascendingOrder bool, l Read) (iterator.I[commit.Commit], error)
	// Descending order. Panics if the server was not initialized.
	PendingAfter(afterId commit.LocalId, l Read) (iterator.I[commit.Commit], error)
	// Panics if the server was not initialized.
	// Returns the last subitted CL
	Top() commit.Commit

	// Handles a push from a client. Returns false and writes back errors to
	// the client if they happen (either due to bad requests or internal errors).
	// Use the PushObserver to modify the pushed commit (for example
	// assigning author) before saving it. Ok to pass nil as an arg.
	HandlePush(w http.ResponseWriter, r *http.Request, v PushVerifier,
		p PushObserver, l Write) (ok bool)
	// Handles a pull from a client. Returns false and writes back errors to
	// the client if they happen (either due to bad requests or internal errors).
	HandlePull(w http.ResponseWriter, r *http.Request, v PullVerifier, l Read) (ok bool)

	// Submits a Commit.
	// Returns ErrSubmitConflict if the submit would cause a rebase conflict.
	// Returns ErrParentNotSubmitted if the commit can't be submitted bc
	// the parent commit was not submitted yet.
	Submit(c commit.Commit, l Write) error

	// Creates a commit that is a rollback of c.
	CreateRollback(c commit.LocalId, authorUserId int64, l Write) (commit.Commit, error)

	// Creates a new version of c with a different message (message-only amend).
	// Returns an error if c is submitted or the message is invalid.
	RenameCommit(cId commit.LocalId, newMessage string, authorUserId int64, l Write) (commit.Commit, error)

	// Returns whether a commit can be submitted or not.
	// If not, returns `false`, an enum indicating the reason and `nil`.
	// Only returns errors if it fails to determine if it can be submitted or not.
	// See the `Submit` function for the errors.
	CanSubmit(c commit.Commit, l Read) (bool, CantSubmitReason, error)

	// Compute diff between two commits.
	Diff(a, b commit.Commit, l Read) (it tree.ParallelIterator, err error)
	// Returns all files with a certain name that are located in directories
	// whose contents changed
	SearchFileInChangedDirs(A, B commit.Commit, l Read, filename string) (repo.FileInChangedDirsIter, error)
	// Write the textual diff between two commits
	WriteDiff(a, b commit.Commit, filename string, w io.Writer, l Read) error
	// Write the file of a commit
	WriteFile(a commit.Commit, filename string, w io.Writer, l Read) error
	// Returns a tree of the specified path in the specified commit
	GetTree(a commit.Commit, path string, l Read) (tree.Tree, error)
	// Returns the size of the root directory on the specified commit
	GetCommitWorkdirSize(c commit.Commit, l Read) (int64, error)

	// Materializes the commit to the workdir
	Load(c commit.LocalId, wd workdir.Workdir, l Read) error

	// Sets the id that the next commit created on the server will get.
	// The new id must be greater than the current next id and at most
	// 10_000 ids ahead of it. Doesn't commit the lock.
	SetNextServerId(id uint64, l Write) error
}

// Creates a server instance.
// It needs a ReadLock to read the server state.
// Using it requires Init() to be called, but that's not done by default
// because inniting writes to a WriteLock.
func NewServer(quotaOwner string, repoId uint64, l Read) (Server, error) {
	return newSrv(quotaOwner, repoId, l)
}

// Wrapper around a server made for testing that creates/commits locks.
// All errors lead to t.Fatal().
type TestServer interface {
	// Returns the underlying server instance
	GetServer() Server
	// Returns a write lock from the internal db instance
	BeginWrite() (w context.Context, closeW func(), commitW func() error)
	// Adapts a write lock to the expected write interface
	BindW(w context.Context) Write
	// Adapts a read lock to the expected read interface
	BindR(r context.Context) Read

	// Root server
	RootUrl() string
	// Clients can send push/pull to RootUrl()/ServerPath()
	ServerPath() string
	// A path which will return a random text response, used to simulate
	// requests that are made not to a twigg server
	RandomResponsePath() string
	// Last submitted Commit
	Top() commit.Commit
	// Returns an iterator to read the pending commits
	Pending() (it iterator.I[commit.Commit], closeIt func())
	// Descending order.
	PendingAfter(afterId commit.LocalId) (it iterator.I[commit.Commit], closeIt func())
	// Returns true if the commit is pending
	IsPending(n commit.LocalId) bool
	// Returns the number of pending commits
	NPending() int
	// Submits a commit
	Submit(c commit.LocalId)
	// Indicates if a commit can be submitted or not
	CanSubmit(c commit.LocalId) bool
	// Create a rollback commit
	CreateRollback(c commit.LocalId, authorUserId int64) commit.Commit
	// Creates a new version of c with a different message (message-only amend).
	RenameCommit(cId commit.LocalId, newMessage string, authorUserId int64) commit.Commit
	// Get a commit by the server CL number
	GetLatest(c commit.LocalId) commit.Commit
	// Get a commit by the server CL number and its version
	GetVersion(c commit.LocalId, v uint64) commit.Commit
	// Verifies a commit is not found
	CheckNotFound(cId commit.LocalId)
	// Verifies a commit version is not found
	CheckVersionNotFound(cId commit.LocalId, version uint64)
	// Writes the contents of a file to a Writer
	WriteFile(c commit.Commit, filename string, w io.Writer)
	// Returns the size of the root directory on the specified commit
	GetCommitWorkdirSize(c commit.Commit) int64
	// Diff two commits for testing
	Diff(a, b commit.Commit) []tree.Diff
	// Simulate the server shutting down and restarting
	Restart()
	// Used to set a custom PushObserver
	SetPushObserver(p PushObserver)
	// Loads the commit to the provided empty workdir
	Load(c commit.LocalId, wd workdir.Workdir)
	// Verifies that Load returns an error
	CheckLoadErrors(c commit.LocalId, wd workdir.Workdir)
}

// Starts and returns a test server that auto cleans up and that is acessible
// for post/push requests of Ag clients.
// The server will only accept requests that have the provided api key.
func NewTestServer(apiKey string, t *testing.T) TestServer {
	return newTestServer(apiKey, t)
}

type CantSubmitReason uint8

const (
	CantSubmitReasonNone CantSubmitReason = iota
	CantSubmitWithConflict
	CantSubmitAlreadySubmitted
	CantSubmitWouldCauseRebaseConflict
	CantSubmitBeforeParent
	CantSubmitObsoleteCommit
)

var (
	ErrNotFound = errors.New("not found")
)