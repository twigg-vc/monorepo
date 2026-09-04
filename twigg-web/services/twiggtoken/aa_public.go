package twiggtoken

import (
	"net/http"
	"time"
)

// Short lived tokens do not use for future proof needs
// Returns token with Prefix.
// len(actions) must be > 0  and len(actionsArg) must be equal to len(actions).
func NewTwiggToken(repoId uint64,
	commitServerId uint64,
	commitVersion uint64,
	actions []TokenAction,
	actionsArg []string,
	duration time.Duration, s TokenSigner) (string, error) {
	return newTwiggToken(repoId, commitServerId, commitVersion, actions, actionsArg, duration, s)
}

func ParseToken(token string, s TokenSigner) (parsedToken ParsedToken, isExpiredErr bool, err error) {
	return parseToken(token, s)
}

type ParsedToken struct {
	RepoId         uint64
	CommitServerId uint64
	CommitVersion  uint64
	Actions        []TokenAction
	ActionsArg     []string
	ExpiresAt      time.Time

	// DEPRECATED
	Action string
}

func (pt ParsedToken) Supports(action TokenAction, arg string) bool {
	return pt.supports(action, arg)
}

type TokenAction string

const (
	TokenActionPull      TokenAction = "token-action-pull"
	TokenActionPush      TokenAction = "token-action-push"
	TokenActionGetSecret TokenAction = "token-action-get-secret"
)

const Prefix string = "tw_tok_"

func GetTwiggTokenInHeader(r *http.Request) string {
	return r.Header.Get(twiggTokenKeyHeader)
}

func SetTwiggTokenInHeader(token string, r *http.Request) {
	r.Header.Set(twiggTokenKeyHeader, token)
}

type TokenSigner interface {
	// Signs a message and returns msg+signature
	SignAndAppend(msg string) string
	// Receives a msg+signature and returns the msg and a boolean indicating
	// if the signature is ok
	VerifyAndExtract(msgAndSig string) (msg string, isOk bool)
}