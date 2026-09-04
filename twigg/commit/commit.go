package commit

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"monorepo/buildmeta"
	"monorepo/twigg/tree"

	"time"
)

func newCommit(
	Birth BirthReason,
	L LocalId,
	Version uint64,
	HasServerL bool,
	ServerL LocalId,
	HasServerV bool,
	ServerV uint64,
	T uint64,
	RootHash [32]byte,
	parentL LocalId,
	parentV uint64,
	hasParentServerL bool,
	ParentServerL LocalId,
	hasParentServerV bool,
	ParentServerV uint64,
	parentT uint64,
	Children []LocalId,
	ChildrenVersions []uint64,
	Status Status,
	SuccessorVersion uint64,
	Obs ObsoleteReason,
	HasRebaseConflicts bool,
	CreatedOn time.Time,
	Message string,
	IsSubmitted bool,
	IsHidden bool,
	IsRestoreOfVersion uint64,
	IsOnServer bool,
	AuthorUserId int64,
	IsDetached bool,
	IsRollbackOfId LocalId,
	IsRollbackOfV uint64,
	FirstVersionIsRollback bool,
	HasDiffData bool,
	DiffDataLinesCreated int64,
	DiffDataLinesDeleted int64,
	DiffDataLinesModified int64,
	DiffDataFilesCreated int64,
	DiffDataFilesDeleted int64,
	DiffDataFilesModified int64,
) Commit {
	c := Commit{
		Birth:                  Birth,
		L:                      L,
		Version:                Version,
		HasServerL:             HasServerL,
		ServerL:                ServerL,
		HasServerV:             HasServerV,
		ServerV:                ServerV,
		IsSubmitted:            IsSubmitted,
		TreeVersion:            T,
		RootDirHash:            RootHash,
		ParentL:                parentL,
		ParentV:                parentV,
		HasParentServerL:       hasParentServerL,
		ParentServerL:          ParentServerL,
		HasParentServerV:       hasParentServerV,
		ParentServerV:          ParentServerV,
		ParentTreeVersion:      parentT,
		Children:               Children,
		ChildrenVersions:       ChildrenVersions,
		Status:                 Status,
		SuccessorVersion:       SuccessorVersion,
		ObsReason:              Obs,
		HasRebaseConflicts:     HasRebaseConflicts,
		Message:                Message,
		CreatedOn:              CreatedOn,
		IsHidden:               IsHidden,
		IsRestoreOf:            IsRestoreOfVersion,
		AuthorUserId:           AuthorUserId,
		IsDetached:             IsDetached,
		IsRollbackOfL:          IsRollbackOfId,
		IsRollbackOfV:          IsRollbackOfV,
		FirstVersionIsRollback: FirstVersionIsRollback,
		HasDiffData:            HasDiffData,
		DiffDataLinesCreated:   DiffDataLinesCreated,
		DiffDataLinesDeleted:   DiffDataLinesDeleted,
		DiffDataLinesModified:  DiffDataLinesModified,
		DiffDataFilesCreated:   DiffDataFilesCreated,
		DiffDataFilesDeleted:   DiffDataFilesDeleted,
		DiffDataFilesModified:  DiffDataFilesModified,
	}
	if IsOnServer {
		c.ServerBuildVersion = buildmeta.Version
	} else {
		c.ClientBuildVersion = buildmeta.Version
	}
	return c
}

