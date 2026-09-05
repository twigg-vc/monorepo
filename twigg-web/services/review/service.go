package review

import (
	"context"
	"errors"
	"fmt"
	"monorepo/base/iterator"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/review"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg-web/user"
	"monorepo/twigg/commit"
	"time"
)

type service struct {
	db          Db
	owners      OwnersChecker
	userService userservice.Service
}

func new(db Db, owners OwnersChecker, userService userservice.Service) (Service, error) {
	return service{db, owners, userService}, nil
}

func (s service) GetData(r context.Context, repoId uint64, cId commit.LocalId, checkOwners bool,
	cIdToReadOwners uint64, supremeLeaders []string) (d review.Data, isNotFoundErr bool, err error) {
	hasRev, err := s.db.HasReview(r, repoId, cId)
	if err != nil {
		return
	}
	if !hasRev {
		// Initialize default empty fields and return
		d = review.Data{
			Description:                 "",
			IsWIP:                       false,
			IsArchived:                  false,
			ReviewStatus:                review.ReviewStatus_MissingLgtm,
			ReviewStatusLgtmCount:       0,
			ReviewStatusUnresolvedCount: 0,
		}
		isNotFoundErr = true
		err = errors.New("not found")
		return
	}
	d, err = s.db.GetReviewData(r, repoId, cId)
	if err != nil {
		return
	}
	d.SetReviewStatus() // Ensure review status is correct
	if !checkOwners {
		return
	}
	if d.ReviewStatus != review.ReviewStatus_Ready {
		return
	}

	it, err := s.GetLgtmAuthors(r, repoId, cId)
	if err != nil {
		err = fmt.Errorf("failed to get LGTM authors: %w", err)
		return
	}

	var usersWhoLgtmd []string
	const maxAuthorsToRead = 1_000
	for it.Next() {
		userId, getErr := it.Get()
		if getErr != nil {
			err = fmt.Errorf("eff getting user by id: %w", err)
			return
		}

		user, _, getErr := s.userService.Get(r, userId)
		if getErr != nil {
			err = fmt.Errorf("err to iterator: %w", err)
			return
		}

		usersWhoLgtmd = append(usersWhoLgtmd, user.Username)
		if len(usersWhoLgtmd) > maxAuthorsToRead {
			err = fmt.Errorf("too many users LGTM's")
			return
		}
	}
	err = it.Err()
	if err != nil {
		err = fmt.Errorf("err to iterator: %w", err)
		return
	}

	ok, errOwners := s.owners.OwnersLgmtIsOk(
		repoId,
		uint64(cId),
		usersWhoLgtmd,
		cIdToReadOwners,
		supremeLeaders,
		r,
	)
	if errOwners != nil {
		err = fmt.Errorf("failed to check owners LGTM: %w", errOwners)
		return
	}

	if !ok {
		d.ReviewStatus = review.ReviewStatus_MissingOwnersApproval
	}
	return
}
func (s service) SetDescription(w context.Context, quotaOwner string, repoId uint64, cId commit.LocalId, desc string, createIfNeeded bool) error {
	if len(desc) > MaxDescriptionLength {
		return fmt.Errorf("description cant be > %d", MaxDescriptionLength)
	}
	d, isNotFoundErr, err := s.GetData(w, repoId, cId, false, uint64(0), []string{})
	if err != nil && !isNotFoundErr {
		return err
	}
	if isNotFoundErr {
		if createIfNeeded {
			d.Description = desc
			d.ReviewStatus = review.ReviewStatus_MissingLgtm
			return s.setData(w, quotaOwner, repoId, cId, d)
		}
		return err
	}
	d.Description = desc
	return s.setData(w, quotaOwner, repoId, cId, d)
}

func (s service) setData(w context.Context, quotaOwner string, repoId uint64, cId commit.LocalId, d review.Data) error {
	err := s.db.CreateReviewIfNotExists(w, repoId, cId)
	if err != nil {
		return err
	}

	return s.db.SetReviewData(w, quotaOwner, repoId, cId, d)
}

func (s service) GetThread(r context.Context, threadId int64) (review.Thread, error) {
	return s.db.GetReviewThread(r, threadId)
}

