package cicdpublisher

import (
	"context"
	"errors"
	"fmt"
	"math"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-web/featureflags"
	"monorepo/twigg-web/job"
	"monorepo/twigg-web/services/sign"
	"monorepo/twigg-web/services/twiggtoken"
	"monorepo/twigg/client"
	"monorepo/twigg/server"
	"reflect"
	"testing"
	"time"
)

var testSigner = sign.NewSigner([]byte("ci-publisher-test-key"))

const testApiKey = "fake-api-key"

func TestNoJobsPublished(t *testing.T) {
	srv := server.NewTestServer(testApiKey, t)
	root, client, wd, clientRead := client.NewTest("owner", 1, t)
	const repoId = 0
	const repoOwnerId = 1
	serverProvider := fakeRepoService{
		srv:            srv,
		srvRepoId:      repoId,
		srvRepoOwnerId: repoOwnerId,
	}
	jobsStorage := fakeJobsStorage{}
	parser := fakeParser{}
	trackClient := fakeTrackClient{}
	timeoutProvider := fakeMaxAllowedTimeoutGetter{
		userIdToDuration: map[int64]time.Duration{repoOwnerId: time.Hour},
	}
	ciService := New(serverProvider, timeoutProvider, &jobsStorage, &parser, &trackClient,
		mockFlagsProvider{}, testSigner)

	// Create and push one commit
	wd.WriteFile("c1.txt", "c1")
	c1, _ := client.Commit(wd, "c1", &root, clientRead)
	client.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)
	// Post the jobs of that commit
	w, closeW, _ := srv.BeginWrite()
	defer closeW()
	const runNumber = 88
	putAndCheckRun := func() {
		err := ciService.PutAutoCiCdRun(
			repoId, c1.ServerL, c1.ServerV, runNumber, runnerlib.OnPush, w,
		)
		if err != nil {
			t.Fatal(err)
		}

		if len(jobsStorage.jobIds) != 0 {
			t.Fatalf("got jobs %v", jobsStorage.jobIds)
		}
		if len(jobsStorage.pipelineById) != 0 {
			t.Fatalf("got pipelines %v", jobsStorage.pipelineById)
		}
	}
	// Call twice to check idempotency
	putAndCheckRun()
	putAndCheckRun()
}

func TestSingleCiFileAtRoot(t *testing.T) {
	srv := server.NewTestServer(testApiKey, t)
	root, client, wd, clientRead := client.NewTest("owner", 1, t)
	const repoId = 0
	const repoOwnerId = 1
	serverProvider := fakeRepoService{
		srv:            srv,
		srvRepoId:      repoId,
		srvRepoOwnerId: repoOwnerId,
	}
	jobsStorage := fakeJobsStorage{}
	timeoutProvider := fakeMaxAllowedTimeoutGetter{
		userIdToDuration: map[int64]time.Duration{repoOwnerId: time.Hour},
	}
	trackClient := fakeTrackClient{}
	parser := fakeParser{
		ciFileParsingMap: map[string][]runnerlib.CiJob{
			"{sh:echo hello},{sh:echo bye}": {
				{
					Job: runnerlib.JobPayload{
						Name: "say-hello",
						Steps: []runnerlib.JobStep{
							{
								Run: "echo hello",
							},
						},
						TimeoutMilliSeconds: 1000,
					},
				},
				{
					Job: runnerlib.JobPayload{
						Name: "say-bye",
						Steps: []runnerlib.JobStep{
							{
								Run: "echo bye",
							},
						},
						TimeoutMilliSeconds: 2000,
					},
				},
			},
		},
	}
	ciService := New(serverProvider, timeoutProvider, &jobsStorage, &parser, &trackClient,
		mockFlagsProvider{}, testSigner)

	// Create a commit with a CI file at root
	wd.WriteFile(CiFilename, "{sh:echo hello},{sh:echo bye}")
	c1, _ := client.Commit(wd, "c1", &root, clientRead)
	client.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)
	// Expect only the root ci file to require ci jobs
	w, closeW, _ := srv.BeginWrite()
	defer closeW()
	const runNumber = 88
	err := ciService.PutAutoCiCdRun(
		repoId, c1.ServerL, c1.ServerV, runNumber, runnerlib.OnPush, w,
	)
	if err != nil {
		t.Fatal(err)
	}

	removeTokens(trackClient.postedJobPayloads, t)

	if !reflect.DeepEqual(
		trackClient.postedJobPayloads, []runnerlib.JobPayload{
			{
				Name: "say-hello",
				Steps: []runnerlib.JobStep{
					{
						Run: "echo hello",
						Env: map[string]string{
							runnerlib.TwiggTokenEnvVarName: "",
							runnerlib.CommitIdEnvVarName:   "c1v0",
							runnerlib.RepoIdEnvVarName:     fmt.Sprintf("%d/%d", repoId, repoId),
						},
						Dir: ".",
					},
				},
				TimeoutMilliSeconds: 1000,
				Token:               "",
			},
			{
				Name: "say-bye",
				Steps: []runnerlib.JobStep{
					{
						Run: "echo bye",
						Env: map[string]string{
							runnerlib.TwiggTokenEnvVarName: "",
							runnerlib.CommitIdEnvVarName:   "c1v0",
							runnerlib.RepoIdEnvVarName:     fmt.Sprintf("%d/%d", repoId, repoId),
						},
						Dir: ".",
					},
				},
				TimeoutMilliSeconds: 2000,
				Token:               "",
			},
		}) {
		t.Fatalf("unexpected jobs at trackClient: %v", trackClient.postedJobPayloads)
	}
	// The jobs should appear in the storage as posted
	if len(jobsStorage.jobStatus) != 2 {
		t.Fatalf("expected 2 jobs in storage")
	}
	if jobsStorage.jobStatus[0] != job.JobStatusQueued {
		t.Fatalf("first job unexpected status")
	}
	if jobsStorage.jobStatus[1] != job.JobStatusQueued {
		t.Fatalf("second job unexpected status")
	}
	if len(jobsStorage.jobNames) != 2 {
		t.Fatalf("expected 2 jobs in storage")
	}
	if jobsStorage.jobNames[0] != "say-hello" {
		t.Fatalf("first job unexpected name")
	}
	if jobsStorage.jobNames[1] != "say-bye" {
		t.Fatalf("second job unexpected name")
	}
}

func TestDeleteCiFileAndModifyOtherCiFile(t *testing.T) {
	srv := server.NewTestServer(testApiKey, t)
	root, client, wd, clientRead := client.NewTest("owner", 1, t)
	const repoId = 0
	const repoOwnerId = 1
	serverProvider := fakeRepoService{
		srv:            srv,
		srvRepoId:      repoId,
		srvRepoOwnerId: repoOwnerId,
	}
	timeoutProvider := fakeMaxAllowedTimeoutGetter{
		userIdToDuration: map[int64]time.Duration{repoOwnerId: time.Hour},
	}
	jobsStorage := fakeJobsStorage{}
	trackClient := fakeTrackClient{}
	parser := fakeParser{
		ciFileParsingMap: map[string][]runnerlib.CiJob{
			"{sh:echo hello}": {
				{
					Job: runnerlib.JobPayload{
						Name: "say-hello",
						Steps: []runnerlib.JobStep{
							{
								Run: "echo hello",
							},
						},
					},
				},
			},
		},
	}
	ciService := New(serverProvider, timeoutProvider, &jobsStorage, &parser, &trackClient,
		mockFlagsProvider{}, testSigner)

	// Create a commit with two CI files in subdirs
	wd.WriteFile("subfolder0/"+CiFilename, "{sh:echo hello}")
	wd.WriteFile("subfolder1/"+CiFilename, "{sh:echo hello}")
	c1, _ := client.Commit(wd, "c1", &root, clientRead)
	// Create another commit that deletes that file and adds some
	// file to the other dir. We expect only this CI to run.
	wd.Delete("subfolder0/" + CiFilename)
	wd.WriteFile("subfolder1/"+"a.txt", "random file")
	c2, _ := client.Commit(wd, "c2", &c1, clientRead)
	// Push both commits
	client.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)
	// When creating the jobs for c2, expect no errors and only the job at the
	// subfolder to run
	w, closeW, _ := srv.BeginWrite()
	defer closeW()
	const runNumber = 77
	err := ciService.PutAutoCiCdRun(
		repoId, c2.ServerL, c2.ServerV, runNumber, runnerlib.OnPush, w,
	)
	if err != nil {
		t.Fatal(err)
	}
	// The jobs should appear in the storage as posted
	if len(jobsStorage.jobStatus) != 1 {
		t.Fatalf("expected 1 jobs in storage, got %d", len(jobsStorage.jobStatus))
	}
	if jobsStorage.jobStatus[0] != job.JobStatusQueued {
		t.Fatalf("first job unexpected status: %s", jobsStorage.jobStatus[0])
	}
}