func newOriginal(isOnServer bool, nextLocalId LocalId, TreeVersion uint64,
	RootDirHash [32]byte, message string, parent *Commit,
	diffCounts tree.TotalDiffCounts) Commit {
	c := newCommit(
		BirthReasonCommit,
		nextLocalId,
		/*Version*/ 0,
		/*HasServerL*/ false,
		/*ServerL*/ 0,
		/*HasServerV*/ false,
		/*ServerV*/ 0,
		TreeVersion,
		RootDirHash,
		parent.L,
		parent.Version,
		parent.HasServerL,
		parent.ServerL,
		parent.HasServerV,
		parent.ServerV,
		parent.TreeVersion,
		/*Children*/ nil,
		/*ChildrenV*/ nil,
		StatusLatest,
		/*SuccessorV*/ 0,
		ObsoleteReasonNone,
		/*HasRebaseConflicts*/ false,
		time.Now(),
		message,
		/*IsSubmitted*/ false,
		/*IsHidden*/ false,
		/*IsRestoreOfVersion*/ 0,
		isOnServer,
		/*AuthorUserId*/ 0,
		/*IsDetached*/ false,
		/*IsRollbackOf*/ 0,
		/*IsRollbackOfV*/ 0,
		/*FirstVersionIsRollback*/ false,
		/*HasDiffData*/ true,
		/*DiffDataLinesCreated*/ diffCounts.LinesCreated,
		/*DiffDataLinesDeleted*/ diffCounts.LinesDeleted,
		/*DiffDataLinesModified*/ diffCounts.LinesModified,
		/*DiffDataFilesCreated*/ diffCounts.FilesCreated,
		/*DiffDataFilesDeleted*/ diffCounts.FilesDeleted,
		/*DiffDataFilesModified*/ diffCounts.FilesModified,
	)
	if !parent.AddChildIfNotPresent(c) {
		panic("child already exists")
	}
	return c
}

func newRollback(old Commit, hasRebaseConflicts bool, isOnServer bool, nextLocalId LocalId, treeVersion uint64,
	rootDirHash [32]byte, message string, attachToParent bool, parent *Commit,
	diffCounts tree.TotalDiffCounts) Commit {
	var ServerL LocalId
	if isOnServer {
		ServerL = nextLocalId
	}

	c := newCommit(
		BirthReasonRollback,
		nextLocalId,
		/*Version*/ 0,
		/*HasServerL*/ isOnServer,
		ServerL,
		/*HasServerV*/ isOnServer,
		/*ServerV*/ 0,
		treeVersion,
		rootDirHash,
		parent.L,
		parent.Version,
		parent.HasServerL,
		parent.ServerL,
		parent.HasServerV,
		parent.ServerV,
		parent.TreeVersion,
		/*Children*/ nil,
		/*ChildrenV*/ nil,
		StatusLatest,
		/*SuccessorV*/ 0,
		ObsoleteReasonNone,
		hasRebaseConflicts,
		time.Now(),
		message,
		/*IsSubmitted*/ false,
		/*IsHidden*/ false,
		/*IsRestoreOfVersion*/ 0,
		isOnServer,
		/*AuthorUserId*/ 0,
		/*IsDetached*/ false,
		/*IsRollbackOf*/ old.L,
		/*IsRollbackOfV*/ old.Version,
		/*FirstVersionIsRollback*/ true,
		/*HasDiffData*/ true,
		/*DiffDataLinesCreated*/ diffCounts.LinesCreated,
		/*DiffDataLinesDeleted*/ diffCounts.LinesDeleted,
		/*DiffDataLinesModified*/ diffCounts.LinesModified,
		/*DiffDataFilesCreated*/ diffCounts.FilesCreated,
		/*DiffDataFilesDeleted*/ diffCounts.FilesDeleted,
		/*DiffDataFilesModified*/ diffCounts.FilesModified,
	)
	if attachToParent {
		if !parent.AddChildIfNotPresent(c) {
			panic("child already exists")
		}
	}
	return c
}