func (s service) CreateThread(w context.Context, quotaOwner string, repoId uint64, cId commit.LocalId, cV uint64, file string, line uint64, userId int64, commentText string, resolved bool) (th review.Thread, err error) {
	if file == "" {
		panic("tried to create thread without file")
	}
	if line > MaxCommentLine {
		err = fmt.Errorf("line cant be > %d", MaxCommentLine)
		return
	}
	th, err = s.createThreadAndComment(w, quotaOwner, repoId, cId, cV,
		file, line, userId, commentText, resolved)
	return
}

func (s service) CreateDiscussionThread(w context.Context, quotaOwner string, repoId uint64, cId commit.LocalId, cV uint64, userId int64, commentText string, resolved bool) (th review.Thread, err error) {
	// A discussion thread is just a thread but with filename=""
	return s.createThreadAndComment(w, quotaOwner, repoId, cId, cV, "",
		/*line=*/ 0, userId, commentText, resolved)
}

// Use file="" and line=0 to create discussion
func (s service) createThreadAndComment(w context.Context,
	quotaOwner string, repoId uint64, cId commit.LocalId, cV uint64,
	file string, line uint64,
	userId int64, commentText string, resolved bool) (th review.Thread, err error) {

	var threadType review.ThreadType
	if file == "" {
		threadType = review.ThreadType_CommentsOnCommitVersion
	} else {
		threadType = review.ThreadType_CommentsOnFileOnCommitVersion
	}

	threadId, err := s.db.CreateReviewThread(w, repoId, cId, userId, uint32(threadType))
	if err != nil {
		return
	}
	th = review.Thread{
		Id:            threadId,
		Type:          threadType,
		AuthorUserId:  userId,
		CommitVersion: cV,
		Filename:      file,
		Line:          line,
		IsResolved:    resolved,
		CreatedOn:     time.Now(),
	}
	err = s.db.SetReviewThread(w, quotaOwner, threadId, th)
	if err != nil {
		return
	}

	var commentId int64
	commentId, err = s.db.CreateReviewComment(w, repoId, cId, threadId, userId)
	if err != nil {
		return
	}
	cm := review.Comment{
		ThreadId:     th.Id,
		AuthorUserId: userId,
		Text:         commentText,
		T:            time.Now(),
	}
	err = s.db.SetReviewComment(w, quotaOwner, commentId, cm)
	if err != nil {
		return
	}
	if !resolved {
		var d review.Data
		var isNotFoundErr bool
		d, isNotFoundErr, err = s.GetData(w, repoId, cId, false, uint64(0), []string{})
		if err != nil && !isNotFoundErr {
			return
		}
		d.ReviewStatusUnresolvedCount++
		d.SetReviewStatus()
		err = s.setData(w, quotaOwner, repoId, cId, d)
	}
	return
}

type threadIter struct {
	db        Db
	threadIds iterator.I[int64]
	r         context.Context
}

func (ti threadIter) Get() (review.Thread, error) {
	threadId, err := ti.threadIds.Get()
	if err != nil {
		return review.Thread{}, err
	}
	return ti.db.GetReviewThread(ti.r, threadId)
}
func (ti threadIter) Next() bool {
	return ti.threadIds.Next()
}
func (ti threadIter) Err() error {
	return ti.threadIds.Err()
}

func (s service) GetThreads(r context.Context, repoId uint64, cId commit.LocalId, ascending bool) (iterator.I[review.Thread], error) {
	threadIds, err := s.db.GetReviewThreadIds(r, repoId, cId)
	if err != nil {
		return nil, err
	}
	return threadIter{db: s.db, r: r, threadIds: threadIds}, nil
}

func (s service) AddToThread(w context.Context, quotaOwner string, repoId uint64, cId commit.LocalId, threadId int64, userId int64, commentText string, resolved bool) (th review.Thread, err error) {
	th, err = s.GetThread(w, threadId)
	if err != nil {
		return
	}
	if th.Type != review.ThreadType_CommentsOnCommitVersion &&
		th.Type != review.ThreadType_CommentsOnFileOnCommitVersion {
		err = fmt.Errorf("tried to add comment to non-comment thread")
		return
	}
	threadWasResolved := th.IsResolved
	threadIsNowResolved := resolved
	if th.IsResolved != resolved {
		th.IsResolved = resolved

		err = s.db.SetReviewThread(w, quotaOwner, threadId, th)
		if err != nil {
			return
		}
	}
	var commentId int64
	commentId, err = s.db.CreateReviewComment(w, repoId, cId, threadId, userId)
	if err != nil {
		return
	}
	cm := review.Comment{
		ThreadId:     th.Id,
		AuthorUserId: userId,
		Text:         commentText,
		T:            time.Now(),
	}
	err = s.db.SetReviewComment(w, quotaOwner, commentId, cm)
	if err != nil {
		return
	}
	if threadWasResolved == threadIsNowResolved {
		return
	}
	var d review.Data
	var isNotFoundErr bool
	d, isNotFoundErr, err = s.GetData(w, repoId, cId, false, uint64(0), []string{})
	if err != nil && !isNotFoundErr {
		return
	}
	// not resolved -> resolved
	if !threadWasResolved && threadIsNowResolved {
		d.ReviewStatusUnresolvedCount--
	}
	// resolved -> unresolved
	if threadWasResolved && !threadIsNowResolved {
		d.ReviewStatusUnresolvedCount++
	}
	d.SetReviewStatus()
	err = s.setData(w, quotaOwner, repoId, cId, d)
	return
}

