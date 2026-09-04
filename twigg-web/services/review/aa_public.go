package review

import (
	"context"
	"monorepo/base/iterator"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/review"
	"monorepo/twigg-web/services/user"
	"monorepo/twigg/commit"
)

// Service related to data of commit reviews
type Service interface {

	// GetData retrieves the Data associated with the given repository and
	// commit ID. If no Data is found, isNotFoundErr will be true, and the
	// returned Data will have ReviewStatus set to MissingLgtm, with
	// all other fields set to their default values.
	// If checkOwners is true, the method will evaluate whether the commit
	// satisfies the ownership rules.
	GetData(r context.Context, repoId uint64, cId commit.LocalId, checkOwners bool,
		cIdToReadOwners uint64, supremeLeaders []string) (d review.Data, isNotFoundErr bool, err error)
	// SetDescription updates the description of the Data associated with the
	// given repository and commit ID. If createIfNeeded is true and no Data
	// exists, a new Data will be created with Description set to desc and
	// ReviewStatus set to MissingLgtm, with all other fields set to
	// their default values. If createIfNeeded is false, it updates the existing
	// Data or returns an error if none is found.
	// Note that an error is returned if the description is larger than MaxDescriptionLength
	SetDescription(w context.Context, quotaOwner string, repoId uint64, cId commit.LocalId, desc string, createIfNeeded bool) error

	// CreateThread creates a thread anchored to file.
	// `line` is the 1-based line of the file it's anchored to. Use line=0
	// to anchor it to the file as a whole. line must be <= MaxCommentLine.
	CreateThread(w context.Context, quotaOwner string, repoId uint64, cId commit.LocalId, cV uint64, file string, line uint64, userId int64, commentText string, resolved bool) (review.Thread, error)
	CreateDiscussionThread(w context.Context, quotaOwner string, repoId uint64, cId commit.LocalId, cV uint64, userId int64, commentText string, resolved bool) (review.Thread, error)
	GetThread(r context.Context, threadId int64) (th review.Thread, err error)
	GetThreads(r context.Context, repoId uint64, cId commit.LocalId, ascendingOrder bool) (it iterator.I[review.Thread], err error)

	// Returns user's last lgtm
	GetUserLastLgtm(r context.Context, repoId uint64, cId commit.LocalId, userId int64) (lgtm review.Thread, isNotFoundErr bool, err error)
	// Simple wraper around GetUserLastLgtm to check for LGTM of a user
	HasLgtm(r context.Context, repoId uint64, cId commit.LocalId, userId int64) (bool, error)
	// Set LGTM of the user to the provided commit version.
	// If the user has an LGTM on a past version, cV must be >= that version.
	AddLgtm(w context.Context, quotaOwner string, repoId uint64, cId commit.LocalId, cV uint64, userId int64) (review.Thread, error)
	// Removes the last LGTM of a user on a commit version
	RemoveLastLgtm(w context.Context, quotaOwner string, repoId uint64, cId commit.LocalId, userId int64) (review.Thread, error)

	AddToThread(w context.Context, quotaOwner string, repoId uint64, cId commit.LocalId, threadId int64, userId int64, commentText string, resolved bool) (review.Thread, error)
	GetComments(r context.Context, repoId uint64, cId commit.LocalId, threadId int64) (it iterator.I[review.Comment], err error)

	// GetLgtmAuthors returns all user who currently have LGTM on the commit.
	GetLgtmAuthors(r context.Context, repoId uint64, cId commit.LocalId) (iterator.I[int64], error)

	// Adds userId to reviewers list. If user is already a reviewer just return nil.
	// Returns error if it would exceed MaxReviewers.
	AddReviewer(w context.Context, quotaOwner string, repoId uint64, cId commit.LocalId, userId int64) error

	// Removes userId from reviewers list.
	RemoveReviewer(w context.Context, quotaOwner string, repoId uint64, cId commit.LocalId, userId int64) error

	// ResolveSupremeLeaders returns the usernames whose collective LGTM bypasses
	// OWNERS file requirements. For user-owned repos this is just the owner; for
	// org-owned repos it is every org owner.
	ResolveSupremeLeaders(db context.Context, ownerUsr user.User) ([]string, error)
}

// OwnersChecker computes if a certain commit passes the Owners' approvar requirements
type OwnersChecker interface {
	OwnersLgmtIsOk(repoId uint64, commitId uint64,
		usersWhoLgtmd []string,
		commitIdToReadOwners uint64,
		supremeLeaders []string,
		r context.Context) (bool, error)
}

type Db interface {
	CreateReviewIfNotExists(writeCtx context.Context, repoId uint64, cId commit.LocalId) error
	HasReview(ctx context.Context, repoId uint64, cId commit.LocalId) (bool, error)
	GetReviewData(ctx context.Context, repoId uint64, cId commit.LocalId) (review.Data, error)
	SetReviewData(writeCtx context.Context, quotaOwner string, repoId uint64, cId commit.LocalId, d review.Data) error

	CreateReviewThread(writeCtx context.Context, repoId uint64, cId commit.LocalId,
		authorId int64, threadType uint32) (threadId int64, err error)
	GetReviewThread(ctx context.Context, threadId int64) (review.Thread, error)
	SetReviewThread(writeCtx context.Context, quotaOwner string, threadId int64, th review.Thread) error
	GetReviewThreadIds(ctx context.Context, repoId uint64, cId commit.LocalId) (iterator.I[int64], error)
	GetReviewUserLastLgtmThreadId(ctx context.Context, repoId uint64, cId commit.LocalId,
		userId int64, addLgtmType, removeLgtmType uint32) (threadId int64, isNotFoundErr bool, err error)
	GetReviewLgtmAuthorIds(ctx context.Context, repoId uint64, cId commit.LocalId,
		addLgtmType, removeLgtmType uint32) (iterator.I[int64], error)

	CreateReviewComment(writeCtx context.Context, repoId uint64, cId commit.LocalId,
		threadId int64, authorId int64) (commentId int64, err error)
	GetReviewComment(ctx context.Context, commentId int64) (review.Comment, error)
	SetReviewComment(writeCtx context.Context, quotaOwner string, commentId int64, cm review.Comment) error
	GetReviewCommentIds(ctx context.Context, repoId uint64, cId commit.LocalId,
		threadId int64) (iterator.I[int64], error)

	GetUsersWithPermission(ctx context.Context, assetId string, p permissions.Permission) (iterator.I[int64], error)
}

func New(db Db, owners OwnersChecker, userService user.Service) (Service, error) {
	return new(db, owners, userService)
}

const (
	MaxDescriptionLength = 5_000
	MaxCommentLine       = 10_000_000
)

var MaxReviewers = 100
