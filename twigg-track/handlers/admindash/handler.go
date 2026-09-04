package admindash

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"monorepo/base/iterator"
	"monorepo/squeue"
	"monorepo/twigg-track/wrappers"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

//go:embed admin.html
var adminHTML string

type handler struct {
	queueStorage squeue.SqliteStorage
	queueRunner  squeue.Runner
}

func (h handler) handleGet(w http.ResponseWriter, r wrappers.AuthMuxRequest) {
	w.Header().Set("Content-Type", "text/html")
	http.ServeContent(w, r.Request, "admin.html", time.Now(), strings.NewReader(adminHTML))
}

func (h handler) handleGetQueued(w http.ResponseWriter, r wrappers.AuthMuxRequest) {
	allQueueItemsIter, unlock, err := h.queueStorage.GetAllQueued()
	if err != nil {
		log.Printf("failed to get QueueItemsIter: %s", err)
		http.Error(w, "failed to get QueueItemsIter", http.StatusInternalServerError)
		return
	}
	defer unlock()
	const allQueueItemsLenLimit = 100
	allQueueItems, err := iterator.GetFirstN(allQueueItemsLenLimit, allQueueItemsIter)
	if err != nil {
		log.Printf("failed to to iterate through all QueueItems: %s", err)
		http.Error(w, "failed to iterate through all QueueItem", http.StatusInternalServerError)
		return
	}
	type frontendQueueItem struct {
		Id          int64
		PayloadType string
		Payload     string
		CreatedAt   string
		AvailableAt string
		RetryCount  int64
	}
	items := make([]frontendQueueItem, 0, len(allQueueItems))
	for _, item := range allQueueItems {
		s, ok := h.queueRunner.GetDisplayString(item.PayloadType, item.Payload)
		if !ok {
			s = ""
		}
		items = append(items, frontendQueueItem{
			Id:          item.Id,
			PayloadType: item.PayloadType,
			Payload:     s,
			CreatedAt:   item.CreatedAt,
			AvailableAt: item.AvailableAt,
			RetryCount:  item.RetryCount,
		})
	}

	allQueueItemsB, err := json.Marshal(items)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal queueItems: %s", err))
	}
	w.Write(allQueueItemsB)
}

func (h handler) handleGetDeadLetter(w http.ResponseWriter, r wrappers.AuthMuxRequest) {
	allDeadLetterItemsIter, unlock, err := h.queueStorage.GetAllDeadLetter()
	if err != nil {
		log.Printf("failed to get allDeadLetterItemsIter: %s", err)
		http.Error(w, "failed to get allDeadLetterItemsIter", http.StatusInternalServerError)
		return
	}
	defer unlock()
	const allDeadLetterItemsLenLimit = 100
	allDeadLetterItems, err := iterator.GetFirstN(allDeadLetterItemsLenLimit,
		allDeadLetterItemsIter)
	if err != nil {
		log.Printf("failed to to iterate through all DeadLetterItems: %s", err)
		http.Error(w, "failed to get DeadLetterItem", http.StatusInternalServerError)
		return
	}
	type frontendDeadletterItem struct {
		Id                int64
		PayloadType       string
		Payload           string
		OriginalCreatedAt string
		FailedAt          string
		RetryCount        int64
	}
	items := make([]frontendDeadletterItem, 0, len(allDeadLetterItems))
	for _, item := range allDeadLetterItems {
		s, ok := h.queueRunner.GetDisplayString(item.PayloadType, item.Payload)
		if !ok {
			s = ""
		}
		items = append(items, frontendDeadletterItem{
			Id:                item.Id,
			PayloadType:       item.PayloadType,
			Payload:           s,
			OriginalCreatedAt: item.OriginalCreatedAt,
			FailedAt:          item.FailedAt,
			RetryCount:        item.RetryCount,
		})
	}
	allDeadLetterItemsB, err := json.Marshal(items)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal deadLetterItems: %s", err))
	}
	w.Write(allDeadLetterItemsB)
}
func (h handler) handlePutRequeue(w http.ResponseWriter, r wrappers.AuthMuxRequest) {
	type requeueRequest struct {
		Id int64
	}
	var req requeueRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	err = h.queueStorage.RequeueDeadLetter(req.Id)
	if err != nil {
		http.Error(w, "failed to requeue", http.StatusInternalServerError)
		return
	}
	w.Write([]byte("ok"))
}

func (h handler) handleGetLogs(w http.ResponseWriter, r wrappers.AuthMuxRequest) {
	n := "100"
	q := r.Request.URL.Query()
	nString := q.Get("numLines")
	if nString != "" {
		n = nString
	}
	cmd := exec.Command("journalctl", "-u", "twigg-track.service",
		"-n", n, "-o", "json")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("error reading journalctl: %s", err)
		http.Error(w, "error reading journalctl", http.StatusInternalServerError)
		return
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var listOfJsons []map[string]interface{}
	for _, line := range lines {
		var jsonEntry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &jsonEntry); err != nil {
			log.Printf("failed to parse line: %s", err)
			continue
		}
		listOfJsons = append(listOfJsons, jsonEntry)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(listOfJsons)
}