type commentIter struct {
	db         Db
	commentIds iterator.I[int64]
	r          context.Context
}

func (ti commentIter) Get() (review.Comment, error) {
	commentId, err := ti.commentIds.Get()
	if err != nil {
		return review.Comment{}, err
	}
	return ti.db.GetReviewComment(ti.r, commentId)
}
func (ti commentIter) Next() bool {
	return ti.commentIds.Next()
}
func (ti commentIter) Err() error {
	return ti.commentIds.Err()
}

func (s service) GetComments(r context.Context, repoId uint64, cId commit.LocalId, threadId int64) (it iterator.I[review.Comment], err error) {
	commentIds, err := s.db.GetReviewCommentIds(r, repoId, cId, threadId)
	if err != nil {
		return nil, err
	}
	return commentIter{db: s.db, r: r, commentIds: commentIds}, nil
}

func (s service) GetUserLastLgtm(r context.Context, repoId uint64, cId commit.LocalId, userId int64) (review.Thread, bool, error) {
	threadId, isNotFoundErr, err := s.db.GetReviewUserLastLgtmThreadId(r, repoId, cId, userId,
		uint32(review.ThreadType_AddLGTM), uint32(review.ThreadType_RemoveLGTM))
	if isNotFoundErr {
		return review.Thread{}, true, err
	}
	if err != nil {
		return review.Thread{}, false, err
	}
	th, err := s.db.GetReviewThread(r, threadId)
	if err != nil {
		return review.Thread{}, false, err
	}
	return th, false, nil
}

