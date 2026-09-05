package admindash

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"monorepo/base/iterator"
	"monorepo/squeue"
	"monorepo/twigg-web/metrics"
	"monorepo/twigg-web/routes"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/webcomponents"
	"monorepo/twigg-web/wrappers"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type handler struct {
	m            metrics.Service
	userService  userservice.Service
	queueStorage squeue.SqliteStorage
	queueRunner  squeue.Runner
}

func (h handler) handleGetDash(w http.ResponseWriter,
	r wrappers.AdminUserMuxRequest, dbRead context.Context) {
	allocMb, heapInUseMb, sysMb, numGcRuns := h.m.MemMb()

	nUsers, err := h.userService.CountAll(dbRead)
	if err != nil {
		log.Printf("failed to count users: %s", err)
		http.Error(w, "failed to count users", http.StatusInternalServerError)
		return
	}
	allUsersIter, err := h.userService.GetAll(dbRead)
	if err != nil {
		log.Printf("failed to get users: %s", err)
		http.Error(w, "failed to get users", http.StatusInternalServerError)
		return
	}

	var userList []user.User
	const userListLenLimit = 50
	for allUsersIter.Next() {
		u, err := allUsersIter.Get()
		if err != nil {
			http.Error(w, "failed to get users", http.StatusInternalServerError)
			return
		}
		userList = append(userList, u)
		if len(userList) > userListLenLimit {
			break
		}
	}
	err = allUsersIter.Err()
	if err != nil {
		http.Error(w, "failed to iterate through users", http.StatusInternalServerError)
		return
	}

	allQueueItemsIter, unlock, err := h.queueStorage.GetAllQueued()
	if err != nil {
		log.Printf("failed to get QueueItemsIter: %s", err)
		http.Error(w, "failed to get QueueItemsIter", http.StatusInternalServerError)
		return
	}
	defer unlock()
	const allQueueItemsLenLimit = 50
	queueItems, err := iterator.GetFirstN(allQueueItemsLenLimit, allQueueItemsIter)
	if err != nil {
		log.Printf("failed to to iterate through all QueueItems: %s", err)
		http.Error(w, "failed to iterate through all QueueItem", http.StatusInternalServerError)
		return
	}
	frontendQueueItems := make([]webcomponents.FrontendQueueItem, 0, len(queueItems))
	for _, item := range queueItems {
		s, ok := h.queueRunner.GetDisplayString(item.PayloadType, item.Payload)
		if !ok {
			s = ""
		}
		frontendQueueItems = append(frontendQueueItems, webcomponents.FrontendQueueItem{
			Id:          item.Id,
			PayloadType: item.PayloadType,
			Payload:     s,
			CreatedAt:   item.CreatedAt,
			AvailableAt: item.AvailableAt,
			RetryCount:  item.RetryCount,
		})
	}

	allDeadLetterItemsIter, unlock, err := h.queueStorage.GetAllDeadLetter()
	if err != nil {
		log.Printf("failed to get allDeadLetterItemsIter: %s", err)
		http.Error(w, "failed to get allDeadLetterItemsIter", http.StatusInternalServerError)
		return
	}
	defer unlock()
	const deadLetterItemsLenLimit = 20
	deadLetterItems, err := iterator.GetFirstN(deadLetterItemsLenLimit, allDeadLetterItemsIter)
	if err != nil {
		log.Printf("failed to to iterate through all DeadLetterItems: %s", err)
		http.Error(w, "failed to get DeadLetterItem", http.StatusInternalServerError)
		return
	}
	frontendDeadLetterItems := make([]webcomponents.FrontendDeadLetterItem, 0, len(deadLetterItems))
	for _, item := range deadLetterItems {
		s, ok := h.queueRunner.GetDisplayString(item.PayloadType, item.Payload)
		if !ok {
			s = ""
		}
		frontendDeadLetterItems = append(frontendDeadLetterItems, webcomponents.FrontendDeadLetterItem{
			Id:                item.Id,
			PayloadType:       item.PayloadType,
			Payload:           s,
			OriginalCreatedAt: item.OriginalCreatedAt,
			FailedAt:          item.FailedAt,
			RetryCount:        item.RetryCount,
		})
	}

	webcomponents.Page( /*hideNavBar=*/ false,
		r.Flags,
		webcomponents.AdminDash(h.m.Uptime(),
			allocMb, heapInUseMb, sysMb, numGcRuns, nUsers, userList,
			frontendQueueItems, frontendDeadLetterItems),
	).Render(w)
}
func (h handler) handleGetRequestCounts(w http.ResponseWriter,
	r wrappers.AdminUserMuxRequest, dbRead context.Context) {
	h.m.GetRequestCountHandler().ServeHTTP(w, r.Request)
}
func (h handler) handleGetLogs(w http.ResponseWriter,
	r wrappers.AdminUserMuxRequest, dbRead context.Context) {
	n := "100"
	q := r.Request.URL.Query()
	nString := q.Get("numLines")
	if nString != "" {
		n = nString
	}
	cmd := exec.Command("journalctl", "-u", "twigg-web.service",
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
func (h handler) handleGetMetricTs(w http.ResponseWriter,
	r wrappers.AdminUserMuxRequest, dbRead context.Context) {

	const minutesAgo = 60 * 6
	startTime := time.Now().Add(-time.Duration(minutesAgo) * time.Minute)
	endTime := time.Now()
	ts := []metrics.TimeSeriesPoint{}
	floatTs := []metrics.FloatTimeSeriesPoint{}
	isFloat := false
	var err error
	metricName := r.Request.PathValue("name")
	switch metricName {
	case metrics.TotalRequestsCounterName:
		ts, err = h.m.GetCounter(metrics.TotalRequestsCounterName, startTime, endTime)
	case metrics.MeanRequestsMillisecLatencyGaugeName:
		isFloat = true
		floatTs, err = h.m.GetMeanGauge(metrics.MeanRequestsMillisecLatencyGaugeName, startTime, endTime)
	default:
		http.Error(w, fmt.Sprintf(
			"metric %q not found", metricName), http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Printf("failed to read %q: %s", metricName, err)
		http.Error(w, fmt.Sprintf(
			"failed to read metric %q", metricName), http.StatusBadRequest)
		return
	}
	var tsJson []byte
	if isFloat {
		tsJson, err = json.Marshal(floatTs)
	} else {
		tsJson, err = json.Marshal(ts)
	}
	if err != nil {
		log.Printf("failed to serialize metric %q: %s", metricName, err)
		http.Error(w, fmt.Sprintf(
			"failed to serialize metric %q", metricName), http.StatusBadRequest)
	}
	w.Write(tsJson)
}

func (h handler) handleRequeueDeadLetter(w http.ResponseWriter,
	r wrappers.AdminUserMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	shouldCommit = false
	deadLetterIdStr := r.Request.PathValue(routes.DeadLetterIdQueryParamName)
	if deadLetterIdStr == "" {
		http.Error(w, "missing dead letter id", http.StatusBadRequest)
		return
	}
	deadLetterId, err := strconv.ParseInt(deadLetterIdStr, 10, 64)
	if err != nil {
		http.Error(w, "dead letter must be a int", http.StatusBadRequest)
		return
	}

	err = h.queueStorage.RequeueDeadLetter(deadLetterId)
	if err != nil {
		log.Printf("failed to requeue dead letter with id=%v, err=%s", deadLetterId, err)
		http.Error(w, "dead letter must be a int", http.StatusBadRequest)
		return
	}
	shouldCommit = true
	return
}