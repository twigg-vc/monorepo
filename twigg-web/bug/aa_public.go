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
	CreatedOn      time.Time
	UpdatedOn      time.Time
}

func NewBug(number uint64, title, body string, status Status,
	authorUserId, assigneeUserId int64, createdOn, updatedOn time.Time) Bug {
	return Bug{
		Number:         number,
		Title:          title,
		Body:           body,
		Status:         status,
		AuthorUserId:   authorUserId,
		AssigneeUserId: assigneeUserId,
		CreatedOn:      createdOn,
		UpdatedOn:      updatedOn,
	}
}
