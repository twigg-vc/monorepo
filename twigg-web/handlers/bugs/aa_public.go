package bugs

import (
	"context"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/bugsearch"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/user"
	twiggwc "monorepo/twigg-web/webcomponents"
	"monorepo/twigg-web/wrappers"
)

func AddHandlers(db Db, perms Permissions, readMux wrappers.UserWithReadPermissionMux,
	userRepoMux wrappers.UserRepoMux) {
	h := handler{db: db, perms: perms}
	readMux.HandleFuncR("GET "+routes.BugsPattern, h.handleGetBugs)
	readMux.HandleFuncR("GET "+routes.BugSearchPattern, h.handleSearchBugs)
	readMux.HandleFuncR("GET "+routes.BugPattern, h.handleGetBug)
	userRepoMux.HandleFuncW("POST "+routes.BugsPattern, h.handlePostBug)
	userRepoMux.HandleFuncW("POST "+routes.BugCommentsPattern, h.handlePostComment)
	userRepoMux.HandleFuncW("POST "+routes.BugStatusPattern, h.handlePostStatus)
	userRepoMux.HandleFuncW("POST "+routes.BugDescriptionPattern, h.handlePostDescription)
	userRepoMux.HandleFuncW("POST "+routes.BugTitlePattern, h.handlePostTitle)
	userRepoMux.HandleFuncW("POST "+routes.BugAssigneePattern, h.handlePostAssignee)
}

type Db interface {
	GetUsername(ctx context.Context, userId int64) (username string, isNotFoundErr bool, err error)
	GetUserByUsername(ctx context.Context, username string) (u user.User, isNotFoundErr bool, err error)
	GetBugsPage(ctx context.Context, repoId uint64, status bug.Status,
		cursor string, limit int) (bugs []bug.Bug, nextCursor string, err error)
	SearchBugs(ctx context.Context, f bugsearch.Filter, cursor string,
		limit int) (bugs []bug.Bug, nextCursor string, err error)
	CountRepoBugs(ctx context.Context, repoId uint64) (open, closed int64, err error)
	CreateBug(writeCtx context.Context, repoId uint64, authorId int64, title, body string) (bug.Bug, error)
	GetBug(ctx context.Context, repoId uint64, number uint64) (b bug.Bug, isNotFoundErr bool, err error)
	GetBugEvents(ctx context.Context, repoId uint64, number uint64,
		cursor string, limit int) (events []bug.Event, nextCursor string, err error)
	AddBugComment(writeCtx context.Context, repoId uint64, number uint64,
		authorId int64, body string) (e bug.Event, isNotFoundErr bool, err error)
	SetBugStatus(writeCtx context.Context, repoId uint64, number uint64,
		authorId int64, status bug.Status) (e bug.Event, isNotFoundErr bool, err error)
	EditBugDescription(writeCtx context.Context, repoId uint64, number uint64,
		authorId int64, newBody string) (e bug.Event, isNotFoundErr bool, err error)
	EditBugTitle(writeCtx context.Context, repoId uint64, number uint64,
		authorId int64, newTitle string) (e bug.Event, isNotFoundErr bool, err error)
	SetBugAssignee(writeCtx context.Context, repoId uint64, number uint64,
		authorId int64, assigneeUserId int64) (e bug.Event, isNotFoundErr bool, err error)
	CreateNotification(writeCtx context.Context, userId int64, message string, assetPath string) error
}

// u is nil for anonymous users
type Permissions interface {
	CanCreateBugs(r context.Context, u *user.User, rp repo.Repo) (bool, error)
	CanWriteBug(r context.Context, u *user.User, rp repo.Repo, b bug.Bug) (bool, error)
	CanBeAssignedBugs(r context.Context, u user.User, rp repo.Repo) (bool, error)
}

// NextCursor is empty on the last page. The counts are of the whole repo.
type GetBugsResponse struct {
	Bugs        []twiggwc.FrontendBug
	NextCursor  string
	OpenCount   int64
	ClosedCount int64
	CanCreate   bool
}

// NextCursor is empty on the last page.
type BugSearchResponse struct {
	Bugs       []twiggwc.FrontendBug
	NextCursor string
}

type PostBugRequest struct {
	Title string
	Body  string
}

type PostCommentRequest struct {
	Body string
}

// Comment is optional. When set, it's recorded before the status change.
type PostStatusRequest struct {
	Status  bug.Status
	Comment string
}

// Events are the new ones, oldest first
type PostStatusResponse struct {
	Bug    twiggwc.FrontendBug
	Events []twiggwc.FrontendBugEvent
}

type PostEditResponse struct {
	Bug   twiggwc.FrontendBug
	Event twiggwc.FrontendBugEvent
}

type PostTitleRequest struct {
	Title string
}

// An empty Username unassigns the bug.
type PostAssigneeRequest struct {
	Username string
}