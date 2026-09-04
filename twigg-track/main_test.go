//go:build linux
// +build linux

package main

import (
	"bytes"
	"io"
	"log"
	"math"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/runners"
	"monorepo/twigg-track/trackclient"
	"monorepo/twigg-track/trackserver"
	"net/http"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	canRunDockerJobs, canRunVmJobs, err := runners.CheckCanRun()
	if err != nil {
		log.Fatalf("CheckCanRun: %s", err)
		return
	}
	_ = canRunVmJobs // These tests don't run anything in a VM image
	if !canRunDockerJobs {
		return
	}
	os.Exit(m.Run())
}

func TestNoTrackKey(t *testing.T) {
	srv := trackserver.GetNoWebhookTestServer(t)
	c := &http.Client{}
	resp, err := c.Get(srv.C.PublicUrl)
	if err != nil {
		t.Fatalf("get failed: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", resp.StatusCode)
	}
}

func TestRateLimiter(t *testing.T) {
	withLowQps := func(c *trackserver.SrvConfig) {
		// Use a very low QPS and a burst size 2 to allow only 2 to go through
		c.RateLimitMaxQps = 0.001
		c.RateLimitMaxQpsBurst = 2
	}
	srv := trackserver.GetNoWebhookTestServer(t, withLowQps)
	c := &http.Client{}
	gotRateLimitedCount := 0
	for i := 0; i < 5; i++ {
		resp, err := c.Get(srv.Url() + "/")
		if err != nil {
			t.Fatalf("get failed: %s", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized &&
			resp.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("unexpected status: %d", resp.StatusCode)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			gotRateLimitedCount += 1
		}
	}
	if gotRateLimitedCount != 3 {
		t.Fatalf("unexpected rate limits: %d", gotRateLimitedCount)
	}
}

func TestHealth(t *testing.T) {
	srv := trackserver.GetNoWebhookTestServer(t)
	// The client HealthIsOk should return true when ok
	trackClient := trackclient.NewClient(srv.Url(), srv.TrackKey())
	if ok, err := trackClient.HealthIsOk(); !ok {
		t.Fatalf("Health check failed: %s", err)
	}
	// Unauthenticated requests should also succeed - usefull for uptime checks
	resp, err := http.Get(srv.Url() + "/health")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/health status=%d", resp.StatusCode)
	}
}

func TestPutGetAndReadOutput(t *testing.T) {
	secrets := map[string]string{"SECRET": "1234"}
	srv, hooks := trackserver.GetTestServerAndFakeWebhookListener(t, secrets)
	trackClient := trackclient.NewClient(srv.Url(), srv.TrackKey())
	payload := runnerlib.JobPayload{
		Name: "say hi",
		Steps: []runnerlib.JobStep{
			{
				Run:     "echo hi!",
				Env:     map[string]string{},
				Secrets: []string{"SECRET"},
			},
			{
				Run:     "echo bye!",
				Env:     map[string]string{},
				Secrets: []string{"SECRET"},
			},
		},
		TimeoutMilliSeconds: 5000,
	}
	err := trackClient.Put("1", payload)
	if err != nil {
		t.Fatalf("client put failed: %v", err)
	}

	_, _, isNotFoundErr, err := trackClient.Get("non existing")
	if !isNotFoundErr || err == nil {
		t.Fatalf("got no error for non existing job")
	}
	gotJob, gotPayload, isNotFoundErr, err := trackClient.Get("1")
	if isNotFoundErr || err != nil {
		t.Fatalf("get job: %v", err)
	}
	if !reflect.DeepEqual(gotPayload, payload) {
		t.Fatalf("got unexpected payload")
	}

	// Wait until the notifications (running, success) are received
	hooks.WaitFor("1", trackclient.TrackJobStatusRunning, t)
	hooks.WaitFor("1", trackclient.TrackJobStatusSuccess, t)
	// Tun run the job, the SECRET must have been read
	if !slices.Equal([]string{"SECRET"}, hooks.GetSecretLastCalledWith()) {
		t.Fatalf("got GetSecretLastCalledWith: %s", hooks.GetSecretLastCalledWith())
	}

	// Get and check job status
	gotJob, _, _, err = trackClient.Get("1")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if gotJob.Status != trackclient.TrackJobStatusSuccess {
		t.Fatalf("unexpected job status: %s", gotJob.Status)
	}

	// Read the job output
	getStdOut, err := trackClient.GetCombinedOutput("1")
	if err != nil {
		t.Fatalf("GetStdOut: %v", err)
	}
	defer getStdOut.Close()
	stdOut, err := io.ReadAll(getStdOut)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if !strings.Contains(string(stdOut), "hi!\n") ||
		!strings.Contains(string(stdOut), "bye!\n") {
		t.Fatalf("unexpected stdOut: %q", stdOut)
	}
}

