package bugs

import (
	"context"
	"monorepo/twigg-web/bug"
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
	userRepoMux.HandleFuncW("POST "+routes.BugsPattern, h.handlePostBug)
}

type Db interface {
	GetUsername(ctx context.Context, userId int64) (username string, isNotFoundErr bool, err error)
	GetBugsPage(ctx context.Context, repoId uint64, status bug.Status,
		cursor string, limit int) (bugs []bug.Bug, nextCursor string, err error)
	CountRepoBugs(ctx context.Context, repoId uint64) (open, closed int64, err error)
	CreateBug(writeCtx context.Context, repoId uint64, authorId int64, title, body string) (bug.Bug, error)
}

// u is nil for anonymous users
type Permissions interface {
	CanCreateBugs(r context.Context, u *user.User, rp repo.Repo) (bool, error)
	CanWriteBug(r context.Context, u *user.User, rp repo.Repo, b bug.Bug) (bool, error)
}

// NextCursor is empty on the last page. The counts are of the whole repo.
type GetBugsResponse struct {
	Bugs        []twiggwc.FrontendBug
	NextCursor  string
	OpenCount   int64
	ClosedCount int64
	CanCreate   bool
}

type PostBugRequest struct {
	Title string
	Body  string
}