func TestCiOn(t *testing.T) {
	srv := server.NewTestServer(testApiKey, t)
	root, client, wd, clientRead := client.NewTest("owner", 1, t)
	const repoId = 0
	const repoOwnerId = 1
	serverProvider := fakeRepoService{
		srv:            srv,
		srvRepoId:      repoId,
		srvRepoOwnerId: repoOwnerId,
	}
	timeoutProvider := fakeMaxAllowedTimeoutGetter{
		userIdToDuration: map[int64]time.Duration{repoOwnerId: time.Hour},
	}
	jobsStorage := fakeJobsStorage{}
	trackClient := fakeTrackClient{postedJobPayloads: []runnerlib.JobPayload{}}
	parser := fakeParser{
		ciFileParsingMap: map[string][]runnerlib.CiJob{
			"{sh:echo hello}": {
				{
					On: []runnerlib.JobTrigger{runnerlib.OnSumit},
					Job: runnerlib.JobPayload{
						Name: "say-hello-on-submit",
						Steps: []runnerlib.JobStep{
							{
								Run: "echo hello",
							},
						},
						TimeoutMilliSeconds: 3000,
					},
				},
			},
		},
	}
	ciService := New(serverProvider, timeoutProvider, &jobsStorage, &parser, &trackClient,
		mockFlagsProvider{}, testSigner)

	// Create a commit with a CI file at root
	wd.WriteFile(CiFilename, "{sh:echo hello}")
	c1, _ := client.Commit(wd, "c1", &root, clientRead)
	client.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)

	// No job should be created OnPush because the CI should only run on submit
	w, closeW, _ := srv.BeginWrite()
	defer closeW()
	const runNumber = 88
	err := ciService.PutAutoCiCdRun(
		repoId, c1.ServerL, c1.ServerV, runNumber, runnerlib.OnPush, w,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(trackClient.postedJobPayloads) != 0 {
		t.Fatalf("job was posted on push")
	}
	// A job should be posted on submit
	err = ciService.PutAutoCiCdRun(
		repoId, c1.ServerL, c1.ServerV, runNumber+1, runnerlib.OnSumit, w,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(trackClient.postedJobPayloads) != 1 {
		t.Fatalf("job was not posted on submit")
	}
}

func TestTooManyJobs(t *testing.T) {
	srv := server.NewTestServer(testApiKey, t)
	root, client, wd, clientRead := client.NewTest("owner", 1, t)
	const repoId = 0
	const repoOwnerId = 1
	serverProvider := fakeRepoService{
		srv:            srv,
		srvRepoId:      repoId,
		srvRepoOwnerId: repoOwnerId,
	}
	jobsStorage := fakeJobsStorage{}
	timeoutProvider := fakeMaxAllowedTimeoutGetter{
		userIdToDuration: map[int64]time.Duration{repoOwnerId: time.Hour},
	}
	trackClient := fakeTrackClient{postedJobPayloads: []runnerlib.JobPayload{}}
	parser := fakeParser{
		ciFileParsingMap: map[string][]runnerlib.CiJob{
			"{fake CI file}": {
				{
					Job: runnerlib.JobPayload{
						Name: "say-hello-1",
						Steps: []runnerlib.JobStep{
							{
								Run: "echo hello",
							},
						},
						TimeoutMilliSeconds: 1000,
					},
				},
				{
					Job: runnerlib.JobPayload{
						Name: "say-hello-2",
						Steps: []runnerlib.JobStep{
							{
								Run: "echo hello",
							},
						},
						TimeoutMilliSeconds: 1000,
					},
				},
			},
		},
	}
	ciService := New(serverProvider, timeoutProvider, &jobsStorage, &parser, &trackClient,
		mockFlagsProvider{}, testSigner)

	// Create a commit with a CI file at root that would publish 2 jobs, which
	// is higher than the max jobs
	originalMaxJobsPerCommit := MaxJobsPerCommit
	MaxJobsPerCommit = 1
	t.Cleanup(func() { MaxJobsPerCommit = originalMaxJobsPerCommit })
	wd.WriteFile(CiFilename, "{fake CI file}")
	c1, _ := client.Commit(wd, "c1", &root, clientRead)
	client.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)

	// No job should be created because its over the MaxJobsPerCommit
	w, closeW, _ := srv.BeginWrite()
	defer closeW()
	const runNumber = 88
	err := ciService.PutAutoCiCdRun(
		repoId, c1.ServerL, c1.ServerV, runNumber, runnerlib.OnPush, w,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(trackClient.postedJobPayloads) != 0 {
		t.Fatalf("job was posted without quota")
	}
}

func TestTooLargeTimeout(t *testing.T) {
	srv := server.NewTestServer(testApiKey, t)
	root, client, wd, clientRead := client.NewTest("owner", 1, t)
	const repoId = 0
	const repoOwnerId = 1
	serverProvider := fakeRepoService{
		srv:            srv,
		srvRepoId:      repoId,
		srvRepoOwnerId: repoOwnerId,
	}
	jobsStorage := fakeJobsStorage{}
	timeoutProvider := fakeMaxAllowedTimeoutGetter{
		userIdToDuration: map[int64]time.Duration{repoOwnerId: 500 * time.Millisecond},
	}
	trackClient := fakeTrackClient{postedJobPayloads: []runnerlib.JobPayload{}}
	parser := fakeParser{
		ciFileParsingMap: map[string][]runnerlib.CiJob{
			"{fake CI file with 500ms timeout}": {
				{
					Job: runnerlib.JobPayload{
						Name: "say-hello-1",
						Steps: []runnerlib.JobStep{
							{
								Run: "echo hello",
							},
						},
						TimeoutMilliSeconds: 400,
					},
				},
				{
					Job: runnerlib.JobPayload{
						Name: "say-hello-2",
						Steps: []runnerlib.JobStep{
							{
								Run: "echo hello",
							},
						},
						TimeoutMilliSeconds: 101,
					},
				},
			},
		},
	}
	ciService := New(serverProvider, timeoutProvider, &jobsStorage, &parser, &trackClient,
		mockFlagsProvider{}, testSigner)
	wd.WriteFile(CiFilename, "{fake CI file with 500ms timeout}")
	c1, _ := client.Commit(wd, "c1", &root, clientRead)
	client.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)

	// No job should be posted because the timeout is
	// over what's allowed by timeoutProvider
	w, closeW, _ := srv.BeginWrite()
	defer closeW()
	const runNumber = 88
	err := ciService.PutAutoCiCdRun(
		repoId, c1.ServerL, c1.ServerV, runNumber, runnerlib.OnPush, w,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(trackClient.postedJobPayloads) != 0 {
		t.Fatalf("job was posted without quota")
	}

	// The jobs should appear in the storage
	if len(jobsStorage.jobStatus) != 1 {
		t.Fatalf("expected 1 jobs in storage, got %d", len(jobsStorage.jobStatus))
	}
	if jobsStorage.jobStatus[0] != job.JobStatusExceedsPlanLimits {
		t.Fatalf("first job unexpected status: %s", jobsStorage.jobStatus[0])
	}
}

func TestDirOfFileInSubfolder(t *testing.T) {
	srv := server.NewTestServer(testApiKey, t)
	root, client, wd, clientRead := client.NewTest("owner", 1, t)
	const repoId = 0
	const repoOwnerId = 1
	serverProvider := fakeRepoService{
		srv:            srv,
		srvRepoId:      repoId,
		srvRepoOwnerId: repoOwnerId,
	}
	const fakeCiFile = "{sh:echo hello}"
	jobsStorage := fakeJobsStorage{}
	timeoutProvider := fakeMaxAllowedTimeoutGetter{
		userIdToDuration: map[int64]time.Duration{repoOwnerId: time.Hour},
	}
	trackClient := fakeTrackClient{postedJobPayloads: []runnerlib.JobPayload{}}
	parser := fakeParser{
		ciFileParsingMap: map[string][]runnerlib.CiJob{
			fakeCiFile: {
				{
					Job: runnerlib.JobPayload{
						Name: "say-hello-from-this-dir",
						Steps: []runnerlib.JobStep{
							{
								Run: "echo hello",
							},
						},
						TimeoutMilliSeconds: 500,
					},
				},
				{
					Job: runnerlib.JobPayload{
						Name: "say-hello-from-another-dir",
						Steps: []runnerlib.JobStep{
							{
								Run: "echo hello",
								Dir: "x/y/z",
							},
						},
						TimeoutMilliSeconds: 500,
					},
				},
			},
		},
	}
	ciService := New(serverProvider, timeoutProvider, &jobsStorage, &parser, &trackClient,
		mockFlagsProvider{}, testSigner)

	// Create a commit with a CI file in a subfolder
	wd.WriteFile("a/b/"+CiFilename, fakeCiFile)
	c1, _ := client.Commit(wd, "c1", &root, clientRead)
	client.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)
	w, closeW, _ := srv.BeginWrite()
	defer closeW()
	const runNumber = 88
	err := ciService.PutAutoCiCdRun(
		repoId, c1.ServerL, c1.ServerV, runNumber, runnerlib.OnPush, w,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(trackClient.postedJobPayloads) != 2 {
		t.Fatalf("got %d jobs", len(trackClient.postedJobPayloads))
	}
	if len(trackClient.postedJobPayloads[0].Steps) != 1 {
		t.Fatalf("got %d steps", len(trackClient.postedJobPayloads[0].Steps))
	}
	if trackClient.postedJobPayloads[0].Steps[0].Dir != "a/b" {
		t.Fatalf("unexpected dir: %s", trackClient.postedJobPayloads[0].Steps[0].Dir)
	}
	if len(trackClient.postedJobPayloads[1].Steps) != 1 {
		t.Fatalf("got %d steps", len(trackClient.postedJobPayloads[1].Steps))
	}
	if trackClient.postedJobPayloads[1].Steps[0].Dir != "x/y/z" {
		t.Fatalf("unexpected dir: %s", trackClient.postedJobPayloads[1].Steps[0].Dir)
	}
}

func TestSingleCdFileAtRoot(t *testing.T) {
	srv := server.NewTestServer(testApiKey, t)
	root, client, wd, clientRead := client.NewTest("owner", 1, t)
	const repoId = 0
	const repoOwnerId = 1
	serverProvider := fakeRepoService{
		srv:            srv,
		srvRepoId:      repoId,
		srvRepoOwnerId: repoOwnerId,
	}
	jobsStorage := fakeJobsStorage{}
	timeoutProvider := fakeMaxAllowedTimeoutGetter{
		userIdToDuration: map[int64]time.Duration{repoOwnerId: time.Hour},
	}
	trackClient := fakeTrackClient{}
	const fakeCdFile = "{fake cd file}"
	parser := fakeParser{
		cdFileParsingMap: map[string][]runnerlib.CdJob{
			fakeCdFile: {
				runnerlib.CdJob{
					Name: "CD Pipeline 0",
					On:   []runnerlib.JobTrigger{runnerlib.OnSumit, runnerlib.OnManual},
					Stages: []runnerlib.CdJobPayload{
						{
							CanAutoStart: true,
							JobPayload: runnerlib.JobPayload{
								Name: "CD Pipeline 0 - Stage 0 (autostart)",
								Steps: []runnerlib.JobStep{
									{Run: "echo hi pipeline 0 stage 0"},
									{Run: "echo bye pipeline 0 stage 0"},
								},
								TimeoutMilliSeconds: 1000,
							},
						},
						{
							CanAutoStart: true,
							JobPayload: runnerlib.JobPayload{
								Name: "CD Pipeline 0 - Stage 1 (autostart)",
								Steps: []runnerlib.JobStep{
									{Run: "echo hi pipeline 0 stage 1"},
									{Run: "echo bye pipeline 0 stage 1"},
								},
								TimeoutMilliSeconds: 1000,
							},
						},
						{
							CanAutoStart: false,
							JobPayload: runnerlib.JobPayload{
								Name: "CD Pipeline 0 - Stage 2 (non-autostart)",
								Steps: []runnerlib.JobStep{
									{Run: "echo hi pipeline 0 stage 2"},
									{Run: "echo bye pipeline 0 stage 2"},
								},
								TimeoutMilliSeconds: 1000,
							},
						},
					},
				},
				runnerlib.CdJob{
					Name: "CD Pipeline 1",
					On:   []runnerlib.JobTrigger{runnerlib.OnSumit, runnerlib.OnManual},
					Stages: []runnerlib.CdJobPayload{
						{
							CanAutoStart: false,
							JobPayload: runnerlib.JobPayload{
								Name: "CD Pipeline 1 - Stage 0 (not-autostart)",
								Steps: []runnerlib.JobStep{
									{Run: "echo hi pipeline 1 stage 0"},
								},
								TimeoutMilliSeconds: 1000,
							},
						},
					},
				},
			},
		},
	}
	ciService := New(serverProvider, timeoutProvider, &jobsStorage, &parser, &trackClient,
		mockFlagsProvider{}, testSigner)

	// Create a commit with a CD file at root and push it
	wd.WriteFile(CdFilename, fakeCdFile)
	c1, _ := client.Commit(wd, "c1", &root, clientRead)
	client.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)
	w, closeW, _ := srv.BeginWrite()
	defer closeW()
	// Check that the cd jobs will be created at the storage and posted
	const runNumber = 88
	err := ciService.PutAutoCiCdRun(
		repoId, c1.ServerL, c1.ServerV,
		runNumber, runnerlib.OnSumit, w,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Check if trackclient posted the jobs
	trackClient.checkNumPostedPayloads(1, t)
	trackClient.checkHasPayloadDisconsideringTokens(runnerlib.JobPayload{
		Name: "CD Pipeline 0 - Stage 0 (autostart)",
		Steps: []runnerlib.JobStep{
			{
				Run: "echo hi pipeline 0 stage 0",
				Env: map[string]string{
					runnerlib.TwiggTokenEnvVarName: "",
					runnerlib.CommitIdEnvVarName:   "c1v0",
					runnerlib.RepoIdEnvVarName:     fmt.Sprintf("%d/%d", repoId, repoId),
				},
				Dir: ".",
			},
			{
				Run: "echo bye pipeline 0 stage 0",
				Env: map[string]string{
					runnerlib.TwiggTokenEnvVarName: "",
					runnerlib.CommitIdEnvVarName:   "c1v0",
					runnerlib.RepoIdEnvVarName:     fmt.Sprintf("%d/%d", repoId, repoId),
				},
				Dir: ".",
			},
		},
		TimeoutMilliSeconds: 1000,
		Token:               "",
	}, t)

	// Refs for both cd jobs should be created
	jobsStorage.checkRefExists(repoId, CdFilename, "CD Pipeline 0", t)
	jobsStorage.checkRefExists(repoId, CdFilename, "CD Pipeline 1", t)
	// Both pipelines should be created and be running
	jobsStorage.checkNumOfPipelines(2, t)
	jobsStorage.checkPipelineStatus(repoId,
		c1.ServerL, c1.ServerV, CdFilename,
		"CD Pipeline 0",
		runNumber, job.PipelineStatusRunning, t)
	jobsStorage.checkPipelineStatus(repoId,
		c1.ServerL, c1.ServerV, CdFilename,
		"CD Pipeline 1",
		runNumber, job.PipelineStatusRunning, t)
	pipeline0Id := job.PipelineId(repoId, c1.L, c1.Version, CdFilename,
		"CD Pipeline 0", runNumber)
	pipeline1Id := job.PipelineId(repoId, c1.L, c1.Version, CdFilename,
		"CD Pipeline 1", runNumber)
	// All stages of each pipelines must have been created
	jobsStorage.checkPipelineStages(t, pipeline0Id, 3)
	jobsStorage.checkPipelineStages(t, pipeline1Id, 1)
	// Check the state of each stage
	jobsStorage.checkPipelineStageStatus(t, pipeline0Id, 0, job.JobStatusQueued)
	jobsStorage.checkPipelineStageStatus(t, pipeline0Id, 1, job.JobStatusWaiting)
	jobsStorage.checkPipelineStageStatus(t, pipeline0Id, 2, job.JobStatusWaiting)
	jobsStorage.checkPipelineStageStatus(t, pipeline1Id, 0, job.JobStatusWaitingManualStart)

	// Resume pipeline 0 stage 1. Note that this should really only be done
	// if some external caller sets the stage 0 to be completed, but this
	// method does not check that
	resumeStage1 := func() {
		err = ciService.PutResumePipelineWaitingStage(pipeline0Id, 1, w)
		if err != nil {
			t.Fatal(err)
		}
		// Pipeline stage should be updated to queued
		jobsStorage.checkPipelineStageStatus(t, pipeline0Id, 1, job.JobStatusQueued)
		// The new payload should be posted
		trackClient.checkNumPostedPayloads(2, t)
		trackClient.checkHasPayloadDisconsideringTokens(runnerlib.JobPayload{
			Name: "CD Pipeline 0 - Stage 1 (autostart)",
			Steps: []runnerlib.JobStep{
				{
					Run: "echo hi pipeline 0 stage 1",
					Env: map[string]string{
						runnerlib.TwiggTokenEnvVarName: "",
						runnerlib.CommitIdEnvVarName:   "c1v0",
						runnerlib.RepoIdEnvVarName:     fmt.Sprintf("%d/%d", repoId, repoId),
					},
					Dir: ".",
				},
				{
					Run: "echo bye pipeline 0 stage 1",
					Env: map[string]string{
						runnerlib.TwiggTokenEnvVarName: "",
						runnerlib.CommitIdEnvVarName:   "c1v0",
						runnerlib.RepoIdEnvVarName:     fmt.Sprintf("%d/%d", repoId, repoId),
					},
					Dir: ".",
				},
			},
			TimeoutMilliSeconds: 1000,
			Token:               "",
		}, t)
	}
	resumeStage1()
	resumeStage1() // Call twice to check idempotency

	// Resume pipeline 0 stage 2. Again; note that this should really only be done
	// if some external caller sets the stage 1 to be completed, but this
	// method does not check that
	resumeStage2 := func() {
		err = ciService.PutResumePipelineWaitingStage(pipeline0Id, 2, w)
		if err != nil {
			t.Fatal(err)
		}
		// Pipeline stage should be updated to waiting manual bc its not auto
		jobsStorage.checkPipelineStageStatus(t, pipeline0Id, 2, job.JobStatusWaitingManualStart)
		// No payload should be posted bc its waiting a manual start
		trackClient.checkNumPostedPayloads(2, t)
	}
	resumeStage2()
	resumeStage2() // Call twice to check idempotency

	// Manually resumte stage 2
	const resumerUserId = 9871
	isCantResumeErr, err := ciService.ManualResumePipeline(pipeline0Id, 2, resumerUserId, w)
	if err != nil || isCantResumeErr {
		t.Fatal(err)
	}
	// Pipeline stage should be updated to queued
	jobsStorage.checkPipelineStageStatus(t, pipeline0Id, 2, job.JobStatusQueued)
	jobsStorage.checkPipelineStageResumedByUser(t, pipeline0Id, 2, resumerUserId)
	// Check the trackclient for the payload
	trackClient.checkNumPostedPayloads(3, t)
	trackClient.checkHasPayloadDisconsideringTokens(runnerlib.JobPayload{
		Name: "CD Pipeline 0 - Stage 2 (non-autostart)",
		Steps: []runnerlib.JobStep{
			{
				Run: "echo hi pipeline 0 stage 2",
				Env: map[string]string{
					runnerlib.TwiggTokenEnvVarName: "",
					runnerlib.CommitIdEnvVarName:   "c1v0",
					runnerlib.RepoIdEnvVarName:     fmt.Sprintf("%d/%d", repoId, repoId),
				},
				Dir: ".",
			},
			{
				Run: "echo bye pipeline 0 stage 2",
				Env: map[string]string{
					runnerlib.TwiggTokenEnvVarName: "",
					runnerlib.CommitIdEnvVarName:   "c1v0",
					runnerlib.RepoIdEnvVarName:     fmt.Sprintf("%d/%d", repoId, repoId),
				},
				Dir: ".",
			},
		},
		TimeoutMilliSeconds: 1000,
	}, t)
	// ManualResumePipeline is not idempotent
	isCantResumeErr, err = ciService.ManualResumePipeline(pipeline0Id, 2, resumerUserId, w)
	if err == nil || !isCantResumeErr {
		t.Fatal("got no cantResumeErr for retrying stage")
	}

	// Launch another run of the pipeline
	const userId = 763
	err = ciService.ManuallyLaunchCd(
		repoId, c1.ServerL, c1.ServerV,
		CdFilename, "CD Pipeline 0", userId, w,
	)
	if err != nil {
		t.Fatal(err)
	}
	pipeline0SecondRunId := job.PipelineId(repoId, c1.L, c1.Version, CdFilename,
		"CD Pipeline 0", runNumber+1)
	// Expect a third pipeline to exist now
	jobsStorage.checkNumOfPipelines(3, t)
	jobsStorage.checkPipelineStatus(repoId,
		c1.ServerL, c1.ServerV, CdFilename,
		"CD Pipeline 0",
		runNumber+1, job.PipelineStatusRunning, t)
	jobsStorage.checkPipelineUserId(repoId,
		c1.ServerL, c1.ServerV, CdFilename,
		"CD Pipeline 0",
		runNumber+1, userId, t)
	// All stages of each pipelines must have been created
	jobsStorage.checkPipelineStages(t, pipeline0SecondRunId, 3)
	jobsStorage.checkPipelineStageStatus(t, pipeline0SecondRunId, 0, job.JobStatusQueued)
	jobsStorage.checkPipelineStageStatus(t, pipeline0SecondRunId, 1, job.JobStatusWaiting)
	jobsStorage.checkPipelineStageStatus(t, pipeline0SecondRunId, 2, job.JobStatusWaiting)
}

func TestPutAutoCiCdRun_OnlyOnSubmitsArePosted(t *testing.T) {
	srv := server.NewTestServer(testApiKey, t)
	root, client, wd, clientRead := client.NewTest("owner", 1, t)
	const repoId = 0
	const repoOwnerId = 1
	serverProvider := fakeRepoService{
		srv:            srv,
		srvRepoId:      repoId,
		srvRepoOwnerId: repoOwnerId,
	}
	jobsStorage := fakeJobsStorage{}
	timeoutProvider := fakeMaxAllowedTimeoutGetter{
		userIdToDuration: map[int64]time.Duration{repoOwnerId: time.Hour},
	}
	trackClient := fakeTrackClient{}
	const fakeCdFile = "{fake cd file}"
	parser := fakeParser{
		cdFileParsingMap: map[string][]runnerlib.CdJob{
			fakeCdFile: {
				runnerlib.CdJob{
					Name: "CD Pipeline 0 - (runs on submit)",
					On:   []runnerlib.JobTrigger{runnerlib.OnSumit},
					Stages: []runnerlib.CdJobPayload{
						{
							CanAutoStart: true,
							JobPayload: runnerlib.JobPayload{
								Name: "CD Pipeline 0 - Stage 0 (autostart)",
								Steps: []runnerlib.JobStep{
									{Run: "echo hi"},
								},
								TimeoutMilliSeconds: 1000,
							},
						},
					},
				},
				runnerlib.CdJob{
					Name: "CD Pipeline 1 - (runs on manual)",
					On:   []runnerlib.JobTrigger{runnerlib.OnManual},
					Stages: []runnerlib.CdJobPayload{
						{
							CanAutoStart: true,
							JobPayload: runnerlib.JobPayload{
								Name: "CD Pipeline 1 - Stage 0 (autostart)",
								Steps: []runnerlib.JobStep{
									{Run: "echo hi"},
								},
								TimeoutMilliSeconds: 1000,
							},
						},
					},
				},
			},
		},
	}
	ciService := New(serverProvider, timeoutProvider, &jobsStorage, &parser, &trackClient,
		mockFlagsProvider{}, testSigner)
	// Create a commit with a CD file at root and push it
	wd.WriteFile(CdFilename, fakeCdFile)
	c1, _ := client.Commit(wd, "c1", &root, clientRead)
	client.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)
	w, closeW, _ := srv.BeginWrite()
	defer closeW()
	// Create the auto CI/CD jobs
	const runNumber = 54
	err := ciService.PutAutoCiCdRun(
		repoId, c1.ServerL, c1.ServerV,
		runNumber, runnerlib.OnSumit, w,
	)
	if err != nil {
		t.Fatal(err)
	}
	// Only one payload should be posted - bc pipeline1 only triggers manually
	trackClient.checkNumPostedPayloads(1, t)
	jobsStorage.checkNumOfPipelines(1, t)
}

