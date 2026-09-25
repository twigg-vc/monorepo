package bugs

import (
	"context"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/routes"
	twiggwc "monorepo/twigg-web/webcomponents"
	"monorepo/twigg-web/wrappers"
)

func AddHandlers(db Db, readMux wrappers.UserWithReadPermissionMux) {
	h := handler{db: db}
	readMux.HandleFuncR("GET "+routes.BugsPattern, h.handleGetBugs)
}

type Db interface {
	GetUsername(ctx context.Context, userId int64) (username string, isNotFoundErr bool, err error)
	GetBugsPage(ctx context.Context, repoId uint64, status bug.Status,
		cursor string, limit int) (bugs []bug.Bug, nextCursor string, err error)
}

// NextCursor is empty on the last page.
type GetBugsResponse struct {
	Bugs       []twiggwc.FrontendBug
	NextCursor string
}