func newAmend(isOnServer bool, treeVersion uint64,
	rootDirHash [32]byte, message string, old, oldParent *Commit, authorUserId *int64,
	diffCounts tree.TotalDiffCounts) Commit {
	if old.Status != StatusLatest {
		panic("tried to amend obsolete commit")
	}
	if old.IsSubmitted {
		panic("tried to amend submitted")
	}

	var aid int64
	if authorUserId != nil {
		aid = *authorUserId
	}

	var serverV uint64
	if isOnServer {
		serverV = old.ServerV + 1
	}

	ammended := newCommit(
		BirthReasonAmend,
		old.L,
		old.Version+1,
		old.HasServerL,
		old.ServerL,
		/*HasServerV*/ isOnServer,
		serverV,
		treeVersion,
		rootDirHash,
		old.ParentL,
		old.ParentV,
		old.HasParentServerL,
		old.ParentServerL,
		old.HasParentServerV,
		old.ParentServerV,
		old.ParentTreeVersion,
		/*Children*/ nil,
		/*ChildrenV*/ nil,
		StatusLatest,
		/*SuccessorV*/ 0,
		ObsoleteReasonNone,
		/*HasRebaseConflicts*/ false,
		time.Now(),
		message,
		/*IsSubmitted*/ false,
		/*IsHidden*/ false,
		/*IsRestoreOfVersion*/ 0,
		isOnServer,
		aid,
		old.IsDetached,
		/*IsRollbackOf*/ 0,
		/*IsRollbackOfV*/ 0,
		/*FirstVersionIsRollback*/ old.FirstVersionIsRollback,
		/*HasDiffData*/ true,
		/*DiffDataLinesCreated*/ diffCounts.LinesCreated,
		/*DiffDataLinesDeleted*/ diffCounts.LinesDeleted,
		/*DiffDataLinesModified*/ diffCounts.LinesModified,
		/*DiffDataFilesCreated*/ diffCounts.FilesCreated,
		/*DiffDataFilesDeleted*/ diffCounts.FilesDeleted,
		/*DiffDataFilesModified*/ diffCounts.FilesModified,
	)
	old.SetSucessor(ammended.Version, ObsoleteReasonAmend)
	if !old.IsDetached && !oldParent.AddChildIfNotPresent(ammended) {
		panic("child already present")
	}
	return ammended
}

func newRebase(isOnServer bool, isAutoRebaseOfChildren bool,
	treeVersion uint64,
	rootDirHash [32]byte, hasRebaseConflicts bool, old, newParent *Commit,
	diffCounts tree.TotalDiffCounts) Commit {
	if old.Status != StatusLatest {
		panic("tried to rebase obsolete commit")
	}
	if old.IsSubmitted {
		panic("tried to rebase submitted")
	}

	var bReason BirthReason
	var oReason ObsoleteReason
	if isAutoRebaseOfChildren {
		bReason = BirthReasonAutoRebaseOfChildren
		oReason = ObsoleteReasonAutoRebaseOfChildren
	} else {
		bReason = BirthReasonManualRebase
		oReason = ObsoleteReasonManualRebase
	}

	rebased := newCommit(
		bReason,
		old.L,
		old.Version+1,
		old.HasServerL,
		old.ServerL,
		/*HasServerV*/ false,
		/*ServerV*/ 0,
		treeVersion,
		rootDirHash,
		newParent.L,
		newParent.Version,
		newParent.HasServerL,
		newParent.ServerL,
		newParent.HasServerV,
		newParent.ServerV,
		newParent.TreeVersion,
		/*Children*/ nil,
		/*ChildrenV*/ nil,
		StatusLatest,
		/*SuccessorV*/ 0,
		ObsoleteReasonNone,
		hasRebaseConflicts,
		time.Now(),
		old.Message,
		/*IsSubmitted*/ false,
		/*IsHidden*/ false,
		/*IsRestoreOfVersion*/ 0,
		isOnServer,
		/*AuthorUserId*/ 0,
		/*IsDetached*/ false,
		/*IsRollbackOf*/ 0,
		/*IsRollbackOfV*/ 0,
		/*FirstVersionIsRollback*/ old.FirstVersionIsRollback,
		/*HasDiffData*/ true,
		/*DiffDataLinesCreated*/ diffCounts.LinesCreated,
		/*DiffDataLinesDeleted*/ diffCounts.LinesDeleted,
		/*DiffDataLinesModified*/ diffCounts.LinesModified,
		/*DiffDataFilesCreated*/ diffCounts.FilesCreated,
		/*DiffDataFilesDeleted*/ diffCounts.FilesDeleted,
		/*DiffDataFilesModified*/ diffCounts.FilesModified,
	)
	old.SetSucessor(rebased.Version, oReason)
	if !newParent.AddChildIfNotPresent(rebased) {
		panic("parent already had child")
	}
	return rebased
}

