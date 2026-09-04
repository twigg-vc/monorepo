package commit

import (
	"fmt"
	"io"
	"monorepo/base/iterator"
	"monorepo/twigg/tree"
	"time"
)

type Commit struct {
	// Indicates how a commit "came to be"
	Birth BirthReason
	// Local identifier that is specific to a local instance
	L LocalId
	// Increased by 1 on rebases, amends, msg changes and submits.
	Version uint64
	// Indicates if ServerL is populated
	HasServerL bool
	// Local identifier on the server. This serves as a global identifier of the
	// all versions of a specific commit. Together with ServerV, it serves
	// as an unique global identifier of a commit (including its contents
	// and all the data).
	ServerL LocalId
	// Indicates this commit was pushed to the server
	HasServerV bool
	// Version on the server
	ServerV uint64
	// Identifies a version of the root tree
	TreeVersion uint64
	// Hash of the root tree
	RootDirHash [32]byte

	ParentL           LocalId
	ParentV           uint64
	HasParentServerL  bool
	ParentServerL     LocalId
	HasParentServerV  bool
	ParentServerV     uint64
	ParentTreeVersion uint64

	// "Pointers" to children
	Children         []LocalId
	ChildrenVersions []uint64
	// Indicates if this commit is the newest version or has been replaced by
	// a new one.
	Status Status
	// Indicates how this commit became obsolete
	ObsReason ObsoleteReason
	// Version that is the successor of this one when obsolete
	SuccessorVersion uint64
	// Is set to true when this commit is created/updated via a rebase that
	// resulted in conflicts. It's always false in all other cases.
	HasRebaseConflicts bool

	// Message used when it was created.
	// It can only change when the commit is ammended.
	Message string
	// Time of creation/last modification.
	// It changes when the commit is ammended/rebased.
	CreatedOn time.Time

	// Indicates the server submitted this commit.
	IsSubmitted bool
	// Indicates Commit is hidden.
	IsHidden bool

	// Used to indicate which version a commit is a restore of
	IsRestoreOf uint64

	// Build-time variable that indicates the version of the
	// client code that created this commit.
	ClientBuildVersion string
	// Build-time variable that indicates the version of the
	// server code that created this commit.
	ServerBuildVersion string
	// Id of the user who created the commit. This optionally used by servers to
	// identify authors.
	AuthorUserId int64

	// If set, indicates this commit is not attached to its parents in the
	// client it is in. This is used to allow commits to be exchanged even
	// without the parent being fully known.
	IsDetached bool

	// If this is a rollback commit, indicates the id of the commit
	IsRollbackOfL LocalId
	// If this is a rollback commit, indicates the version of the commit
	IsRollbackOfV uint64
	// Indicates that the first version of this commit was a rollback
	FirstVersionIsRollback bool

	// If true, the DiffData fields are populated
	HasDiffData bool
	// If HasDiffData, contains the total num of lines created wrt the parent
	DiffDataLinesCreated int64
	// If HasDiffData, contains the total num of lines deleted wrt the parent
	DiffDataLinesDeleted int64
	// If HasDiffData, contains the total num of lines modified wrt the parent
	DiffDataLinesModified int64
	// If HasDiffData, contains the total num of files created wrt the parent
	DiffDataFilesCreated int64
	// If HasDiffData, contains the total num of files deleted wrt the parent
	DiffDataFilesDeleted int64
	// If HasDiffData, contains the total num of files modified wrt the parent
	DiffDataFilesModified int64
}

const RootCommitId = 0

func NewRoot(isOnServer bool, emptyHash [32]byte) Commit {
	return newCommit(
		BirthReasonCommit,
		/*L*/ RootCommitId,
		/*Version*/ 0,
		/*HasServerL*/ true,
		/*ServerL*/ RootCommitId,
		/*HasServerV*/ true,
		/*ServerV*/ 0,
		/*TreeVersion*/ 0,
		/*RootHash*/ emptyHash,
		/*ParentL*/ 0,
		/*ParentV*/ 0,
		/*HasParentServerL*/ true,
		/*ParentServerL*/ 0,
		/*HasParentServerV*/ true,
		/*ParentServerV*/ 0,
		/*ParentTreeVersion*/ 0,
		[]LocalId{},
		[]uint64{},
		StatusLatest,
		/*SucessorVersion*/ 0,
		ObsoleteReasonNone,
		/*HasRebaseConflicts=*/ false,
		time.UnixMicro(0),
		/*message=*/ "",
		/*IsSubmitted=*/ true,
		/*IsHidden=*/ false,
		/*IsRestoreVersionOf*/ 0,
		isOnServer,
		/*AuthorUserId*/ 0,
		/*IsDetached*/ false,
		/*IsRollbackOf*/ 0,
		/*IsRollbackOfV*/ 0,
		/*FirstVersionIsRollback*/ false,
		/*HasDiffData*/ true,
		/*DiffDataLinesCreated*/ 0,
		/*DiffDataLinesDeleted*/ 0,
		/*DiffDataLinesModified*/ 0,
		/*DiffDataFilesCreated*/ 0,
		/*DiffDataFilesDeleted*/ 0,
		/*DiffDataFilesModified*/ 0,
	)
}

