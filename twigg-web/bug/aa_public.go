package bug

import "time"

type Status string

const (
	Status_Open   Status = "open"
	Status_Closed Status = "closed"
)

func (s Status) IsValid() bool {
	return s == Status_Open || s == Status_Closed
}

// Number is sequential per repo and is what users see (e.g. "b/3").
// AssigneeUserId is 0 when unassigned.
type Bug struct {
	Number         uint64
	Title          string
	Body           string
	Status         Status
	AuthorUserId   int64
	AssigneeUserId int64
	CommentCount   int64
	CreatedOn      time.Time
	UpdatedOn      time.Time
}

func NewBug(number uint64, title, body string, status Status,
	authorUserId, assigneeUserId int64, commentCount int64, createdOn, updatedOn time.Time) Bug {
	return Bug{
		Number:         number,
		Title:          title,
		Body:           body,
		Status:         status,
		AuthorUserId:   authorUserId,
		AssigneeUserId: assigneeUserId,
		CommentCount:   commentCount,
		CreatedOn:      createdOn,
		UpdatedOn:      updatedOn,
	}
}

// Identifies what happened in an Event. Stored in the db: never reuse a value.
type EventKind uint8

const (
	EventKind_Comment         EventKind = 1
	EventKind_StatusChange    EventKind = 2
	EventKind_DescriptionEdit EventKind = 3
)

// Something that happened to a bug, shown in its timeline. Only the details
// field matching Kind is set.
type Event struct {
	Id              uint64
	Kind            EventKind
	AuthorUserId    int64
	CreatedOn       time.Time
	Comment         Comment
	StatusChange    StatusChange
	DescriptionEdit DescriptionEdit
}

func NewEvent(id uint64, kind EventKind, authorUserId int64, createdOn time.Time) Event {
	return Event{
		Id:           id,
		Kind:         kind,
		AuthorUserId: authorUserId,
		CreatedOn:    createdOn,
	}
}

type Comment struct {
	Body string
}

func NewComment(body string) Comment {
	return Comment{Body: body}
}

type StatusChange struct {
	NewStatus Status
}

func NewStatusChange(newStatus Status) StatusChange {
	return StatusChange{NewStatus: newStatus}
}

// The new body is the OldBody of the next edit, or the bug's Body for the
// latest edit.
type DescriptionEdit struct {
	OldBody string
}

func NewDescriptionEdit(oldBody string) DescriptionEdit {
	return DescriptionEdit{OldBody: oldBody}
}