func newRebaseSubmit(treeVersion uint64,
	rootDirHash [32]byte, old, newParent *Commit,
	diffCounts tree.TotalDiffCounts) Commit {
	if old.Status != StatusLatest {
		panic("tried to rebase obsolete commit")
	}
	if old.IsSubmitted {
		panic("tried to rebase submitted")
	}
	if !newParent.IsSubmitted {
		panic("the new parent must be submitted first")
	}
	checkCommitOnTheServerOrDie(*old)
	checkCommitOnTheServerOrDie(*newParent)

	submitted := newCommit(
		BirthReasonSubmit,
		old.L,
		old.Version+1,
		/*HasServerL*/ true,
		/*ServerL*/ old.ServerL, // same as L
		/*HasServerV*/ true,
		/*ServerV*/ old.ServerV+1, // Same as Version
		treeVersion,
		rootDirHash,
		newParent.L,
		newParent.Version,
		/*HasParentServerL*/ true,
		newParent.ServerL, // Same as L
		/*HasParentServerV*/ true,
		newParent.ServerV, // Same as Version
		newParent.TreeVersion,
		/*Children*/ nil,
		/*ChildrenV*/ nil,
		StatusLatest,
		/*SuccessorV*/ 0,
		ObsoleteReasonNone,
		/*HasRebaseConflicts*/ false,
		time.Now(),
		old.Message,
		/*IsSubmitted*/ true,
		/*IsHidden*/ false,
		/*IsRestoreOfVersion*/ 0,
		/*isOnServer*/ true,
		/*AuthorUserId*/ old.AuthorUserId,
		/*IsDetached*/ false,
		/*IsRollbackOf*/ 0,
		/*IsRollbackOfV*/ 0,
		/*FirstVersionIsRollback*/ old.FirstVersionIsRollback,
		/*HasDiffData*/ true,
		/*DiffDataLinesCreated*/ diffCounts.LinesCreated,
		/*DiffDataLinesDeleted*/ diffCounts.LinesDeleted,
		/*DiffDataLinesModified*/ diffCounts.LinesModified,
		/*DiffDataFilesCreated*/ diffCounts.FilesCreated,
		/*DiffDataFilesDeleted*/ diffCounts.FilesDeleted,
		/*DiffDataFilesModified*/ diffCounts.FilesModified,
	)
	old.SetSucessor(submitted.Version, ObsoleteReasonSubmit)
	if !newParent.AddChildIfNotPresent(submitted) {
		panic("parent already had child")
	}
	return submitted
}

func newRestoreCommit(isOnServer bool, latest *Commit, old Commit, oldParent *Commit) Commit {
	if old.Status == StatusLatest {
		panic("tried to restore non obsolete commit")
	}
	if old.IsSubmitted {
		panic("tried to restore submitted")
	}
	if latest.Status != StatusLatest {
		panic("latest was obsolete")
	}

	restore := newCommit(
		BirthReasonRestore,
		old.L,
		latest.Version+1,
		latest.HasServerL,
		latest.ServerL,
		/*HasServerV*/ false,
		/*ServerV*/ 0,
		old.TreeVersion,
		old.RootDirHash,
		old.ParentL,
		old.ParentV,
		old.HasParentServerL,
		old.ParentServerL,
		old.HasParentServerV,
		old.ParentServerV,
		old.ParentTreeVersion,
		/*Children*/ nil,
		/*ChildrenV*/ nil,
		StatusLatest,
		/*SuccessorV*/ 0,
		ObsoleteReasonNone,
		old.HasRebaseConflicts,
		time.Now(),
		old.Message,
		/*IsSubmitted*/ false,
		/*IsHidden*/ false,
		/*IsRestoreOfVersion*/ old.Version,
		isOnServer,
		/*AuthorUserId*/ old.AuthorUserId,
		old.IsDetached,
		/*IsRollbackOf*/ 0,
		/*IsRollbackOfV*/ 0,
		/*FirstVersionIsRollback*/ old.FirstVersionIsRollback,
		/*HasDiffData*/ old.HasDiffData,
		/*DiffDataLinesCreated*/ old.DiffDataLinesCreated,
		/*DiffDataLinesDeleted*/ old.DiffDataLinesDeleted,
		/*DiffDataLinesModified*/ old.DiffDataLinesModified,
		/*DiffDataFilesCreated*/ old.DiffDataFilesCreated,
		/*DiffDataFilesDeleted*/ old.DiffDataFilesDeleted,
		/*DiffDataFilesModified*/ old.DiffDataFilesModified,
	)

	latest.SetSucessor(restore.Version, ObsoleteReasonRestored)
	if !oldParent.AddChildIfNotPresent(restore) {
		panic("old already had child")
	}
	return restore
}

