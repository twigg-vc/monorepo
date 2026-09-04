package twiggtoken

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"strings"
	"time"
)

func newTwiggToken(repoId uint64,
	commitServerId uint64,
	commitVersion uint64,
	actions []TokenAction,
	actionsArg []string,
	duration time.Duration, s TokenSigner) (string, error) {

	if len(actions) == 0 {
		return "", errors.New("token must contain at least one action")
	}
	if len(actions) != len(actionsArg) {
		return "", fmt.Errorf("actions and actionsArg must have same len. len(actions)=%d, len(actionsArg)=%d", len(actions), len(actionsArg))
	}
	token := ParsedToken{
		RepoId:         repoId,
		CommitServerId: commitServerId,
		CommitVersion:  commitVersion,
		Actions:        actions,
		ActionsArg:     actionsArg,
		ExpiresAt:      time.Now().Add(duration),
	}

	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(token)
	if err != nil {
		return "", fmt.Errorf("failed to encode ParsedToken err=%s", err)
	}

	msg := base64.StdEncoding.EncodeToString(buf.Bytes())
	signed := s.SignAndAppend(msg)

	return Prefix + signed, nil
}

func parseToken(token string, s TokenSigner) (ParsedToken, bool, error) {
	if !strings.HasPrefix(token, Prefix) {
		return ParsedToken{}, false, errors.New("invalid prefix")
	}

	// Remove prefix
	signed := token[len(Prefix):]
	msg, ok := s.VerifyAndExtract(signed)
	if !ok {
		return ParsedToken{}, false, errors.New("invalid signature")
	}

	var parsedToken ParsedToken
	rawMsg, err := base64.StdEncoding.DecodeString(msg)
	if err != nil {
		return ParsedToken{}, false, fmt.Errorf("failed to decode string msg. err=%s", err)
	}

	err = gob.NewDecoder(bytes.NewBuffer(rawMsg)).Decode(&parsedToken)
	if err != nil {
		return ParsedToken{}, false, fmt.Errorf("failed to decode raw msg. err=%s", err)
	}
	if len(parsedToken.Actions) != len(parsedToken.ActionsArg) {
		return ParsedToken{}, false, fmt.Errorf("len(parsedToken.Actions)=%d and len(parsedToken.ActionsArg)=%d are different", len(parsedToken.Actions), len(parsedToken.ActionsArg))
	}

	if time.Now().After(parsedToken.ExpiresAt) {
		return parsedToken, true, errors.New("token expired")
	}

	return parsedToken, false, nil
}

func (pt ParsedToken) supports(action TokenAction, arg string) bool {
	for i := range pt.Actions {
		if pt.Actions[i] == action && pt.ActionsArg[i] == arg {
			return true
		}
	}
	return false
}

// Header used to carry a twigg api key
const twiggTokenKeyHeader = "X-Twigg-Token"