func TestModifyOrDeleteCdFile(t *testing.T) {
	srv := server.NewTestServer(testApiKey, t)
	root, client, wd, clientRead := client.NewTest("owner", 1, t)
	const repoId = 0
	const repoOwnerId = 1
	serverProvider := fakeRepoService{
		srv:            srv,
		srvRepoId:      repoId,
		srvRepoOwnerId: repoOwnerId,
	}
	jobsStorage := fakeJobsStorage{}
	timeoutProvider := fakeMaxAllowedTimeoutGetter{
		userIdToDuration: map[int64]time.Duration{repoOwnerId: time.Hour},
	}
	trackClient := fakeTrackClient{}
	const fakeCdFileWithPipelineZeroAndOne = "{fake cd file 0}"
	const fakeCdFileWithPipelineZero = "{fake cd file 1}"
	parser := fakeParser{
		cdFileParsingMap: map[string][]runnerlib.CdJob{
			fakeCdFileWithPipelineZeroAndOne: {
				runnerlib.CdJob{
					Name: "CD Pipeline 0",
					On:   []runnerlib.JobTrigger{runnerlib.OnSumit},
					Stages: []runnerlib.CdJobPayload{
						{
							CanAutoStart: true,
							JobPayload: runnerlib.JobPayload{
								Name: "CD Pipeline 0 - Stage 0 (autostart)",
								Steps: []runnerlib.JobStep{
									{Run: "echo hi pipeline 0 stage 0"},
								},
								TimeoutMilliSeconds: 1000,
							},
						},
					},
				},
				runnerlib.CdJob{
					Name: "CD Pipeline 1",
					On:   []runnerlib.JobTrigger{runnerlib.OnSumit},
					Stages: []runnerlib.CdJobPayload{
						{
							CanAutoStart: false,
							JobPayload: runnerlib.JobPayload{
								Name: "CD Pipeline 1 - Stage 0 (not-autostart)",
								Steps: []runnerlib.JobStep{
									{Run: "echo hi pipeline 1 stage 0"},
								},
								TimeoutMilliSeconds: 1000,
							},
						},
					},
				},
			},
			fakeCdFileWithPipelineZero: {
				runnerlib.CdJob{
					Name: "CD Pipeline 0",
					On:   []runnerlib.JobTrigger{runnerlib.OnSumit},
					Stages: []runnerlib.CdJobPayload{
						{
							CanAutoStart: true,
							JobPayload: runnerlib.JobPayload{
								Name: "CD Pipeline 0 - Stage 0 (autostart)",
								Steps: []runnerlib.JobStep{
									{Run: "echo hi pipeline 0 stage 0"},
								},
								TimeoutMilliSeconds: 1000,
							},
						},
					},
				},
			},
		},
	}
	ciService := New(serverProvider, timeoutProvider, &jobsStorage, &parser, &trackClient,
		mockFlagsProvider{}, testSigner)

	// Create a commit with a CD file at root and push it
	wd.WriteFile(CdFilename, fakeCdFileWithPipelineZeroAndOne)
	c1, _ := client.Commit(wd, "c1", &root, clientRead)
	client.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)
	putTx0, closePutTx0, commitPutTx0 := srv.BeginWrite()
	defer closePutTx0()
	const randomRunNumber = 23
	err := ciService.PutAutoCiCdRun(
		repoId, c1.ServerL, c1.ServerV, randomRunNumber, runnerlib.OnSumit, putTx0,
	)
	if err != nil {
		t.Fatal(err)
	}
	err = commitPutTx0()
	if err != nil {
		t.Fatal(err)
	}
	closePutTx0()
	// Refs for both cd refs should exist
	jobsStorage.checkRefExists(repoId, CdFilename, "CD Pipeline 0", t)
	jobsStorage.checkRefExists(repoId, CdFilename, "CD Pipeline 1", t)

	// Create c2 on top of c1 that modifies the file
	// so that the pipeline 1 is deleted.
	// After a CiCd run, Pipeline1 should be archived.
	wd.WriteFile(CdFilename, fakeCdFileWithPipelineZero)
	c2, _ := client.Commit(wd, "c2", &c1, clientRead)
	client.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)
	putTx1, closePutTx1, commitPutTx1 := srv.BeginWrite()
	defer closePutTx1()
	err = ciService.PutAutoCiCdRun(
		repoId, c2.ServerL, c2.ServerV, randomRunNumber, runnerlib.OnSumit, putTx1,
	)
	if err != nil {
		t.Fatal(err)
	}
	jobsStorage.checkRefExists(repoId, CdFilename, "CD Pipeline 0", t)
	jobsStorage.checkRefIsArchived(repoId, CdFilename, "CD Pipeline 1", t)
	err = commitPutTx1()
	if err != nil {
		t.Fatal(err)
	}
	closePutTx1()

	// Create c3 on top of c2 that deletes the file and run a CiCd run.
	// The refs should be archived.
	wd.Delete(CdFilename)
	c3, _ := client.Commit(wd, "c3", &c2, clientRead)
	client.Push(&c3, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)
	putTx2, closePutTx2, _ := srv.BeginWrite()
	defer closePutTx2()
	err = ciService.PutAutoCiCdRun(
		repoId, c3.ServerL, c3.ServerV, randomRunNumber, runnerlib.OnSumit, putTx2,
	)
	if err != nil {
		t.Fatal(err)
	}
	jobsStorage.checkRefIsArchived(repoId, CdFilename, "CD Pipeline 0", t)
	jobsStorage.checkRefIsArchived(repoId, CdFilename, "CD Pipeline 1", t)

}