func isSameish(incoming, local Commit) bool {
	if incoming.RootDirHash != local.RootDirHash {
		return false
	}
	if incoming.Message != local.Message {
		return false
	}
	if incoming.IsSubmitted != local.IsSubmitted {
		return false
	}
	if incoming.ParentServerL != local.ParentServerL {
		return false
	}
	if incoming.ParentServerV != local.ParentServerV {
		return false
	}
	return true
}

func newLocal(isOnServer bool,
	incoming Commit, treeVersion uint64, treeHash [32]byte,
	hasLocalVersion bool, latestLocalVersion *Commit, localParent *Commit,
	nextLocalId LocalId) (local Commit, usedNextLocalId bool) {
	if treeHash != incoming.RootDirHash {
		panic("inconsisten hash")
	}
	if hasLocalVersion {
		if latestLocalVersion.Status != StatusLatest {
			panic("latest local version should be non obsolete")
		}
		if latestLocalVersion.IsSubmitted {
			panic("latest local version is submitted and cant be modified")
		}
		if !latestLocalVersion.HasServerL {
			panic("latest local version doesnt have server id")
		}
		if latestLocalVersion.ServerL != incoming.ServerL {
			panic("latest local version server id mismatch")
		}
		if incoming.IsOnServer() &&
			latestLocalVersion.ServerL == incoming.ServerL &&
			latestLocalVersion.ServerV == incoming.ServerV {
			panic("commit already present locally")
		}
	}
	if localParent != nil {
		if !localParent.HasServerL || !localParent.HasServerV {
			panic("shared parents must have serverL and serverV to be identifiable")
		}
		if localParent.ServerL != incoming.ParentServerL {
			panic("wrong local parent L")
		}
		if localParent.ServerV != incoming.ParentServerV {
			panic("wrong local parent v")
		}
	}

	var localId LocalId
	var version uint64
	if hasLocalVersion {
		localId = latestLocalVersion.L
		version = latestLocalVersion.Version + 1
		usedNextLocalId = false
	} else {
		localId = nextLocalId
		version = 0
		usedNextLocalId = true
	}
	var serverL LocalId
	var serverV uint64
	if isOnServer {
		serverL = localId
		serverV = version
	} else {
		if !incoming.HasServerL || !incoming.HasServerV {
			panic("commits sent to the client must have global identifiers")
		}
		serverL = incoming.ServerL
		serverV = incoming.ServerV
	}

	if localParent != nil {
		local = newCommit(
			incoming.Birth,
			localId,
			version,
			/*HasServerL*/ true,
			serverL,
			/*HasServerV*/ true,
			serverV,
			treeVersion,
			treeHash,
			localParent.L,
			localParent.Version,
			localParent.HasServerL,
			localParent.ServerL,
			localParent.HasServerV,
			localParent.ServerV,
			localParent.TreeVersion,
			/*Children*/ nil,
			/*ChildrenVersion*/ nil,
			StatusLatest,
			/*SuccessorVersion*/ 0,
			ObsoleteReasonNone,
			incoming.HasRebaseConflicts,
			incoming.CreatedOn,
			incoming.Message,
			incoming.IsSubmitted,
			/*IsHidden*/ false,
			/*IsRestoreOfVersion*/ 0,
			isOnServer,
			/*AuthorUserId*/ 0,
			/*IsDetached*/ false,
			/*IsRollbackOf*/ 0,
			/*IsRollbackOfV*/ 0,
			/*FirstVersionIsRollback*/ incoming.FirstVersionIsRollback,
			/*HasDiffData*/ incoming.HasDiffData,
			/*DiffDataLinesCreated*/ incoming.DiffDataLinesCreated,
			/*DiffDataLinesDeleted*/ incoming.DiffDataLinesDeleted,
			/*DiffDataLinesModified*/ incoming.DiffDataLinesModified,
			/*DiffDataFilesCreated*/ incoming.DiffDataFilesCreated,
			/*DiffDataFilesDeleted*/ incoming.DiffDataFilesDeleted,
			/*DiffDataFilesModified*/ incoming.DiffDataFilesModified,
		)
	} else {
		local = newCommit(
			incoming.Birth,
			localId,
			version,
			/*HasServerL*/ true,
			serverL,
			/*HasServerV*/ true,
			serverV,
			treeVersion,
			treeHash,
			/*ParentLocalId*/ 0,
			/*ParentLocalVersion*/ 0,
			incoming.HasParentServerL,
			incoming.ParentServerL,
			incoming.HasParentServerV,
			incoming.ParentServerV,
			/*ParentTreeVersion*/ 0,
			/*Children*/ nil,
			/*ChildrenVersion*/ nil,
			StatusLatest,
			/*SuccessorVersion*/ 0,
			ObsoleteReasonNone,
			incoming.HasRebaseConflicts,
			incoming.CreatedOn,
			incoming.Message,
			incoming.IsSubmitted,
			/*IsHidden*/ false,
			/*IsRestoreOfVersion*/ 0,
			isOnServer,
			/*AuthorUserId*/ 0,
			/*IsDetached*/ true,
			/*IsRollbackOf*/ 0,
			/*IsRollbackOfV*/ 0,
			/*FirstVersionIsRollback*/ incoming.FirstVersionIsRollback,
			/*HasDiffData*/ incoming.HasDiffData,
			/*DiffDataLinesCreated*/ incoming.DiffDataLinesCreated,
			/*DiffDataLinesDeleted*/ incoming.DiffDataLinesDeleted,
			/*DiffDataLinesModified*/ incoming.DiffDataLinesModified,
			/*DiffDataFilesCreated*/ incoming.DiffDataFilesCreated,
			/*DiffDataFilesDeleted*/ incoming.DiffDataFilesDeleted,
			/*DiffDataFilesModified*/ incoming.DiffDataFilesModified,
		)
	}
	if isOnServer {
		local.ClientBuildVersion = incoming.ClientBuildVersion
	} else {
		local.ServerBuildVersion = incoming.ServerBuildVersion
	}
	if localParent != nil {
		if !localParent.AddChildIfNotPresent(local) {
			panic("local parent already had child")
		}
	}
	if hasLocalVersion {
		if isOnServer {
			latestLocalVersion.SetSucessor(local.Version,
				ObsoleteReasonPushOverwrite)
		} else {
			latestLocalVersion.SetSucessor(local.Version,
				ObsoleteReasonPullOverwrite)
		}
	}
	return
}