// Create a new commit that is just created from the workdir.
// Automatically adds it as a child to the parent.
func NewOriginal(isOnServer bool, nextLocalId LocalId, treeVersion uint64,
	rootDirHash [32]byte, message string, parent *Commit,
	diffCounts tree.TotalDiffCounts) Commit {
	return newOriginal(isOnServer, nextLocalId, treeVersion, rootDirHash, message, parent,
		diffCounts)
}

// Create a new commit that is an amend of the old one.
// Automatically adds it as a child to the old parent and sets it as successor.
func NewAmend(treeVersion uint64,
	rootDirHash [32]byte, message string, old, oldParent *Commit, isOnServer bool, authorUserId *int64,
	diffCounts tree.TotalDiffCounts) Commit {
	return newAmend(isOnServer, treeVersion,
		rootDirHash, message, old, oldParent, authorUserId,
		diffCounts)
}

// Create a new commit that is a rebase of old into newParent.
// Automatically marks the successor of old and attaches it to the new parent.
func NewRebase(isOnServer bool, isAutoRebaseOfChildren bool,
	treeVersion uint64,
	rootDirHash [32]byte, hasRebaseConflicts bool, old, newParent *Commit,
	diffCounts tree.TotalDiffCounts) Commit {
	return newRebase(isOnServer, isAutoRebaseOfChildren, treeVersion,
		rootDirHash, hasRebaseConflicts, old, newParent,
		diffCounts)
}

// Returns true if the incoming commit is "the same" (same...ish) as the
// local one.
// I.e. they are different only due to "non-content-altering" operations.
// Content-altering operations are: creating a new commit, amending it,
// rebasing it or submitting it.
func IsSameish(incoming, local Commit) bool {
	return isSameish(incoming, local)
}

// Create a new local commit from one that was pushed by a client or is pulled
// from the server.
// Automatically marks the local one as obsolete and attaches the
// pulled to the local parent (if provided).
// Note that this function "trusts" the diff data from the incoming commit;
// which means an attacked could spoof these values. This is an accepted limitation
// because that would not cause any serious security vuln; the only effect would be
// that someone could fake their commit's diff stats if they want to.
// If we ever want to fix this we def can; but the server would need to
// count the diffs of each pushed commit. This is not hugely costly but its also
// not trivially cheap.
func NewLocal(isOnServer bool,
	incoming Commit, treeVersion uint64, treeHash [32]byte,
	hasLocalVersion bool, latestLocalVersion *Commit, localParent *Commit,
	nextLocalId LocalId) (local Commit, usedNextLocalId bool) {
	return newLocal(isOnServer, incoming, treeVersion, treeHash, hasLocalVersion,
		latestLocalVersion, localParent, nextLocalId)
}

// Create a commit that is a trivial submit of an old one.
// Automatically marks the old one as obsolete and attaches the new one
// to the parent.
func NewTrivialSubmit(old, oldParent *Commit) Commit {
	return newTrivialSubmit(old, oldParent)
}

// Create a new commit that is the submit made by a rebase (non-trivial).
// Auto attach it to the parent and mark the old one as obsolete.
func NewRebaseSubmit(treeVersion uint64,
	rootDirHash [32]byte, old, newParent *Commit,
	diffCounts tree.TotalDiffCounts) Commit {
	return newRebaseSubmit(treeVersion,
		rootDirHash, old, newParent, diffCounts)
}

// Use to create a new commit that is a restore of an old one.
// Automatically sets latest as obsolete and attaches the new one
// to the oldParent.
func NewRestoreCommit(isOnServer bool, latest *Commit, old Commit, oldParent *Commit) Commit {
	return newRestoreCommit(isOnServer, latest, old, oldParent)
}

// Create a new commit that is the rollback of an old one.
// Automatically adds it as a child to the parent if attachToParent is used.
func NewRollback(old Commit, hasRebaseConflicts bool, isOnServer bool, nextLocalId LocalId, treeVersion uint64,
	rootDirHash [32]byte, message string, attachToParent bool, parent *Commit,
	diffCounts tree.TotalDiffCounts) Commit {
	return newRollback(old, hasRebaseConflicts, isOnServer, nextLocalId, treeVersion,
		rootDirHash, message, attachToParent, parent,
		diffCounts)
}

