package twiggwebclient

import (
	"errors"
	"fmt"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/routes"
	"monorepo/twigg/xchange"
	"net/http"
)

type cliclient struct {
	httpClient  *http.Client
	repoUrl     string
	twiggCliKey string
}

var (
	ErrFailedToReachServer = errors.New("failed to reach the server")
)

func (c cliclient) ping() (ok bool, err error) {
	req, err := http.NewRequest("GET", c.repoUrl+"/ping", nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create GET request to pull: %s", err))
	}
	xchange.SetApiKeyHeader(c.twiggCliKey, req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("%w: %s", ErrFailedToReachServer, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("%w: %s", ErrFailedToReachServer, resp.Status)
	}
	return true, nil
}

var _ int = func() int {
	if routes.CliClientPing != "/{owner}/{repo}/ping" {
		panic("CliClientPing pattern changed! did you update the client?")
	}
	return 0
}()

func bugToFrontend(b bug.Bug, authorUsername, assigneeUsername string) FrontendBug {
	return FrontendBug{
		Number:           b.Number,
		Title:            b.Title,
		Body:             b.Body,
		Status:           b.Status,
		AuthorUsername:   authorUsername,
		AssigneeUsername: assigneeUsername,
		CommentCount:     b.CommentCount,
		CreatedOn:        b.CreatedOn,
		UpdatedOn:        b.UpdatedOn,
	}
}

func bugEventToFrontend(e bug.Event, authorUsername, newAssigneeUsername string) FrontendBugEvent {
	fe := FrontendBugEvent{
		Id:             e.Id,
		AuthorUsername: authorUsername,
		CreatedOn:      e.CreatedOn,
	}
	switch e.Kind {
	case bug.EventKind_Comment:
		fe.Kind = FrontendBugEventKind_Comment
		fe.Comment = &e.Comment
	case bug.EventKind_StatusChange:
		fe.Kind = FrontendBugEventKind_StatusChange
		fe.StatusChange = &e.StatusChange
	case bug.EventKind_DescriptionEdit:
		fe.Kind = FrontendBugEventKind_DescriptionEdit
		fe.DescriptionEdit = &e.DescriptionEdit
	case bug.EventKind_TitleEdit:
		fe.Kind = FrontendBugEventKind_TitleEdit
		fe.TitleEdit = &e.TitleEdit
	case bug.EventKind_SubmittedCommit:
		fe.Kind = FrontendBugEventKind_SubmittedCommit
		fe.SubmittedCommit = &e.SubmittedCommit
	case bug.EventKind_Assignment:
		fe.Kind = FrontendBugEventKind_Assignment
		fe.Assignment = &FrontendAssignment{NewAssigneeUsername: newAssigneeUsername}
	default:
		panic(fmt.Sprintf("unknown bug event kind %d", e.Kind))
	}
	return fe
}