func newTrivialSubmit(old, oldParent *Commit) Commit {
	if old.Status != StatusLatest {
		panic("old must be non obsolete")
	}
	if old.IsSubmitted {
		panic("old already submitted")
	}
	if old.HasRebaseConflicts {
		panic("cant submit commit with conflicts")
	}
	if !oldParent.IsSubmitted {
		panic("old parent must be submitted first")
	}
	if old.ParentL != oldParent.L {
		panic("inconsistend ids")
	}
	if old.ParentV != oldParent.Version {
		panic("inconsistend ids")
	}
	checkCommitOnTheServerOrDie(*old)
	checkCommitOnTheServerOrDie(*oldParent)

	submitted := newCommit(
		BirthReasonSubmit,
		old.L,
		old.Version+1,
		/*HasServerL*/ true,
		old.ServerL,
		/*HasServerV*/ true,
		old.Version+1,
		old.TreeVersion,
		old.RootDirHash,
		oldParent.L,
		oldParent.Version,
		/*HasParentServerL*/ true,
		oldParent.L,
		/*HasParentServerV*/ true,
		oldParent.Version,
		oldParent.TreeVersion,
		/*Children*/ nil,
		/*ChildrenVers*/ nil,
		StatusLatest,
		/*SuccessorVersion*/ 0,
		ObsoleteReasonNone,
		/*HasRebaseConflicts*/ false,
		time.Now(),
		old.Message,
		/*IsSubmitted*/ true,
		/*IsHidden*/ false,
		/*IsRestoredVersion*/ 0,
		/*isOnServer*/ true,
		/*AuthorUserId*/ old.AuthorUserId,
		/*IsDetached*/ false,
		/*IsRollbackOf*/ 0,
		/*IsRollbackOfV*/ 0,
		/*FirstVersionIsRollback*/ old.FirstVersionIsRollback,
		/*HasDiffData*/ true,
		/*DiffDataLinesCreated*/ old.DiffDataLinesCreated,
		/*DiffDataLinesDeleted*/ old.DiffDataLinesDeleted,
		/*DiffDataLinesModified*/ old.DiffDataLinesModified,
		/*DiffDataFilesCreated*/ old.DiffDataFilesCreated,
		/*DiffDataFilesDeleted*/ old.DiffDataFilesDeleted,
		/*DiffDataFilesModified*/ old.DiffDataFilesModified,
	)
	old.SetSucessor(submitted.Version, ObsoleteReasonSubmit)
	if !oldParent.AddChildIfNotPresent(submitted) {
		panic("parent already had child")
	}
	return submitted
}

