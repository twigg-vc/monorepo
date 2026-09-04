package usereducation

import (
	"context"
	"net/http"

	"monorepo/twigg-web/education"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/wrappers"
)

func AddHandlers(db Db, userMux wrappers.UserMux) {
	h := NewHandler(db)

	userMux.HandleFuncR("GET "+routes.UserEducation, h.HandleGetUserEducation)
	userMux.HandleFuncW("PUT "+routes.UserEducationWelcomeWasShown, h.HandlePutWelcomeWasShown)
}

type Handler struct {
	h handler
}

func NewHandler(db Db) Handler {
	return Handler{handler{db: db}}
}

func (h Handler) HandleGetUserEducation(
	w http.ResponseWriter,
	r wrappers.UserMuxRequest,
	dbRead context.Context,
) {
	h.h.HandleGetUserEducation(w, r, dbRead)
}

func (h Handler) HandlePutWelcomeWasShown(
	w http.ResponseWriter,
	r wrappers.UserMuxRequest,
	dbWrite context.Context,
) (shouldCommit bool) {
	return h.h.HandlePutWelcomeWasShown(w, r, dbWrite)
}

type Db interface {
	GetUserEducation(ctx context.Context, userId int64) (education.UserEducation, error)
	SetWelcomeWasShown(writeCtx context.Context, userId int64, welcomeWasShown bool) error
}

type GetUserEducationResponse struct {
	WelcomeWasShown bool
}