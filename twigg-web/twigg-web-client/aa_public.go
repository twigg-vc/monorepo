package twiggwebclient

import (
	"monorepo/twigg-web/bug"
	"net/http"
	"time"
)

// MUST BE INITIALIZED WITH `NewClient`
// Client for services to communicate with the twigg web server
type Client struct {
	c client
}

// Instantiates a new Client ready to be used
func NewClient(twiggServerUrl string, twiggServerKey string) Client {
	return Client{
		c: client{
			httpClient:     &http.Client{},
			twiggServerUrl: twiggServerUrl,
			twiggServerKey: twiggServerKey,
		},
	}
}

func (c Client) GetSecretValsFromTwiggWeb(
	requiredSecretsNames []string,
	twiggToken string,
) (requiredSecrets map[string]string, isNotFoundOrForbiddenErr bool, err error) {
	return c.c.getSecretValsFromTwiggWeb(requiredSecretsNames, twiggToken)
}

// MUST BE INITIALIZED WITH `NewClient`
// Client for services to communicate with the twigg web server via CLI key
type CliKeyClient struct {
	c cliclient
}

// Instantiates a new Client ready to be used
func NewCliKeyClient(repoUrl string, twiggCliKey string) CliKeyClient {
	return CliKeyClient{
		c: cliclient{
			httpClient:  &http.Client{},
			repoUrl:     repoUrl,
			twiggCliKey: twiggCliKey,
		},
	}
}

func (c CliKeyClient) Ping() (ok bool, err error) {
	return c.c.ping()
}

func (c CliKeyClient) GetBugAndEvents(id uint64) (b GetBugAndEventsResponse, isNotFoundErr bool, err error) {
	return c.c.GetBugAndEvents(id)
}

// AssigneeUsername is empty when unassigned
type FrontendBug struct {
	Number           uint64
	Title            string
	Body             string
	Status           bug.Status
	AuthorUsername   string
	AssigneeUsername string
	CommentCount     int64
	CreatedOn        time.Time
	UpdatedOn        time.Time
}

type FrontendBugEventKind string

const (
	FrontendBugEventKind_Comment         FrontendBugEventKind = "comment"
	FrontendBugEventKind_StatusChange    FrontendBugEventKind = "status-change"
	FrontendBugEventKind_DescriptionEdit FrontendBugEventKind = "description-edit"
	FrontendBugEventKind_TitleEdit       FrontendBugEventKind = "title-edit"
	FrontendBugEventKind_SubmittedCommit FrontendBugEventKind = "submitted-commit"
	FrontendBugEventKind_Assignment      FrontendBugEventKind = "assignment"
)

// Only the details field matching Kind is set
type FrontendBugEvent struct {
	Id              uint64
	Kind            FrontendBugEventKind
	AuthorUsername  string
	CreatedOn       time.Time
	Comment         *bug.Comment         `json:",omitempty"`
	StatusChange    *bug.StatusChange    `json:",omitempty"`
	DescriptionEdit *bug.DescriptionEdit `json:",omitempty"`
	TitleEdit       *bug.TitleEdit       `json:",omitempty"`
	SubmittedCommit *bug.SubmittedCommit `json:",omitempty"`
	Assignment      *FrontendAssignment  `json:",omitempty"`
}

// NewAssigneeUsername is empty when the bug was unassigned
type FrontendAssignment struct {
	NewAssigneeUsername string
}

func BugToFrontend(b bug.Bug, authorUsername, assigneeUsername string) FrontendBug {
	return bugToFrontend(b, authorUsername, assigneeUsername)
}

// newAssigneeUsername is only used by assignment events, and is empty when
// the bug was unassigned.
func BugEventToFrontend(e bug.Event, authorUsername, newAssigneeUsername string) FrontendBugEvent {
	return bugEventToFrontend(e, authorUsername, newAssigneeUsername)
}

type GetBugAndEventsResponse struct {
	Bug    FrontendBug
	Events []FrontendBugEvent
}