func TestPutJobThatFails(t *testing.T) {
	secrets := map[string]string{}
	srv, hooks := trackserver.GetTestServerAndFakeWebhookListener(t, secrets)
	trackClient := trackclient.NewClient(srv.Url(), srv.TrackKey())
	payload := runnerlib.JobPayload{
		Name: "try to run non existing binary",
		Steps: []runnerlib.JobStep{
			{
				Run: "no-existing-binary",
			},
		},
		TimeoutMilliSeconds: 5000,
	}
	err := trackClient.Put("1", payload)
	if err != nil {
		t.Fatalf("client put failed: %v", err)
	}

	// Wait until the notifications (running, failed) are received
	hooks.WaitFor("1", trackclient.TrackJobStatusRunning, t)
	hooks.WaitFor("1", trackclient.TrackJobStatusFail, t)

	// Get and check job status
	gotJob, _, _, err := trackClient.Get("1")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if gotJob.Status != trackclient.TrackJobStatusFail {
		t.Fatalf("unexpected job status: %s", gotJob.Status)
	}

	// Read the job output
	getStdOut, err := trackClient.GetCombinedOutput("1")
	if err != nil {
		t.Fatalf("GetStdOut: %v", err)
	}
	defer getStdOut.Close()
	stdOut, err := io.ReadAll(getStdOut)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if !strings.Contains(string(stdOut), runners.StdErrPrefix) {
		t.Fatalf("unexpected stdOut: %q", stdOut)
	}
}

func TestMockJobsCantRun(t *testing.T) {
	// Mock a server that simply can't run any jobs due to mocked runtime errors
	// In this scenario, the server should eventually deadletter the job
	// execution and send a webhook communicating it can't run
	secrets := map[string]string{}
	srv, hooks := trackserver.GetTestServerAndFakeWebhookListener(
		t, secrets, func(sc *trackserver.SrvConfig) {
			sc.MockJobCantRun = true
			sc.JobQueueSleep = time.Millisecond
			sc.UseCustomJobQueueParams = true
			sc.CustomJobQueueBaseRetryDelay = time.Millisecond
			sc.CustomJobQueueMaxRetries = 1
		})
	trackClient := trackclient.NewClient(srv.Url(), srv.TrackKey())
	payload := runnerlib.JobPayload{
		Name: "say-hi",
		Steps: []runnerlib.JobStep{
			{
				Run: "echo hi",
			},
		},
		TimeoutMilliSeconds: 5000,
	}
	err := trackClient.Put("1", payload)
	if err != nil {
		t.Fatalf("client put failed: %v", err)
	}

	// Wait until the cancel notification is received
	hooks.WaitFor("1", trackclient.TrackJobStatusCancel, t)

	// Get and check job status
	gotJob, _, _, err := trackClient.Get("1")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if gotJob.Status != trackclient.TrackJobStatusCancel {
		t.Fatalf("unexpected job status: %s", gotJob.Status)
	}
}