func TestManuallyLaunchCd(t *testing.T) {
	srv := server.NewTestServer(testApiKey, t)
	root, client, wd, clientRead := client.NewTest("owner", 1, t)
	const repoId = 0
	const repoOwnerId = 1
	serverProvider := fakeRepoService{
		srv:            srv,
		srvRepoId:      repoId,
		srvRepoOwnerId: repoOwnerId,
	}
	jobsStorage := fakeJobsStorage{}
	timeoutProvider := fakeMaxAllowedTimeoutGetter{
		userIdToDuration: map[int64]time.Duration{repoOwnerId: time.Hour},
	}
	trackClient := fakeTrackClient{}
	const fakeCdFile = "{fake cd file}"
	parser := fakeParser{
		cdFileParsingMap: map[string][]runnerlib.CdJob{
			fakeCdFile: {
				runnerlib.CdJob{
					Name: "CD Pipeline 0",
					On:   []runnerlib.JobTrigger{runnerlib.OnManual},
					Stages: []runnerlib.CdJobPayload{
						{
							CanAutoStart: true,
							JobPayload: runnerlib.JobPayload{
								Name: "CD Pipeline 0 - Stage 0 (autostart)",
								Steps: []runnerlib.JobStep{
									{Run: "echo hi pipeline 0 stage 0"},
								},
								TimeoutMilliSeconds: 1000,
							},
						},
						{
							CanAutoStart: true,
							JobPayload: runnerlib.JobPayload{
								Name: "CD Pipeline 0 - Stage 1 (autostart)",
								Steps: []runnerlib.JobStep{
									{Run: "echo hi pipeline 0 stage 1"},
								},
								TimeoutMilliSeconds: 1000,
							},
						},
					},
				},
			},
		},
	}
	ciService := New(serverProvider, timeoutProvider, &jobsStorage, &parser, &trackClient,
		mockFlagsProvider{}, testSigner)

	// Create a commit with a CD file at root and push it
	wd.WriteFile(CdFilename, fakeCdFile)
	c1, _ := client.Commit(wd, "c1", &root, clientRead)
	client.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)
	w, closeW, _ := srv.BeginWrite()
	defer closeW()
	const userId = 96912
	// Launch a CD of the pipeline 0
	err := ciService.ManuallyLaunchCd(
		repoId, c1.ServerL, c1.ServerV,
		CdFilename, "CD Pipeline 0", userId, w,
	)
	if err != nil {
		t.Fatal(err)
	}
	// Check if trackclient posted the job
	trackClient.checkNumPostedPayloads(1, t)
	trackClient.checkHasPayloadDisconsideringTokens(runnerlib.JobPayload{
		Name: "CD Pipeline 0 - Stage 0 (autostart)",
		Steps: []runnerlib.JobStep{
			{
				Run: "echo hi pipeline 0 stage 0",
				Env: map[string]string{
					runnerlib.TwiggTokenEnvVarName: "",
					runnerlib.CommitIdEnvVarName:   "c1v0",
					runnerlib.RepoIdEnvVarName:     fmt.Sprintf("%d/%d", repoId, repoId),
				},
				Dir: ".",
			},
		},
		TimeoutMilliSeconds: 1000,
		Token:               "",
	}, t)

	// A ref should be created
	jobsStorage.checkRefExists(repoId, CdFilename, "CD Pipeline 0", t)
	// Pipeline should have the proper author
	jobsStorage.checkPipelineUserId(repoId,
		c1.ServerL, c1.ServerV, CdFilename,
		"CD Pipeline 0",
		/*runNumber*/ 0, userId, t)
	// There should be one pipeline
	jobsStorage.checkNumOfPipelines(1, t)
	// The runNumber should be zero and the pipeline should be running
	jobsStorage.checkPipelineStatus(repoId,
		c1.ServerL, c1.ServerV, CdFilename,
		"CD Pipeline 0",
		/*runNumber*/ 0, job.PipelineStatusRunning, t)
	// All stages of each pipelines must have been created
	pipeline0Run0Id := job.PipelineId(repoId, c1.L, c1.Version, CdFilename,
		"CD Pipeline 0",
		/*runNumber*/ 0)
	jobsStorage.checkPipelineStages(t, pipeline0Run0Id, 2)
	// Check the state of each stage
	jobsStorage.checkPipelineStageStatus(t, pipeline0Run0Id, 0, job.JobStatusQueued)
	jobsStorage.checkPipelineStageStatus(t, pipeline0Run0Id, 1, job.JobStatusWaiting)
}

