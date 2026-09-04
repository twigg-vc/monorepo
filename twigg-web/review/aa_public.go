// Package review holds the entities of commit reviews.
package review

import "time"

// Data of the review of a commit
type Data struct {
	// Description of the commit being reviewed
	Description string
	// Indicates the commit is a "work-in-progress"
	IsWIP bool
	// Indicates the commit is archived
	IsArchived bool

	// Indicates the current review status of the latest version
	ReviewStatus ReviewStatus

	// Following variables are used to cheaply keep the ReviewStatus up to date
	ReviewStatusLgtmCount       int64
	ReviewStatusUnresolvedCount int64

	// Indicates user ids
	ReviewersUserIds []int64
}

// Recomputes ReviewStatus from the unresolved/LGTM counters.
func (d *Data) SetReviewStatus() {
	if d.ReviewStatusUnresolvedCount > 0 {
		d.ReviewStatus = ReviewStatus_Unresolved
		return
	}
	if d.ReviewStatusLgtmCount > 0 {
		d.ReviewStatus = ReviewStatus_Ready
		return
	}
	d.ReviewStatus = ReviewStatus_MissingLgtm
}

type Thread struct {
	// (RepositoryName + CommitId + ThreadId) uniquely identify a thread.
	Id int64
	// Indicates the type of thread this is
	Type ThreadType
	// Id of the user who created the thread
	AuthorUserId int64
	// Indicates the commit version to which the thread is anchored
	CommitVersion uint64
	// Used to indicate if resolved or not
	IsResolved bool
	// Used to indicate which file the thread is anchored to.
	// Can be set to "" (for LGTM/Discussion threads for example).
	Filename string
	// Filename the thread is anchored to. It's 1-based: i.e. 1 -> first line.
	// 0 means the thread is anchored to the whole file
	Line uint64
	// Used for AddLGTM/RemoveLGTM to indicate if this is an LGTM add
	// or an LGTM remove
	IsLgtm bool
	// When the thread was created.
	CreatedOn time.Time
}

// IsInline says whether the thread is anchored to a specific line of a file
func (t Thread) IsInline() bool {
	return t.Filename != "" && t.Line != 0
}

type Comment struct {
	ThreadId     int64
	AuthorUserId int64
	Text         string
	T            time.Time
}

type ThreadType uint32

const (
	ThreadType_None ThreadType = iota
	// Comment thread on a file on a specific version of a commit
	ThreadType_CommentsOnFileOnCommitVersion
	// Comment thread on a commit version not anchored to a file
	// (aka a discussion thread)
	ThreadType_CommentsOnCommitVersion
	// Not a comment thread "per-se". It's just an adition of an LGTM
	// at a specific commit version.
	ThreadType_AddLGTM
	// Not a comment thread "per-se". It's just an removal of an LGTM
	// at a specific commit version.
	ThreadType_RemoveLGTM
)

type ReviewStatus uint32

const (
	ReviewStatus_MissingLgtm ReviewStatus = iota
	ReviewStatus_MissingOwnersApproval
	ReviewStatus_Ready
	ReviewStatus_Unresolved
)