func TestPutJobWithSkipWebhook(t *testing.T) {
	secrets := map[string]string{}
	// Use MockJobRuns=true option to mock the actual job execution to
	// speed up this test
	srv, hooks := trackserver.GetTestServerAndFakeWebhookListener(
		t, secrets,
		func(sc *trackserver.SrvConfig) { sc.MockJobRuns = true })
	trackClient := trackclient.NewClient(srv.Url(), srv.TrackKey())
	const jobId = "mock job that sends no webhook"
	payload := runnerlib.JobPayload{}
	err := trackClient.PutSkipWebhook(jobId, payload)
	if err != nil {
		t.Fatalf("client put failed: %v", err)
	}

	// Wait for the job to run
	start := time.Now()
	for true {
		time.Sleep(50 * time.Millisecond)
		if time.Since(start) > 5*time.Second {
			t.Fatalf("waited too long for job to run")
		}
		gotJob, _, _, err := trackClient.Get(jobId)
		if err != nil {
			continue
		}
		if gotJob.Status == trackclient.TrackJobStatusSuccess {
			break
		}
	}

	// Ensure no webhooks were posted
	n := hooks.GetNotificationsCount()
	if n != 0 {
		t.Fatalf("%d notifications were sent", n)
	}
}

func TestPutBadPayload(t *testing.T) {
	srv := trackserver.GetNoWebhookTestServer(t)
	c := &http.Client{}
	req, err := http.NewRequest(http.MethodPut,
		srv.Url()+"/job/1", bytes.NewReader([]byte("invalid payload")))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("TrackKey", srv.TrackKey())
	putResp, err := c.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", putResp.StatusCode)
	}
}

func TestPutAndCancelJob(t *testing.T) {
	secrets := map[string]string{}
	srv, hooks := trackserver.GetTestServerAndFakeWebhookListener(t, secrets)
	trackClient := trackclient.NewClient(srv.Url(), srv.TrackKey())
	const job1Id = "sleep-job-30s-id"
	payload := runnerlib.JobPayload{
		Name: "sleep forever",
		Steps: []runnerlib.JobStep{
			{
				Run: "echo HELLO",
			},
			{
				Run: "sleep 30",
			},
		},
		TimeoutMilliSeconds: 60_000,
	}
	err := trackClient.Put(job1Id, payload)
	if err != nil {
		t.Fatalf("client put failed: %v", err)
	}

	// Wait for the job to start to run
	hooks.WaitFor(job1Id, trackclient.TrackJobStatusRunning, t)
	// Since there ares still some small tasks (like fetch secrets and actually
	// calling the "Run") before the job is cancellable, check via its log
	// that it is already sleeping
	isSleeping := func() bool {
		getStdOut, err := trackClient.GetCombinedOutput(job1Id)
		if err != nil {
			return false
		}
		defer getStdOut.Close()
		stdOut, err := io.ReadAll(getStdOut)
		if err != nil {
			return false
		}
		return strings.Contains(string(stdOut), "HELLO")
	}
	for !isSleeping() {
		time.Sleep(20 * time.Millisecond)
	}

	// Cancel the job
	err = trackClient.Cancel(job1Id)
	if err != nil {
		t.Fatalf("client cancel failed: %v", err)
	}
	hooks.WaitFor(job1Id, trackclient.TrackJobStatusCancel, t)

	// Get and check job status
	gotJob, _, _, err := trackClient.Get(job1Id)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if gotJob.Status != trackclient.TrackJobStatusCancel {
		t.Fatalf("unexpected job status: %s", gotJob.Status)
	}
	// Read the job output
	getStdOut, err := trackClient.GetCombinedOutput(job1Id)
	if err != nil {
		t.Fatalf("GetStdOut: %v", err)
	}
	defer getStdOut.Close()
	stdOut, err := io.ReadAll(getStdOut)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if !strings.Contains(string(stdOut), "CANCEL") {
		t.Fatalf("job output doesnt contain CANCEL: %q", stdOut)
	}

	// Try canceling the job even before putting. This will work because
	// the cancelation is best effort and will happen once the job is
	// run
	const job2Id = "pre-canceled-job"
	err = trackClient.Cancel(job2Id)
	if err != nil {
		t.Fatalf("client cancel failed: %v", err)
	}
	err = trackClient.Put(job2Id, payload)
	if err != nil {
		t.Fatalf("client put failed: %v", err)
	}
	hooks.WaitFor(job2Id, trackclient.TrackJobStatusCancel, t)
	// Check job status
	gotJob2, _, _, err := trackClient.Get(job2Id)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if gotJob2.Status != trackclient.TrackJobStatusCancel {
		t.Fatalf("unexpected job status: %s", gotJob2.Status)
	}
}

