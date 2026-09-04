package usereducation

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"monorepo/twigg-web/wrappers"
)

type handler struct {
	db Db
}

func (hl handler) HandleGetUserEducation(
	w http.ResponseWriter,
	r wrappers.UserMuxRequest,
	dbRead context.Context,
) {
	ed, err := hl.db.GetUserEducation(dbRead, r.User.Id)
	if err != nil {
		log.Printf("failed to get user education: %v", err)
		http.Error(w, "internal error getting user education", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(GetUserEducationResponse{
		WelcomeWasShown: ed.WelcomeWasShown,
	})
}

func (hl handler) HandlePutWelcomeWasShown(
	w http.ResponseWriter,
	r wrappers.UserMuxRequest,
	dbWrite context.Context,
) (shouldCommit bool) {
	err := hl.db.SetWelcomeWasShown(dbWrite, r.User.Id, true)
	if err != nil {
		log.Printf("failed to set welcome was shown: %v", err)
		http.Error(w, "internal error setting welcome was shown", http.StatusInternalServerError)
		shouldCommit = false
		return
	}
	shouldCommit = true
	return
}