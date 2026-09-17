package indexer_test

import (
	"context"
	"monorepo/twigg-web/services/indexer"
	"reflect"
	"testing"
	"time"
)

func Test_CommitSearch_DoesNotIndexWithoutAnInterval(t *testing.T) {
	db := newDbMock()
	calls := 0
	db.indexCommitsForSearch = func(after string, limit int) (
		next string, done bool, err error) {
		calls += 1
		return
	}
	cs := indexer.NewCommitSearch(db, 0 /*batchSize=*/, 10)
	cs.Start()
	defer cs.Stop()

	time.Sleep(2 * time.Millisecond)

	if calls != 0 {
		t.Fatalf("indexed %d times, want 0", calls)
	}
}

func Test_CommitSearch_IndexesEveryInterval(t *testing.T) {
	db := newDbMock()
	gotIndexCalls := []string{}
	// Mock that indexing service will say to call:
	// [0, 1], [0, 2], [1, 0]
	// After the last one is called, it'll say it's done
	mockNextToIndex := map[string]string{
		"0/0": "0/1",
		"0/1": "0/2",
		"0/2": "1/0",
	}
	expectedIndexCalls := []string{
		"0/0",
		"0/1",
		"0/2",
		"1/0",
	}
	db.indexCommitsForSearch = func(after string, limit int) (
		string, bool, error) {
		gotIndexCalls = append(gotIndexCalls, after)
		next, ok := mockNextToIndex[after]
		if !ok {
			return "", true, nil
		}
		return next, false, nil
	}
	cs := indexer.NewCommitSearch(db, time.Millisecond /*batchSize=*/, 2)
	cs.Start()
	defer cs.Stop()

	// Check the index calls
	for len(gotIndexCalls) != len(expectedIndexCalls) {
		time.Sleep(time.Millisecond)
	}
	if !reflect.DeepEqual(gotIndexCalls, expectedIndexCalls) {
		t.Fatalf("unexpected index calls: %v", gotIndexCalls)
	}
}

type dbMock struct {
	calls                 int
	beginWrite            func() (context.Context, func(), func() error, error)
	indexCommitsForSearch func(after string, limit int) (
		next string, done bool, err error)
}

func newDbMock() *dbMock {
	return &dbMock{
		beginWrite: func() (context.Context, func(), func() error, error) {
			return nil, func() {}, func() error { return nil }, nil
		},
		indexCommitsForSearch: func(after string, limit int) (
			string, bool, error) {
			return after, false, nil
		},
	}
}

func (m *dbMock) BeginWrite() (context.Context, func(), func() error, error) {
	return m.beginWrite()
}

func (m *dbMock) IndexCommitsForSearch(w context.Context,
	after string, limit int) (
	string, bool, error) {
	return m.indexCommitsForSearch(after, limit)
}