// Provides the server for a repository
type fakeRepoService struct {
	srv            server.TestServer
	srvRepoId      uint64
	srvRepoOwnerId int64
}

func (sp fakeRepoService) GetRepoOwnerId(rl context.Context, repoId uint64) (int64, error) {
	if sp.srvRepoId != repoId {
		return 0, errors.New("not found")
	}
	return sp.srvRepoOwnerId, nil
}
func (sp fakeRepoService) GetServerByRepoId(rl context.Context, repoId uint64) (server.Server, error) {
	if sp.srvRepoId != repoId {
		return nil, errors.New("not found")
	}
	return sp.srv.GetServer(), nil
}
func (sp fakeRepoService) GetServerRead(rl context.Context) server.Read {
	return sp.srv.BindR(rl)
}

type fakeJobsStorage struct {
	publishedCiCdRuns       map[string]bool
	jobIds                  []string
	jobStatus               []job.JobStatus
	jobNames                []string
	nonArchivedPipelineRefs map[job.PipelineRef]bool
	archivedPipelineRefs    map[job.PipelineRef]bool
	pipelineById            map[string]job.Pipeline
	pipelineStagesById      map[string][]job.PipelineStage
}

func (j *fakeJobsStorage) CiCdRunWasPublished(tx context.Context,
	repoId uint64, commit uint64, commitV uint64, runNumber int64) (bool, error) {
	if j.publishedCiCdRuns == nil {
		j.publishedCiCdRuns = map[string]bool{}
	}
	_, ok := j.publishedCiCdRuns[fmt.Sprintf("%d.%d.%d.%d", repoId, commit, commitV, runNumber)]
	return ok, nil
}
func (j *fakeJobsStorage) SetCiCdToPublished(tx context.Context,
	repoId uint64, commit uint64, commitV uint64, runNumber int64) error {
	if j.publishedCiCdRuns == nil {
		j.publishedCiCdRuns = map[string]bool{}
	}
	j.publishedCiCdRuns[fmt.Sprintf("%d.%d.%d.%d", repoId, commit, commitV, runNumber)] = true
	return nil
}

