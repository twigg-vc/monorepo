package repository

import (
	"encoding/json"
	"errors"
	"monorepo/twigg-web/commitsearch"
	"monorepo/twigg-web/review"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/user"
	"monorepo/twigg/commit"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

// A handler whose repo, review and user services answer the little that
// rendering a commit needs.
func newCommitSearchHandler(searchDb *commitSearchDbMock) handler {
	repoS := &repoServiceMock{}
	repoS.getRepoTopCommit = func() (commit.Commit, error) {
		return commit.Commit{ServerL: 10}, nil
	}
	revS := &reviewServiceMock{}
	revS.resolveSupremeLeaders = func(user.User) ([]string, error) {
		return nil, nil
	}
	revS.getData = func([]string) (review.Data, bool, error) {
		return review.Data{ReviewStatus: review.ReviewStatus_Ready}, false, nil
	}
	userS := &userServiceMock{}
	userS.get = func() (user.User, bool, error) {
		return user.User{Username: "aang"}, false, nil
	}
	return NewHandler(repoS, revS, userS, searchDb)
}

func searchRequest(query, cursor string) url.Values {
	q := url.Values{}
	q.Set(routes.CommitSearchQueryParamName, query)
	if cursor != "" {
		q.Set(routes.CommitSearchCursorParamName, cursor)
	}
	return q
}

func Test_HandleCommitSearch_AnswersWithTheCommitsAndTheNextCursor(t *testing.T) {
	searchDb := &commitSearchDbMock{}
	searchDb.searchCommits = func(commitsearch.Filter, string, int) (
		[]commit.Commit, string, error) {
		return []commit.Commit{{L: 3}, {L: 2}}, "the-next-cursor", nil
	}
	h := newCommitSearchHandler(searchDb)
	w := httptest.NewRecorder()

	h.handleCommitSearch(w, newMockReq(nil, searchRequest("queue", "")), nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status is %d, want %d: %s", w.Code, http.StatusOK, w.Body)
	}
	var resp CommitSearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.NextCursor != "the-next-cursor" {
		t.Fatalf("answered the cursor %q, want the-next-cursor", resp.NextCursor)
	}
	gotIds := []uint64{}
	for _, c := range resp.Commits {
		gotIds = append(gotIds, c.L)
	}
	if !reflect.DeepEqual(gotIds, []uint64{3, 2}) {
		t.Fatalf("answered the commits %v, want [3 2]", gotIds)
	}
}

func Test_HandleCommitSearch_SearchesWhatTheQueryAsksFor(t *testing.T) {
	var gotFilter commitsearch.Filter
	var gotCursor string
	var gotLimit int
	searchDb := &commitSearchDbMock{}
	searchDb.searchCommits = func(f commitsearch.Filter, cursor string,
		limit int) ([]commit.Commit, string, error) {
		gotFilter, gotCursor, gotLimit = f, cursor, limit
		return nil, "", nil
	}
	h := newCommitSearchHandler(searchDb)
	w := httptest.NewRecorder()

	h.handleCommitSearch(w,
		newMockReq(nil, searchRequest("is:pending author:aang queue", "a-cursor")), nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status is %d, want %d: %s", w.Code, http.StatusOK, w.Body)
	}
	if gotFilter.Message != "queue" || gotFilter.AuthorUsername != "aang" ||
		gotFilter.State != commitsearch.StatePending {
		t.Fatalf("searched %+v, want the pending commits of aang about the queue",
			gotFilter)
	}
	if gotCursor != "a-cursor" {
		t.Fatalf("searched after %q, want a-cursor", gotCursor)
	}
	if gotLimit != commitSearchPageSize {
		t.Fatalf("searched %d commits, want %d", gotLimit, commitSearchPageSize)
	}
}

func Test_HandleCommitSearch_AnswersAQueryItCanNotParseWithItsReason(t *testing.T) {
	searchDb := &commitSearchDbMock{}
	searchDb.searchCommits = func(commitsearch.Filter, string, int) (
		[]commit.Commit, string, error) {
		t.Fatal("searched a query that could not be parsed")
		return nil, "", nil
	}
	h := newCommitSearchHandler(searchDb)
	w := httptest.NewRecorder()

	h.handleCommitSearch(w, newMockReq(nil, searchRequest("is:nope", "")), nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status is %d, want %d", w.Code, http.StatusBadRequest)
	}
	// The body is what the search bar shows under itself.
	if !strings.Contains(w.Body.String(), "nope") {
		t.Fatalf("the answer is %q, want it to name the bad term", w.Body)
	}
}

func Test_HandleCommitSearch_FailsWhenTheSearchOfTheDbFails(t *testing.T) {
	searchDb := &commitSearchDbMock{}
	searchDb.searchCommits = func(commitsearch.Filter, string, int) (
		[]commit.Commit, string, error) {
		return nil, "", errors.New("boom")
	}
	h := newCommitSearchHandler(searchDb)
	w := httptest.NewRecorder()

	h.handleCommitSearch(w, newMockReq(nil, searchRequest("queue", "")), nil)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status is %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