func (c Commit) IsDetachedOrRoot() bool {
	return c.IsDetached || c.HasServerL && c.ServerL == 0
}
func (c Commit) IsOnServer() bool {
	if c.HasServerV && !c.HasServerL {
		panic("has serverV but not serverL")
	}
	return c.HasServerL && c.HasServerV
}
func (c Commit) ParentIsOnServer() bool {
	if c.HasParentServerV && !c.HasParentServerL {
		panic("has parentServerV but not parentServerL")
	}
	return c.HasParentServerL && c.HasParentServerV
}

// Sets the ParentServer* attributes using the parent.
// Returns true if something was set
// Panics if the parent doesn't yet have the server ids.
func (c *Commit) SetParentServerData(parent Commit) bool {
	return c.setParentServerLV(parent)
}

// Attaches a detached commit to the local copy of its parent, also adding it
// as a child of the parent.
// Panics if c is not detached or if parent is not the parent of c, or if the
// parent is not on the server.
func (c *Commit) AttachToLocalParent(parent *Commit) {
	attachToLocalParent(c, parent)
}

// Simply appends a child if not there yet
func (c *Commit) AddChildIfNotPresent(child Commit) (added bool) {
	return addChildIfNotPresent(c, child)
}

// Simple function to check if commit has a child
func (c Commit) HasChild(child Commit) bool {
	return hasChild(c, child)
}

// Sets the necessary fields to mark this as obsolete
func (c *Commit) SetSucessor(successorVersion uint64, reason ObsoleteReason) {
	if c.Status != StatusLatest {
		panic(fmt.Sprintf("#%d was already obsolete", c.L))
	}
	if c.IsSubmitted {
		panic(fmt.Sprintf("#%d is submitted and can't become obsolete", c.L))
	}
	c.Status = StatusObsolete
	c.SuccessorVersion = successorVersion
	c.ObsReason = reason
}

type Read interface {
	GetLatestCommitByLocalId(repoId uint64, n uint64) (c Commit, isNotFoundErr bool, err error)
	GetCommitVersionByLocalId(repoId uint64, L uint64, v uint64) (c Commit, isNotFoundErr bool, err error)
	GetLatestCommitByServerId(repoId uint64, ServerL uint64) (c Commit, isNotFoundErr bool, err error)
	GetCommitVersionByServerId(repoId uint64, ServerL uint64, ServerV uint64) (c Commit, isNotFoundErr bool, err error)
	GetPendingCommits(ascendingOrder bool, repoId uint64) (iterator.I[Commit], error)
	GetPendingCommitsAfter(repoId uint64, afterId LocalId) (iterator.I[Commit], error)
}

type Write interface {
	Read
	// Read "c.IsSubmitted" to determine if commit is submitted or not.
	// Submitted commits can't be altered.
	SetCommit(quotaOwner string, repoId uint64, c Commit) error
}

// Writes binary representation to a writer
func (c Commit) WriteDataTo(w io.Writer) error {
	return c.writeTo(w)
}

// Populates this entity reading binary representation from a reader
func (c *Commit) ReadDataFrom(r io.Reader) error {
	return c.readFrom(r)
}

func (c Commit) Bytes() []byte {
	return c.bytes()
}

func (c *Commit) PopulateFromReader(r io.Reader) error {
	return c.populateFromReader(r)
}

type LocalId = uint64

type Status uint8

const (
	// Latest version, i.e. non-obsolete
	StatusLatest Status = iota
	// Commit of version v has been replaced with commit of version v+1
	StatusObsolete
)

// Indicates how a commit was born
type BirthReason uint8

const (
	// Created by a commit command
	BirthReasonCommit BirthReason = iota
	// Created by an amend
	BirthReasonAmend
	// Created by a manual (i.e. done by an user) rebase
	BirthReasonManualRebase
	// Created by an automatic (i.e. not done by an user) rebase of children
	// of a commit (for example after an amend)
	BirthReasonAutoRebaseOfChildren
	// Created by a submit
	BirthReasonSubmit
	// Restore of an old commit
	BirthReasonRestore
	// Rollback of a commit
	BirthReasonRollback
)

// Indicates why a commit became obsolete
type ObsoleteReason uint8

const (
	// This commit is not obsolete
	ObsoleteReasonNone ObsoleteReason = iota
	// Ammended into a new version
	ObsoleteReasonAmend
	// Overwritten by a pushed commit
	ObsoleteReasonPushOverwrite
	// Overwritten by a pulled commit
	ObsoleteReasonPullOverwrite
	// Manually (i.e. by a user) rebased
	ObsoleteReasonManualRebase
	// Automatically (i.e. not by a user) rebased
	ObsoleteReasonAutoRebaseOfChildren
	// Submitted by a rebase
	ObsoleteReasonSubmit
	// Commit was made restored into an old version
	ObsoleteReasonRestored
)