// Checks that a commit is on the server. I.e id has server V and L and they
// match with the local L and V.
func checkCommitOnTheServerOrDie(c Commit) {
	if !c.HasServerL || !c.HasServerV {
		panic("submitted commits must have server full ids")
	}
	if c.ServerL != c.L || c.ServerV != c.Version {
		panic("inconsistent server IDs")
	}
	if !c.HasParentServerL || !c.HasParentServerV {
		panic("missing parent server IDs")
	}
	if c.ParentL != c.ParentServerL || c.ParentV != c.ParentServerV {
		panic("inconsistent server parent IDs")
	}
	if c.IsDetached {
		panic("detached commit on the server")
	}

	// Leave this commented for now as we're keeping this backwards compat
	if c.ServerBuildVersion == "" {
		panic("server commit without ServerBuildVersion")
	}
}

func (c Commit) writeTo(w io.Writer) error {
	encoder := gob.NewEncoder(w)
	err := encoder.Encode(c)
	return err
}

func (c *Commit) readFrom(r io.Reader) error {
	decoder := gob.NewDecoder(r)
	return decoder.Decode(&c)
}

func (c Commit) bytes() []byte {
	buff := bytes.NewBuffer(nil)
	encoder := gob.NewEncoder(buff)
	err := encoder.Encode(c)
	if err != nil {
		panic("commit encoding failed: " + err.Error())
	}
	return buff.Bytes()
}

func (c *Commit) populateFromReader(r io.Reader) error {
	decoder := gob.NewDecoder(r)
	return decoder.Decode(&c)
}
func (c *Commit) setParentServerLV(parent Commit) bool {
	if !parent.IsOnServer() {
		panic("parent is not on serve")
	}
	if c.HasParentServerL && c.HasParentServerV {
		return false
	}
	if c.HasParentServerL && parent.ServerL != c.ParentServerL {
		panic("tried to change parentServerL of commit")
	}
	if c.HasParentServerV && parent.ServerV != c.ParentServerV {
		panic("tried to change parentServerV of commit")
	}
	c.HasParentServerL = true
	c.HasParentServerV = true
	c.ParentServerL = parent.ServerL
	c.ParentServerV = parent.ServerV
	return true
}

func attachToLocalParent(c *Commit, parent *Commit) {
	if !c.IsDetached {
		panic(fmt.Sprintf("#%d is not detached", c.L))
	}

	// Check if the parent is on the server looking at both c and parent.
	if !parent.IsOnServer() {
		panic("parent must be on the server to attach a detached commit to it")
	}
	if !c.ParentIsOnServer() {
		panic(fmt.Sprintf("#%d doesn't know its parent server ids", c.L))
	}

	// Ensure the values match
	if c.ParentServerL != parent.ServerL || c.ParentServerV != parent.ServerV {
		panic(fmt.Sprintf("c/%dv%d is not the parent of #%d",
			parent.ServerL, parent.ServerV, c.L))
	}

	c.IsDetached = false
	c.ParentL = parent.L
	c.ParentV = parent.Version
	c.ParentTreeVersion = parent.TreeVersion
	addChildIfNotPresent(parent, *c)
}

// Return nil if child already there.
func addChildIfNotPresent(c *Commit, child Commit) (added bool) {
	for i := range c.Children {
		if c.Children[i] == child.L && c.ChildrenVersions[i] == child.Version {
			added = false
			return
		}
	}
	c.Children = append(c.Children, child.L)
	c.ChildrenVersions = append(c.ChildrenVersions, child.Version)
	added = true
	return
}

func hasChild(c, child Commit) bool {
	for i := range c.Children {
		if c.Children[i] == child.L && c.ChildrenVersions[i] == child.Version {
			return true
		}
	}
	return false
}