func (j *fakeJobsStorage) CreateNewJob(wl context.Context,
	repoId uint64, commit uint64, commitV uint64,
	filePath string, jobName string, runNumber int64, st job.JobStatus) (job.Job, error) {
	id := job.JobId(repoId, commit, commitV, filePath, jobName, runNumber)
	for _, jId := range j.jobIds {
		if jId == id {
			return job.Job{}, errors.New("id already used")
		}
	}
	j.jobIds = append(j.jobIds, id)
	j.jobStatus = append(j.jobStatus, st)
	j.jobNames = append(j.jobNames, jobName)
	return job.Job{
		RepoId:        repoId,
		Commit:        commit,
		CommitVersion: commitV,
		Path:          filePath,
		Name:          jobName,
		RunNumber:     runNumber,
		Status:        st,
	}, nil
}

func (j *fakeJobsStorage) PutPipelineRef(tx context.Context,
	repoId uint64, filePath string, jobName string) (job.PipelineRef, error) {
	if j.nonArchivedPipelineRefs == nil {
		j.nonArchivedPipelineRefs = map[job.PipelineRef]bool{}
	}
	if j.archivedPipelineRefs == nil {
		j.archivedPipelineRefs = map[job.PipelineRef]bool{}
	}
	ref := job.PipelineRef{
		RepoId: repoId,
		Path:   filePath,
		Name:   jobName,
	}
	_, isArchived := j.archivedPipelineRefs[ref]
	if isArchived {
		delete(j.archivedPipelineRefs, ref)
	}
	j.nonArchivedPipelineRefs[ref] = true
	return ref, nil
}
func (j *fakeJobsStorage) ArchivePipelineRefIfExists(tx context.Context,
	repoId uint64, filePath string, jobName string) error {
	if j.nonArchivedPipelineRefs == nil {
		j.nonArchivedPipelineRefs = map[job.PipelineRef]bool{}
	}
	if j.archivedPipelineRefs == nil {
		j.archivedPipelineRefs = map[job.PipelineRef]bool{}
	}
	ref := job.PipelineRef{
		RepoId: repoId,
		Path:   filePath,
		Name:   jobName,
	}
	_, isNonArchived := j.nonArchivedPipelineRefs[ref]
	if isNonArchived {
		delete(j.nonArchivedPipelineRefs, ref)
	}
	j.archivedPipelineRefs[ref] = true
	return nil
}
func (j *fakeJobsStorage) checkRefExists(repoId uint64, filePath string, jobName string, t *testing.T) {
	ref := job.PipelineRef{
		RepoId: repoId,
		Path:   filePath,
		Name:   jobName,
	}
	_, found := j.nonArchivedPipelineRefs[ref]
	if found {
		return
	}
	t.Fatalf("ref repoId=%d path=%s name=%s not found", repoId, filePath, jobName)
}
func (j *fakeJobsStorage) checkRefIsArchived(repoId uint64, filePath string, jobName string, t *testing.T) {
	ref := job.PipelineRef{
		RepoId: repoId,
		Path:   filePath,
		Name:   jobName,
	}
	_, found := j.archivedPipelineRefs[ref]
	if found {
		return
	}
	t.Fatalf("ref repoId=%d path=%s name=%s not archived", repoId, filePath, jobName)
}

