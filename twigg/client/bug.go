package client

import (
	"fmt"
	"monorepo/twigg-web/bug"
)

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
