package cli

import (
	"fmt"
	"monorepo/twigg-web/bug"
	twiggwebclient "monorepo/twigg-web/twigg-web-client"
	"strings"
	"time"
)

// Simple helper that logs an error and returns false if the server and api
// were not setup yet.
func (a *app) checkServerIsSetup() (ok bool) {
	if a.s.ServerUrl == "" {
		a.logError(serverUrlNotSet)
		return
	}
	if a.s.ApiKey == "" {
		a.logError(apiKeyNotSet)
		return
	}
	return true
}

func (a *app) getTwiggWebClient() twiggwebclient.CliKeyClient {
	return twiggwebclient.NewCliKeyClient(a.s.ServerUrl, a.s.ApiKey)
}

func (a *app) pingTwiggWeb(args commandArgs) {
	if !a.checkServerIsSetup() {
		return
	}

	client := a.getTwiggWebClient()
	ok, err := client.Ping()
	if err != nil || !ok {
		a.logError(err.Error())
		return
	}
	a.logSuccess("ok")
}

func (a *app) setCommitDescription(args commandArgs) {
	if !a.checkServerIsSetup() {
		return
	}
	var serverId uint64
	if args.commit0 == "" {
		if !a.s.Current.HasServerL {
			a.logError(cantDescribeNotPushed)
			return
		}
		serverId = uint64(a.s.Current.ServerL)
	} else {
		match, id, hasVersion, _ := a.tryParseServerCommitString(args.commit0)
		if !match {
			a.logError(badCommitServerSyntax)
			return
		}
		if hasVersion {
			a.logError(cantDescribeCommitVersion)
			return
		}
		serverId = id
	}
	client := a.getTwiggWebClient()
	err := client.SetCommitDescription(serverId, args.description)
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.logSuccess("ok")
}

func (a *app) getBug(args commandArgs) {
	if !a.checkServerIsSetup() {
		return
	}
	client := a.getTwiggWebClient()
	bug, isNotFoundErr, err := client.GetBugAndEvents(args.bugNumber)
	if isNotFoundErr {
		a.logError(fmt.Sprintf("b/%d not found", args.bugNumber))
		return
	}
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.logInfo(bugToString(bug))
}

// Human readable version of a bug and its events
func bugToString(b twiggwebclient.GetBugAndEventsResponse) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "b/%d %s [%s]\n", b.Bug.Number, b.Bug.Title, b.Bug.Status)
	assignee := b.Bug.AssigneeUsername
	if assignee == "" {
		assignee = "nobody"
	}
	fmt.Fprintf(&sb, "author: %s | assignee: %s | comments: %d\n",
		b.Bug.AuthorUsername, assignee, b.Bug.CommentCount)
	fmt.Fprintf(&sb, "created: %s | updated: %s\n",
		bugTime(b.Bug.CreatedOn), bugTime(b.Bug.UpdatedOn))
	if strings.TrimSpace(b.Bug.Body) != "" {
		sb.WriteString("\n" + b.Bug.Body + "\n")
	}
	if len(b.Events) > 0 {
		sb.WriteString("\n")
	}
	for _, e := range b.Events {
		sb.WriteString(bugEventString(e) + "\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func bugEventString(e twiggwebclient.FrontendBugEvent) string {
	who := fmt.Sprintf("[%s %s]", bugTime(e.CreatedOn), e.AuthorUsername)
	switch e.Kind {
	case twiggwebclient.FrontendBugEventKind_Comment:
		body := ""
		if e.Comment != nil {
			body = e.Comment.Body
		}
		return who + " commented:\n" + body
	case twiggwebclient.FrontendBugEventKind_StatusChange:
		if e.StatusChange != nil && e.StatusChange.NewStatus == bug.Status_Closed {
			return who + " closed the bug"
		}
		return who + " reopened the bug"
	case twiggwebclient.FrontendBugEventKind_DescriptionEdit:
		return who + " edited the description"
	case twiggwebclient.FrontendBugEventKind_TitleEdit:
		return who + " edited the title"
	case twiggwebclient.FrontendBugEventKind_SubmittedCommit:
		if e.SubmittedCommit != nil {
			return who + fmt.Sprintf(" submitted c/%d", e.SubmittedCommit.CommitL)
		}
		return who + " submitted a commit"
	case twiggwebclient.FrontendBugEventKind_ClosingCommit:
		if e.ClosingCommit != nil {
			return who + fmt.Sprintf(" closed the bug with c/%d", e.ClosingCommit.CommitL)
		}
		return who + " closed the bug with a commit"
	case twiggwebclient.FrontendBugEventKind_Assignment:
		if e.Assignment != nil && e.Assignment.NewAssigneeUsername != "" {
			return who + " assigned the bug to " + e.Assignment.NewAssigneeUsername
		}
		return who + " unassigned the bug"
	}
	return who + " " + string(e.Kind)
}

func bugTime(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04")
}