func (j *fakeJobsStorage) checkNumOfPipelines(expected int, t *testing.T) {
	if len(j.pipelineById) != expected {
		t.Fatalf("expected %d pipelines got %d", expected, len(j.pipelineById))
	}
}

func (j *fakeJobsStorage) checkPipelineUserId(repoId uint64, commit, commitV uint64,
	filePath string, jobName string, runNumber int64, expectedUserId int64, t *testing.T) {

	pipelineId := job.PipelineId(repoId, commit, commitV, filePath, jobName, runNumber)
	pipeline, ok := j.pipelineById[pipelineId]
	if !ok {
		t.Fatalf(
			"pipeline not found: repoId=%d commit=%d commitVersion=%d path=%q name=%q runNumber=%d",
			repoId, commit, commitV, filePath, jobName, runNumber,
		)
	}
	if !pipeline.IsCreatedByUser {
		t.Fatalf(
			"pipeline not created by user: repoId=%d commit=%d commitVersion=%d path=%q name=%q runNumber=%d",
			repoId, commit, commitV, filePath, jobName, runNumber,
		)
	}
	if pipeline.CreatedByUserId != expectedUserId {
		t.Fatalf(
			"pipeline created by unexpected user %d: repoId=%d commit=%d commitVersion=%d path=%q name=%q runNumber=%d",
			expectedUserId, repoId, commit, commitV, filePath, jobName, runNumber,
		)
	}
}

func (j *fakeJobsStorage) checkPipelineStatus(repoId uint64, commit, commitV uint64,
	filePath string, jobName string, runNumber int64, expectedStatus job.PipelineStatus, t *testing.T) {

	pipelineId := job.PipelineId(repoId, commit, commitV, filePath, jobName, runNumber)
	pipeline, ok := j.pipelineById[pipelineId]
	if !ok {
		t.Fatalf(
			"pipeline not found: repoId=%d commit=%d commitVersion=%d path=%q name=%q runNumber=%d",
			repoId, commit, commitV, filePath, jobName, runNumber,
		)
	}
	if pipeline.Status != expectedStatus {
		t.Fatalf(
			"unexpected pipeline status: repoId=%d commit=%d commitVersion=%d path=%q name=%q runNumber=%d expected=%s got=%s",
			repoId, commit, commitV, filePath, jobName, runNumber,
			expectedStatus, pipeline.Status,
		)
	}
}

func (j *fakeJobsStorage) CreateNewPipeline(tx context.Context,
	repoId uint64, commit uint64, commitV uint64,
	filePath string, jobName string, runNumber int64,
	stageNames []string, isCreatedByUser bool, createdByUserId int64) (job.Pipeline, error) {

	// Create the pipeline
	_, _ = j.PutPipelineRef(tx, repoId, filePath, jobName)
	if j.pipelineById == nil {
		j.pipelineById = map[string]job.Pipeline{}
	}
	p := job.Pipeline{
		RepoId:          repoId,
		Commit:          commit,
		CommitVersion:   commitV,
		Path:            filePath,
		Name:            jobName,
		RunNumber:       runNumber,
		NumberOfStages:  int32(len(stageNames)),
		Status:          job.PipelineStatusRunning,
		CreatedTime:     "", // skipped just to make testing easier
		IsCreatedByUser: isCreatedByUser,
		CreatedByUserId: createdByUserId,
	}
	_, ok := j.pipelineById[p.Id()]
	if ok {
		return job.Pipeline{}, errors.New("id already used")
	}
	j.pipelineById[p.Id()] = p

	// Create all the stages
	if j.pipelineStagesById == nil {
		j.pipelineStagesById = map[string][]job.PipelineStage{}
	}
	for i := range stageNames {
		stage := job.PipelineStage{
			PipelineId:  p.Id(),
			Name:        stageNames[i],
			Stage:       int32(i),
			CreatedTime: "",
			Status:      job.JobStatusWaiting,
		}
		j.pipelineStagesById[p.Id()] = append(j.pipelineStagesById[p.Id()], stage)
	}
	return p, nil
}
func (j *fakeJobsStorage) GetPipelineStage(tx context.Context, pipelineId string, stageN int32) (job.PipelineStage, error) {
	if j.pipelineStagesById == nil {
		j.pipelineStagesById = map[string][]job.PipelineStage{}
	}
	stages := j.pipelineStagesById[pipelineId]
	for _, stage := range stages {
		if stage.Stage == stageN {
			return stage, nil
		}
	}
	return job.PipelineStage{}, fmt.Errorf("stage %d not found in pipelineId=%q", stageN, pipelineId)
}
func (j *fakeJobsStorage) SetStatusOfPipelineStage(tx context.Context, pipelineId string, stageN int32, status job.JobStatus) error {
	if j.pipelineStagesById == nil {
		j.pipelineStagesById = map[string][]job.PipelineStage{}
	}
	stages := j.pipelineStagesById[pipelineId]
	for i := range stages {
		if stages[i].Stage == stageN {
			stages[i].Status = status
			j.pipelineStagesById[pipelineId] = stages
			return nil
		}
	}
	return fmt.Errorf("stage %d not found in pipelineId=%q", stageN, pipelineId)
}

