package indexer

import (
	"context"
	"sync"
	"time"
)

type Db interface {
	BeginWrite() (writeCtx context.Context, closeTx func(), commitTx func() error, err error)
	IndexCommitsForSearch(w context.Context, after string,
		limit int) (next string, done bool, err error)
}

type CommitSearch struct {
	i *commitSearch
}

// Starts indexing entries until Stop() is called.
func (cs CommitSearch) Start() {
	cs.i.Start()
}

func (cs CommitSearch) Stop() {
	cs.i.Stop()
}

func NewCommitSearch(db Db, interval time.Duration,
	batchSize int) CommitSearch {
	return CommitSearch{&commitSearch{
		db:        db,
		interval:  interval,
		batchSize: batchSize,
		cursor:    "",
		stopCh:    make(chan struct{}),
		wg:        sync.WaitGroup{},
	}}
}
