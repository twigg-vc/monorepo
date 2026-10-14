package twiggwebclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// General function to make a request. Returns error for any non200 status.
func (c cliclient) doRequest(method string, url string, body io.Reader, out any) (int, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return 0, err
	}
	xchange.SetApiKeyHeader(c.twiggCliKey, req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrFailedToReachServer, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, fmt.Errorf("%w: %s", ErrFailedToReachServer, resp.Status)
	}
	if out == nil {
		return 0, nil
	}
	decoder := json.NewDecoder(resp.Body)
	return 0, decoder.Decode(out)
}

func (c cliclient) ping() (ok bool, err error) {
	_, err = c.doRequest("GET", c.repoUrl+"/ping", nil, nil)
	ok = err == nil
	return
}

var _ int = func() int {
	if routes.CliClientPing != "/{owner}/{repo}/ping" {
		panic("CliClientPing pattern changed! did you update the client?")
	}
	return 0
}()

func (c cliclient) GetBugAndEvents(id uint64) (b GetBugAndEventsResponse, isNotFoundErr bool, err error) {
	status, err := c.doRequest("GET", fmt.Sprintf("%s/cli-b/%d", c.repoUrl, id), nil, &b)
	isNotFoundErr = status == http.StatusNotFound
	return
}

var _ int = func() int {
	if routes.CliBugPattern != "/{owner}/{repo}/cli-b/{bug}" {
		panic("CliBugsPattern pattern changed! did you update the client?")
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
