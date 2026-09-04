package admindash

import (
	"monorepo/squeue"
	"monorepo/twigg-track/wrappers"
)

func AddHandlers(queueStorage squeue.SqliteStorage, queueRunner squeue.Runner, mux wrappers.AuthMux) {
	h := handler{
		queueStorage: queueStorage,
		queueRunner:  queueRunner,
	}
	mux.HandleFunc("GET /", h.handleGet)
	mux.HandleFunc("GET /logs", h.handleGetLogs)
	mux.HandleFunc("GET /queued", h.handleGetQueued)
	mux.HandleFunc("GET /deadletter", h.handleGetDeadLetter)
	mux.HandleFunc("PUT /deadletter/requeue", h.handlePutRequeue)
}