func TestPutWith_Go_Node_Bun_Images(t *testing.T) {
	secrets := map[string]string{}
	srv, hooks := trackserver.GetTestServerAndFakeWebhookListener(t, secrets)
	trackClient := trackclient.NewClient(srv.Url(), srv.TrackKey())
	goPayload := runnerlib.JobPayload{
		Name:      "show go version",
		ImageName: runnerlib.GoImage,
		Steps: []runnerlib.JobStep{
			{
				Run: "go version",
			},
		},
		TimeoutMilliSeconds: 2000,
	}
	err := trackClient.Put("go-job-id", goPayload)
	if err != nil {
		t.Fatalf("client put failed: %v", err)
	}
	bunPayload := runnerlib.JobPayload{
		Name:      "show bun version",
		ImageName: runnerlib.BunImage,
		Steps: []runnerlib.JobStep{
			{
				Run: "bun -v",
			},
		},
		TimeoutMilliSeconds: 2000,
	}
	err = trackClient.Put("bun-job-id", bunPayload)
	if err != nil {
		t.Fatalf("client put failed: %v", err)
	}
	nodePayload := runnerlib.JobPayload{
		Name:      "show node version",
		ImageName: runnerlib.Node20Image,
		Steps: []runnerlib.JobStep{
			{
				Run: "node -v",
			},
		},
		TimeoutMilliSeconds: 2000,
	}
	err = trackClient.Put("node-job-id", nodePayload)
	if err != nil {
		t.Fatalf("client put failed: %v", err)
	}

	// Wait until the notifications (running, success) are received
	hooks.WaitFor("go-job-id", trackclient.TrackJobStatusRunning, t)
	hooks.WaitFor("go-job-id", trackclient.TrackJobStatusSuccess, t)
	hooks.WaitFor("bun-job-id", trackclient.TrackJobStatusRunning, t)
	hooks.WaitFor("bun-job-id", trackclient.TrackJobStatusSuccess, t)
	hooks.WaitFor("node-job-id", trackclient.TrackJobStatusRunning, t)
	hooks.WaitFor("node-job-id", trackclient.TrackJobStatusSuccess, t)
}

func TestPutOverMaxTimeout(t *testing.T) {
	secrets := map[string]string{}
	srv, _ := trackserver.GetTestServerAndFakeWebhookListener(t, secrets)
	trackClient := trackclient.NewClient(srv.Url(), srv.TrackKey())
	payload := runnerlib.JobPayload{
		Name: "say hi with infinite timeout",
		Steps: []runnerlib.JobStep{
			{
				Run: "echo hi!",
			},
		},
		TimeoutMilliSeconds: math.MaxInt64,
	}
	err := trackClient.Put("1", payload)
	if err == nil {
		t.Fatalf("successfully put job with inlimited timeout")
	}
	if !strings.Contains(err.Error(), "timeout too big") {
		t.Fatalf("unexpected err: %s", err.Error())
	}
}

type queueCounter struct {
	successes int
	errors    int
	total     int
}

func (q *queueCounter) OnHandle(payloadType string, payload []byte, result error) {
	q.total += 1
	if result == nil {
		q.successes += 1
	} else {
		q.errors += 1
	}
}
