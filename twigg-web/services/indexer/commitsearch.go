package indexer

import (
	"log"
	"sync"
	"time"
)

type commitSearch struct {
	db        Db
	interval  time.Duration
	batchSize int
	cursor    string
	isDone    bool
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

func (cs *commitSearch) Start() {
	if cs.interval <= 0 {
		log.Print("[commit search] the indexer is disabled")
		return
	}
	cs.wg.Add(1)
	go func() {
		defer cs.wg.Done()
		ticker := time.NewTicker(cs.interval)
		for {
			select {
			case <-cs.stopCh:
				return
			case <-ticker.C:
				if cs.isDone {
					continue
				}
				cs.indexOneBatch()
			}
		}
	}()
}

func (cs *commitSearch) Stop() {
	close(cs.stopCh)
	cs.wg.Wait()
}

// The cursor only moves once the batch is committed, so a failed batch is
// read again on the next tick.
func (cs *commitSearch) indexOneBatch() {
	w, closeTx, commitTx, err := cs.db.BeginWrite()
	if err != nil {
		log.Printf("[commit search] failed to begin the write: %s", err)
		return
	}
	defer closeTx()
	next, done, err := cs.db.IndexCommitsForSearch(w, cs.cursor, cs.batchSize)
	if err != nil {
		log.Printf("[commit search] failed to index the batch after %+v: %s",
			cs.cursor, err)
		return
	}
	err = commitTx()
	if err != nil {
		log.Printf("[commit search] failed to commit the batch: %s", err)
		return
	}
	cs.cursor = next
	cs.isDone = done
	if done {
		log.Print("[commit search] every commit is indexed")
	}
}