func (s service) HasLgtm(r context.Context, repoId uint64, cId commit.LocalId, userId int64) (bool, error) {
	lg, isNotFoundErr, err := s.GetUserLastLgtm(r, repoId, cId, userId)
	if isNotFoundErr {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return lg.IsLgtm, nil
}
func (s service) AddLgtm(w context.Context,
	quotaOwner string, repoId uint64, cId commit.LocalId, cV uint64, userId int64) (review.Thread, error) {
	lastLgtm, isNotFoundErr, err := s.GetUserLastLgtm(w, repoId, cId, userId)
	if err != nil && !isNotFoundErr {
		return review.Thread{}, err
	}
	if !isNotFoundErr && lastLgtm.IsLgtm && lastLgtm.CommitVersion == cV {
		return review.Thread{}, fmt.Errorf("lgtm already present at version %d", cV)
	}
	if !isNotFoundErr && lastLgtm.CommitVersion > cV {
		return review.Thread{}, fmt.Errorf("lgtm present at version %d, cant add at %d",
			lastLgtm.CommitVersion, cV)
	}

	threadId, err := s.db.CreateReviewThread(w, repoId, cId, userId, uint32(review.ThreadType_AddLGTM))
	if err != nil {
		return review.Thread{}, err
	}
	th := review.Thread{
		Id:            threadId,
		Type:          review.ThreadType_AddLGTM,
		AuthorUserId:  userId,
		CommitVersion: cV,
		IsLgtm:        true,
		IsResolved:    true,
		CreatedOn:     time.Now(),
	}
	err = s.db.SetReviewThread(w, quotaOwner, threadId, th)
	if err != nil {
		return th, err
	}

	var d review.Data
	d, isNotFoundErr, err = s.GetData(w, repoId, cId, false, uint64(0), []string{})
	if err != nil && !isNotFoundErr {
		return review.Thread{}, err
	}
	d.ReviewStatusLgtmCount++
	d.SetReviewStatus()
	err = s.setData(w, quotaOwner, repoId, cId, d)
	if err != nil {
		return review.Thread{}, err
	}
	return th, nil
}
func (s service) RemoveLastLgtm(w context.Context,
	quotaOwner string, repoId uint64, cId commit.LocalId, userId int64) (review.Thread, error) {
	// Read last lgtm just to make sure it exists and get the commit version
	lastLgtm, _, err := s.GetUserLastLgtm(w, repoId, cId, userId)
	if err != nil {
		return review.Thread{}, err
	}

	threadId, err := s.db.CreateReviewThread(w, repoId, cId, userId, uint32(review.ThreadType_RemoveLGTM))
	if err != nil {
		return review.Thread{}, err
	}
	th := review.Thread{
		Id:            threadId,
		Type:          review.ThreadType_RemoveLGTM,
		AuthorUserId:  userId,
		CommitVersion: lastLgtm.CommitVersion,
		IsLgtm:        false,
		IsResolved:    true,
		CreatedOn:     time.Now(),
	}
	err = s.db.SetReviewThread(w, quotaOwner, threadId, th)
	if err != nil {
		return th, err
	}

	var d review.Data
	var isNotFoundErr bool
	d, isNotFoundErr, err = s.GetData(w, repoId, cId, false, uint64(0), []string{})
	if err != nil && !isNotFoundErr {
		return review.Thread{}, err
	}
	d.ReviewStatusLgtmCount--
	d.SetReviewStatus()
	err = s.setData(w, quotaOwner, repoId, cId, d)
	if err != nil {
		return review.Thread{}, err
	}
	return th, nil
}

func (s service) GetLgtmAuthors(
	r context.Context,
	repoId uint64,
	cId commit.LocalId,
) (iterator.I[int64], error) {
	it, err := s.db.GetReviewLgtmAuthorIds(r, repoId, cId,
		uint32(review.ThreadType_AddLGTM), uint32(review.ThreadType_RemoveLGTM))
	if err != nil {
		return nil, fmt.Errorf("failed to get LGTM authors: %w", err)
	}
	return it, nil
}
func (s service) AddReviewer(
	w context.Context,
	quotaOwner string,
	repoId uint64,
	cId commit.LocalId,
	userId int64,
) error {

	d, isNotFoundErr, err := s.GetData(w, repoId, cId, false, 0, []string{})
	if err != nil && !isNotFoundErr {
		return err
	}

	// Prevent duplicates
	for _, id := range d.ReviewersUserIds {
		if id == userId {
			return nil // already reviewer
		}
	}

	if len(d.ReviewersUserIds) >= MaxReviewers {
		return fmt.Errorf("max reviewers limit (%d) reached", MaxReviewers)
	}

	d.ReviewersUserIds = append(d.ReviewersUserIds, userId)

	return s.setData(w, quotaOwner, repoId, cId, d)
}

func (s service) RemoveReviewer(
	w context.Context,
	quotaOwner string,
	repoId uint64,
	cId commit.LocalId,
	userId int64,
) error {

	d, isNotFoundErr, err := s.GetData(w, repoId, cId, false, 0, []string{})
	if err != nil && !isNotFoundErr {
		return err
	}

	newList := make([]int64, 0, len(d.ReviewersUserIds))
	found := false

	for _, id := range d.ReviewersUserIds {
		if id == userId {
			found = true
			continue
		}
		newList = append(newList, id)
	}

	if !found {
		return nil
	}

	d.ReviewersUserIds = newList

	return s.setData(w, quotaOwner, repoId, cId, d)
}

func (s service) ResolveSupremeLeaders(db context.Context, ownerUsr user.User) ([]string, error) {
	if !ownerUsr.IsOrganization {
		return []string{ownerUsr.Username}, nil
	}
	orgAssetId := permissions.OrganizationAssetId(ownerUsr.Id)
	it, err := s.db.GetUsersWithPermission(db, orgAssetId, permissions.Permission_OrganizationOwner)
	if err != nil {
		return nil, fmt.Errorf("getting org owners: %w", err)
	}
	var leaders []string
	for it.Next() {
		userId, err := it.Get()
		if err != nil {
			return nil, fmt.Errorf("iterating org owners: %w", err)
		}
		u, _, err := s.userService.Get(db, userId)
		if err != nil {
			return nil, fmt.Errorf("getting org owner user: %w", err)
		}
		leaders = append(leaders, u.Username)
	}
	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("iterating org owners: %w", err)
	}
	return leaders, nil
}