func (j *fakeJobsStorage) checkPipelineStages(
	t *testing.T,
	jobPipelineId string,
	expected int,
) {
	t.Helper()
	count := 0
	if j.pipelineStagesById == nil {
		j.pipelineStagesById = map[string][]job.PipelineStage{}
	}
	stages := j.pipelineStagesById[jobPipelineId]
	for _, s := range stages {
		if s.PipelineId == jobPipelineId {
			count++
		}
	}
	if count != expected {
		t.Fatalf(
			"pipeline %s has %d stages, expected %d",
			jobPipelineId,
			count,
			expected,
		)
	}
}
func (j *fakeJobsStorage) checkPipelineStageStatus(
	t *testing.T,
	jobPipelineId string,
	stage int,
	expectedStatus job.JobStatus,
) {
	t.Helper()
	if j.pipelineStagesById == nil {
		j.pipelineStagesById = map[string][]job.PipelineStage{}
	}
	stages := j.pipelineStagesById[jobPipelineId]
	if len(stages) <= stage {
		t.Fatalf(
			"pipeline %s only has %d stages",
			jobPipelineId,
			len(stages),
		)
	}
	for _, stage_ := range stages {
		if stage_.Stage != int32(stage) {
			continue
		}
		if stage_.Status != expectedStatus {
			t.Fatalf(
				"pipeline %s stage %d has status %s, expected %s",
				jobPipelineId,
				stage,
				stages[stage].Status,
				expectedStatus,
			)
		}
		return
	}
	t.Fatalf(
		"pipeline %s stage %d not found",
		jobPipelineId,
		stage,
	)
}
func (j *fakeJobsStorage) SetResumerOfPipelineStage(tx context.Context, pipelineId string, stageN int32, userId int64) error {
	if j.pipelineStagesById == nil {
		j.pipelineStagesById = map[string][]job.PipelineStage{}
	}
	stages := j.pipelineStagesById[pipelineId]
	for i := range stages {
		if stages[i].Stage == stageN {
			stages[i].IsResumedByUser = true
			stages[i].ResumedByUserId = userId
			j.pipelineStagesById[pipelineId] = stages
			return nil
		}
	}
	return fmt.Errorf("stage %d not found in pipelineId=%q", stageN, pipelineId)
}
func (j *fakeJobsStorage) checkPipelineStageResumedByUser(
	t *testing.T,
	jobPipelineId string,
	stage int,
	expectedResumedByUserId int64,
) {
	t.Helper()
	if j.pipelineStagesById == nil {
		j.pipelineStagesById = map[string][]job.PipelineStage{}
	}
	stages := j.pipelineStagesById[jobPipelineId]
	if len(stages) <= stage {
		t.Fatalf(
			"pipeline %s only has %d stages",
			jobPipelineId,
			len(stages),
		)
	}
	for _, stage_ := range stages {
		if stage_.Stage != int32(stage) {
			continue
		}
		if !stage_.IsResumedByUser {
			t.Fatalf(
				"pipeline %s stage %d not resumed by user",
				jobPipelineId,
				stage,
			)
		}
		if stage_.ResumedByUserId != expectedResumedByUserId {
			t.Fatalf(
				"pipeline %s stage %d resumed by userId=%d, expected %d",
				jobPipelineId,
				stage,
				stage_.ResumedByUserId,
				expectedResumedByUserId,
			)
		}
		return
	}
	t.Fatalf(
		"pipeline %s stage %d not found",
		jobPipelineId,
		stage,
	)
}

func (j *fakeJobsStorage) GetRepoPipelineRefNextAvailableRunNumber(tx context.Context,
	repoId uint64, filePath string, jobName string) (int64, error) {
	found := false
	maxRun := int64(math.MinInt64)
	var maxRunPipeline job.Pipeline
	for _, pipeline := range j.pipelineById {
		if pipeline.RepoId == repoId &&
			pipeline.Path == filePath &&
			pipeline.Name == jobName {
			found = true
			if pipeline.RunNumber > maxRun {
				maxRunPipeline = pipeline
				maxRun = pipeline.RunNumber
			}
		}
	}
	if found {
		return maxRunPipeline.RunNumber + 1, nil
	}
	return 0, nil
}

type fakeTrackClient struct {
	runningJobs       map[string]bool
	postedJobPayloads []runnerlib.JobPayload
}

func (f *fakeTrackClient) Put(ownerId int64, jobId string, jobPayload runnerlib.JobPayload, tx context.Context) error {
	if f.runningJobs == nil {
		f.runningJobs = map[string]bool{}
	}
	// Idempotent: ignore duplicates
	if _, ok := f.runningJobs[jobId]; ok {
		return nil
	}
	f.runningJobs[jobId] = true
	f.postedJobPayloads = append(f.postedJobPayloads, jobPayload)
	return nil
}

func (f *fakeTrackClient) checkHasPayloadDisconsideringTokens(jobPayload runnerlib.JobPayload, t *testing.T) {
	t.Helper()
	removeTokens(f.postedJobPayloads, t)
	for i := range f.postedJobPayloads {
		if reflect.DeepEqual(f.postedJobPayloads[i], jobPayload) {
			return
		}
	}
	t.Fatalf("could not find payload")
}

func (f *fakeTrackClient) checkNumPostedPayloads(n int, t *testing.T) {
	if len(f.postedJobPayloads) != n {
		t.Fatalf("%d payloads were posted, expected %d", len(f.postedJobPayloads), n)
	}
}

type fakeParser struct {
	ciFileParsingMap map[string][]runnerlib.CiJob
	cdFileParsingMap map[string][]runnerlib.CdJob
}

func (f *fakeParser) ParseCiFile(ciFile []byte) ([]runnerlib.CiJob, bool, string) {
	if f.ciFileParsingMap == nil {
		f.ciFileParsingMap = map[string][]runnerlib.CiJob{}
	}
	jobs, ok := f.ciFileParsingMap[string(ciFile)]
	notOkMsg := ""
	if !ok {
		notOkMsg = "mock not ok msg"
	}
	return jobs, ok, notOkMsg
}
func (f *fakeParser) ParseCdFile(ciFile []byte) ([]runnerlib.CdJob, bool, string) {
	if f.cdFileParsingMap == nil {
		f.cdFileParsingMap = map[string][]runnerlib.CdJob{}
	}
	jobs, ok := f.cdFileParsingMap[string(ciFile)]
	notOkMsg := ""
	if !ok {
		notOkMsg = "mock not ok msg"
	}
	return jobs, ok, notOkMsg
}

type fakeMaxAllowedTimeoutGetter struct {
	userIdToDuration map[int64]time.Duration
}

func (f fakeMaxAllowedTimeoutGetter) GetMaxAllowedTimeout(
	repoOwnerId int64, repoId uint64, rl context.Context) (time.Duration, error) {
	if f.userIdToDuration == nil {
		return 0, errors.New("not found")
	}
	d, ok := f.userIdToDuration[repoOwnerId]
	if !ok {
		return 0, errors.New("not found")
	}
	return d, nil
}

// Helper to remove tokens from the jobpayloads for easier testing
func removeTokens(payloads []runnerlib.JobPayload, t *testing.T) {
	t.Helper()
	for j := range payloads {
		if payloads[j].Token == "" {
			continue
		}
		_, _, err := twiggtoken.ParseToken(payloads[j].Token, testSigner)
		if err != nil {
			t.Fatalf("unexpected twigg token: %s", err)
		}
		payloads[j].Token = ""
		for s := range payloads[j].Steps {
			_, _, err := twiggtoken.ParseToken(
				payloads[j].Steps[s].Env[runnerlib.TwiggTokenEnvVarName], testSigner)
			if err != nil {
				t.Fatalf("unexpected twigg token: %s", err)
			}
			payloads[j].Steps[s].Env[runnerlib.TwiggTokenEnvVarName] = ""
		}
	}
}

type mockFlagsProvider struct{}

func (f mockFlagsProvider) GetFlagsByRepoOwnerUserId(repoOwnerUserId int64, tx context.Context) (featureflags.Flags, error) {
	return featureflags.Flags{
		CreateCdJobs: true,
	}, nil
}
