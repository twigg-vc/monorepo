package jobs

import (
	"fmt"
	"monorepo/base/iterator"
	"monorepo/twigg-web/webdb"
	"reflect"
	"testing"
)

func TestCiCdRunWasPublished(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()

	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()

	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}
	const (
		repoId    = 1
		commit    = 2
		commitV   = 3
		runNumber = 4
	)
	wasPublished, err := s.CiCdRunWasPublished(w,
		repoId, commit, commitV, runNumber)
	if err != nil {
		t.Fatal(err)
	}
	if wasPublished {
		t.Fatal("run marked as published")
	}
	err = s.SetCiCdToPublished(w,
		repoId, commit, commitV, runNumber)
	if err != nil {
		t.Fatal(err)
	}
	wasPublished, err = s.CiCdRunWasPublished(w,
		repoId, commit, commitV, runNumber)
	if err != nil {
		t.Fatal(err)
	}
	if !wasPublished {
		t.Fatal("run marked as non-published after SetCiCdToPublished")
	}
}

func TestCreateJob(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()

	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()

	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}

	repoId := uint64(1)
	commit := uint64(1)
	commitV := uint64(2)
	path := "file/path"
	name := "job1"
	runNumber := int64(3)

	j, err := s.CreateNewJob(w, repoId, commit, commitV,
		path, name, runNumber, JobStatusQueued)
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	if j.InternalId != 1 {
		t.Fatalf("expected InternalId %d, got %d", 1, j.InternalId)
	}
	if j.RepoId != repoId {
		t.Fatalf("expected Repo %d, got %d", repoId, j.RepoId)
	}
	if j.Commit != commit {
		t.Fatalf("expected Commit %d, got %d", commit, j.Commit)
	}
	if j.CommitVersion != commitV {
		t.Fatalf("expected CommitVersion %d, got %d", commitV, j.CommitVersion)
	}
	if j.Path != path {
		t.Fatalf("expected Path %q, got %q", path, j.Path)
	}
	if j.Name != name {
		t.Fatalf("expected Name %q, got %q", name, j.Name)
	}
	if j.RunNumber != runNumber {
		t.Fatalf("expected runNumber %d, got %d", runNumber, j.RunNumber)
	}
	if j.Status != JobStatusQueued {
		t.Fatalf("expected Status %q, got %q", JobStatusQueued, j.Status)
	}
	if j.CreatedTime == "" {
		t.Fatalf("expected CreatedTime to be set, got empty")
	}

	gotJ, err := s.GetJobById(w, j.Id())
	if err != nil {
		t.Fatalf("GetJobById returned error: %v", err)
	}
	if j != gotJ {
		t.Fatalf("read unexpected job")
	}
}

func Test_PutPipelineRef_And_ArchivePipelineRefIfExists(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()
	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.PutPipelineRef(w, 1, "path", "name")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.PutPipelineRef(w, 1, "path2", "name2")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.PutPipelineRef(w, 1, "path3", "name3")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.PutPipelineRef(w, 2, "path", "name")
	if err != nil {
		t.Fatal(err)
	}

	// Helper to check the number of refs
	checkNumberOfRefs := func(expectedNumRefs int) {
		refsIter, err := s.GetRepoPipelineRefs(w, 1, "", "")
		if err != nil {
			t.Fatal(err)
		}
		refs, err := iterator.GetFirstN(expectedNumRefs+1, refsIter)
		if err != nil {
			t.Fatal(err)
		}
		if len(refs) != expectedNumRefs {
			t.Fatalf("expected %d refs got %d", expectedNumRefs, len(refs))
		}
	}
	checkNumberOfRefs(3)

	// Helper to check twice to ensure idempotency
	checkArchiveRef2 := func() {
		// Archive ref 2
		err = s.ArchivePipelineRefIfExists(w, 1, "path2", "name2")
		if err != nil {
			t.Fatal(err)
		}
		// Archiving non existing is ok
		err = s.ArchivePipelineRefIfExists(w, 1, "non-existing-path", "name")
		if err != nil {
			t.Fatal(err)
		}
		checkNumberOfRefs(2)
	}
	checkArchiveRef2()
	checkArchiveRef2()

	// Archive all to ensure no errors when nothing is found
	err = s.ArchivePipelineRefIfExists(w, 1, "path", "name")
	if err != nil {
		t.Fatal(err)
	}
	err = s.ArchivePipelineRefIfExists(w, 1, "path3", "name3")
	if err != nil {
		t.Fatal(err)
	}
	checkNumberOfRefs(0)

	// PutPipelineRef should unarchive
	_, err = s.PutPipelineRef(w, 1, "path", "name")
	if err != nil {
		t.Fatal(err)
	}
	checkNumberOfRefs(1)
}

func TestSetJobStatus(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()

	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()

	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}

	repoId := uint64(1)
	commit := uint64(1)
	commitV := uint64(2)
	path := "file/path"
	name := "job1"
	runNumber := int64(3)

	created, err := s.CreateNewJob(w, repoId, commit, commitV,
		path, name, runNumber, JobStatusQueued)
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	newStatus := JobStatusSuccess
	if err := s.SetJobStatus(w, created.Id(), newStatus); err != nil {
		t.Fatalf("SetJobStatus returned error: %v", err)
	}
	got, err := s.GetJobById(w, created.Id())
	if err != nil {
		t.Fatalf("GetJobById returned error: %v", err)
	}

	if got.Status != newStatus {
		t.Fatalf("expected Status %q, got %q", newStatus, got.Status)
	}
}
func TestGetCommitJobs(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()

	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()

	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}

	repoId := uint64(1)
	commit := uint64(1)
	commitV := uint64(2)

	// Jobs that must be return
	_, err = s.CreateNewJob(w, repoId, commit, commitV, "file/path-1", "jobname1", 1, JobStatusQueued)
	if err != nil {
		t.Fatalf("CreateNewJob returned error: %v", err)
	}
	_, err = s.CreateNewJob(w, repoId, commit, commitV, "file/path-2", "jobname2", 2, JobStatusPosted)
	if err != nil {
		t.Fatalf("CreateNewJob returned error: %v", err)
	}

	// Noise jobs
	_, err = s.CreateNewJob(w, repoId+99, commit, commitV, "file/other-1", "jobname3", 3, JobStatusQueued)
	if err != nil {
		t.Fatalf("CreateNewJob returned error: %v", err)
	}
	_, err = s.CreateNewJob(w, repoId, commit+1, commitV, "file/other-2", "jobname4", 4, JobStatusQueued)
	if err != nil {
		t.Fatalf("CreateNewJob returned error: %v", err)
	}

	it, err := s.GetCommitJobs(w, repoId, commit,
		/*afterInternalJobId*/ 0)
	if err != nil {
		t.Fatalf("GetCommitJobs returned error: %v", err)
	}
	jobs, err := iterator.GetFirstN(100, it)
	if err != nil {
		t.Fatalf("iterator Get returned error: %v", err)
	}
	const want = 2
	if len(jobs) != want {
		t.Fatalf("expected %d jobs, got %d", want, len(jobs))
	}
	for i := range jobs {
		jobs[i].CreatedTime = ""
	}
	expectedJobs := []Job{
		{
			InternalId:    2,
			RepoId:        repoId,
			Commit:        commit,
			CommitVersion: commitV,
			Status:        JobStatusPosted,
			CreatedTime:   "",
			Path:          "file/path-2",
			Name:          "jobname2",
			RunNumber:     2,
		},
		{
			InternalId:    1,
			RepoId:        repoId,
			Commit:        commit,
			CommitVersion: commitV,
			Status:        JobStatusQueued,
			CreatedTime:   "",
			Path:          "file/path-1",
			Name:          "jobname1",
			RunNumber:     1,
		},
	}
	if !reflect.DeepEqual(jobs, expectedJobs) {
		t.Fatalf("jobs = %#v, want %#v", jobs, expectedJobs)
	}
	// Read after id=2
	it, err = s.GetCommitJobs(w, repoId, commit,
		/*afterInternalJobId*/ 2)
	if err != nil {
		t.Fatalf("GetCommitJobs failed: %v", err)
	}
	jobs, err = iterator.GetFirstN(100, it)
	if err != nil {
		t.Fatalf("GetFirstN failed: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("unexpected jobs: %d", len(jobs))
	}
	jobs[0].CreatedTime = ""
	expectedJobsAfter1 := []Job{expectedJobs[1]}
	if !reflect.DeepEqual(jobs, expectedJobsAfter1) {
		t.Fatalf("jobs = %#v, want %#v", jobs, expectedJobsAfter1)
	}
}

func TestGetRepoJobs(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()

	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()

	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}

	repoId := uint64(1)
	otherRepoId := uint64(99)

	commit1 := uint64(1)
	commitV1 := uint64(1)

	commit2 := uint64(2)
	commitV2 := uint64(1)

	// Jobs that must be return (right repo)
	j1, _ := s.CreateNewJob(w, repoId, commit1, commitV1, "file/path-1", "jobname1", 1, JobStatusQueued)

	j2, _ := s.CreateNewJob(w, repoId, commit2, commitV2, "file/path-2", "jobname2", 2, JobStatusQueued)

	// Noise
	s.CreateNewJob(w, otherRepoId, commit1, commitV1, "file/other-1", "jobname3", 3, JobStatusQueued)

	it, _ := s.GetRepoJobs(w, repoId /*afterInternalJobId*/, 0)
	got, err := iterator.GetFirstN(100, it)
	if err != nil {
		t.Fatalf("failed to read iter: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(got))
	}
	expected := []Job{j2, j1}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("GetRepoJobs mismatch.\n got: %#v\n want: %#v", got, expected)
	}

	// Read after id=2
	it, _ = s.GetRepoJobs(w, repoId /*afterInternalJobId*/, j2.InternalId)
	got, err = iterator.GetFirstN(100, it)
	if err != nil {
		t.Fatalf("failed to read iter: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected jobs, got %d", len(got))
	}
	expected = []Job{j1}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("GetRepoJobs mismatch.\n got: %#v\n want: %#v", got, expected)
	}
}

func TestCantReuseRunNumber(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()

	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()

	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}

	repoId := uint64(1)
	commit := uint64(1)
	commitV := uint64(2)
	path := "file/path"
	name := "job1"

	_, err = s.CreateNewJob(w, repoId, commit, commitV, path, name, 99, JobStatusQueued)
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	_, err = s.CreateNewJob(w, repoId, commit, commitV, path, name, 100, JobStatusQueued)
	if err != nil {
		t.Fatalf("CreateJob returned error with different runNumber: %v", err)
	}
	_, err = s.CreateNewJob(w, repoId, commit, commitV,
		path, name, 99, JobStatusQueued)
	if err == nil {
		t.Fatalf("Got no error when reusing run number")
	}
}

func TestCreateNewPipeline(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()
	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}
	tx := w
	const (
		repoId          = 1
		commitId        = 2
		commitV         = 3
		filePath        = "path/to/file"
		jobName         = "name"
		runNumber       = 4
		isCreatedByUser = true
		createdByUserId = 54
	)
	jp, err := s.CreateNewPipeline(tx,
		repoId, commitId, commitV,
		filePath, jobName, runNumber,
		[]string{"stage0"}, isCreatedByUser, createdByUserId)
	if err != nil {
		t.Fatal(err)
	}
	if jp.CreatedTime == "" {
		t.Fatal("got empty crated time")
	}
	jp.CreatedTime = ""
	expected := Pipeline{
		InternalId:      1,
		RepoId:          repoId,
		Commit:          commitId,
		CommitVersion:   commitV,
		Path:            filePath,
		Name:            jobName,
		RunNumber:       runNumber,
		NumberOfStages:  1,
		Status:          PipelineStatusRunning,
		CreatedTime:     "",
		IsCreatedByUser: isCreatedByUser,
		CreatedByUserId: createdByUserId,
	}
	if !reflect.DeepEqual(jp, expected) {
		t.Fatalf("expected %#v got %#v", expected, jp)
	}
	stagesIter, err := s.GetPipelineStagesById(tx, jp.Id())
	if err != nil {
		t.Fatal(err)
	}
	stages, err := iterator.GetFirstN(100, stagesIter)
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 1 {
		t.Fatalf("unexpexted num of stages %d", len(stages))
	}
	stages[0].CreatedTime = ""
	expectedStage := PipelineStage{
		PipelineId:      jp.Id(),
		Stage:           0,
		Name:            "stage0",
		Status:          JobStatusWaiting,
		IsResumedByUser: false,
		ResumedByUserId: 0,
		CreatedTime:     "",
	}
	if !reflect.DeepEqual(expectedStage, stages[0]) {
		t.Fatalf("expected %#v got %#v", expected, stages[0])
	}

	// Create another one with isCreatedByUser=false and createdByUserId=99.
	// The userId should be ignored
	jp2, err := s.CreateNewPipeline(tx,
		repoId, commitId, commitV,
		filePath, jobName, runNumber+1,
		[]string{"stage0"},
		/*isCreatedByUser*/ false,
		/*createdByUserId*/ 99)
	if err != nil {
		t.Fatal(err)
	}
	if jp2.IsCreatedByUser {
		t.Fatalf("got pipeline is created by used with isCreatedByUser=false")
	}
	if jp2.CreatedByUserId != 0 {
		t.Fatalf("got pipeline CreatedByUserId != 0 by used with isCreatedByUser=false")
	}
}

func TestGetRepoPipelineRefNextAvailableRunNumber(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()
	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}
	tx := w
	const (
		repoId   = 1
		commitId = 2
		commitV  = 3
		filePath = "path/to/file"
		jobName  = "name"
	)
	// When no pipeline exists, returns zero
	n, err := s.GetRepoPipelineRefNextAvailableRunNumber(tx, repoId, filePath, jobName)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected 0 got %d", n)
	}

	// Else, returns the highest runNumber+1
	_, err = s.CreateNewPipeline(tx,
		repoId, commitId, commitV,
		filePath, jobName,
		/*runNumber=*/ 0,
		[]string{"stage0"}, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	n, err = s.GetRepoPipelineRefNextAvailableRunNumber(tx, repoId, filePath, jobName)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 got %d", n)
	}

	_, err = s.CreateNewPipeline(tx,
		repoId, commitId, commitV,
		filePath, jobName,
		/*runNumber=*/ 99,
		[]string{"stage0"}, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	n, err = s.GetRepoPipelineRefNextAvailableRunNumber(tx, repoId, filePath, jobName)
	if err != nil {
		t.Fatal(err)
	}
	if n != 100 {
		t.Fatalf("expected 100 got %d", n)
	}
}

func TestGetPipelineById(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()
	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}
	tx := w

	const (
		repoId          = 1
		commitId        = 2
		commitV         = 3
		filePath        = "path"
		jobName         = "job"
		runNumber       = 4
		numberOfStages  = 5
		isCreatedByUser = true
		createdByUserId = 21
	)
	jp, err := s.CreateNewPipeline(
		tx,
		repoId, commitId, commitV,
		filePath, jobName, runNumber,
		[]string{"first-stage"},
		isCreatedByUser, createdByUserId,
	)
	if err != nil {
		t.Fatal(err)
	}

	id := PipelineId(
		repoId, commitId, commitV,
		filePath, jobName, runNumber)
	got, err := s.GetPipelineById(tx, id)
	if err != nil {
		t.Fatal(err)
	}
	got.CreatedTime = ""
	jp.CreatedTime = ""
	if !reflect.DeepEqual(got, jp) {
		t.Fatalf("expected %#v got %#v", jp, got)
	}

	nonExistingId := PipelineId(
		repoId+1, commitId, commitV,
		filePath, jobName, runNumber)
	_, err = s.GetPipelineById(tx, nonExistingId)
	if err == nil {
		t.Fatal("got nil error for non existing id")
	}
}

func TestSetStatusOfPipelineStageAndGetPipelineStagesById(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()

	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()

	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}
	tx := w

	nonExistingId := PipelineId(
		0, 0, 0,
		"", "", 0)
	emptyIt, err := s.GetPipelineStagesById(tx, nonExistingId)
	if err != nil {
		t.Fatal(err)
	}
	if emptyIt.Next() {
		t.Fatal("got non empty iter for non existing pipeline id")
	}

	const (
		repoId         = 1
		commitId       = 2
		commitV        = 3
		filePath       = "path"
		jobName        = "job"
		runNumber      = 4
		numberOfStages = 2

		stage1     = 0
		stage1Name = "build"

		stage2     = 1
		stage2Name = "test"
	)

	pipe, err := s.CreateNewPipeline(
		tx,
		repoId, commitId, commitV,
		filePath, jobName, runNumber,
		[]string{stage1Name, stage2Name},
		false, 0,
	)
	if err != nil {
		t.Fatal(err)
	}
	err = s.SetStatusOfPipelineStage(tx, pipe.Id(), 0, JobStatusRunning)
	if err != nil {
		t.Fatal(err)
	}
	err = s.SetStatusOfPipelineStage(tx, pipe.Id(), 1, JobStatusQueued)
	if err != nil {
		t.Fatal(err)
	}

	it, err := s.GetPipelineStagesById(tx, pipe.Id())
	if err != nil {
		t.Fatal(err)
	}

	stages, err := iterator.GetFirstN(100, it)
	if err != nil {
		t.Fatal(err)
	}
	for i := range stages {
		stages[i].CreatedTime = ""
	}
	expected := []PipelineStage{
		{
			PipelineId:      pipe.Id(),
			Stage:           stage1,
			Name:            stage1Name,
			IsResumedByUser: false,
			ResumedByUserId: 0,
			Status:          JobStatusRunning,
		},
		{
			PipelineId:      pipe.Id(),
			Stage:           stage2,
			Name:            stage2Name,
			IsResumedByUser: false,
			ResumedByUserId: 0,
			Status:          JobStatusQueued,
		},
	}
	if !reflect.DeepEqual(stages, expected) {
		t.Fatalf("expected %#v got %#v", expected, stages)
	}
}

func TesLastStageOfPipelineToSuccess(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()
	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}
	tx := w
	const (
		repoId    = 1
		commitId  = 2
		commitV   = 3
		filePath  = "path"
		jobName   = "job"
		runNumber = 4
	)
	checkPipelineStatus := func(pipelineId string, expected PipelineStatus) {
		pipeline, err := s.GetPipelineById(tx, pipelineId)
		if err != nil {
			t.Fatal(err)
		}
		if pipeline.Status != expected {
			t.Fatalf("expected status %q got %q", expected, pipeline.Status)
		}
	}

	// Create a pipeline with two stages.
	pipeline, _ := s.CreateNewPipeline(
		tx,
		repoId, commitId, commitV,
		filePath, jobName, runNumber,
		[]string{"stage0", "stage1"},
		false, 0,
	)

	// Update the first stage to completed - pipeline should still be running
	err = s.SetStatusOfPipelineStage(tx, pipeline.Id(),
		0, JobStatusSuccess)
	if err != nil {
		t.Fatal(err)
	}
	checkPipelineStatus(pipeline.Id(), PipelineStatusRunning)
	// Update second stage - pipeline should be marked as succeeded
	err = s.SetStatusOfPipelineStage(tx, pipeline.Id(),
		1, JobStatusSuccess)
	if err != nil {
		t.Fatal(err)
	}
	checkPipelineStatus(pipeline.Id(), PipelineStatusSuccess)
}

func TestSetIntermediaryStageToFailure(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()
	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}
	tx := w
	const (
		repoId    = 1
		commitId  = 2
		commitV   = 3
		filePath  = "path"
		jobName   = "job"
		runNumber = 4
	)
	checkPipelineStatus := func(pipelineId string, expected PipelineStatus) {
		pipeline, err := s.GetPipelineById(tx, pipelineId)
		if err != nil {
			t.Fatal(err)
		}
		if pipeline.Status != expected {
			t.Fatalf("expected status %q got %q", expected, pipeline.Status)
		}
	}

	// Create a pipeline with three stages.
	// First is running, others are waiting
	pipeline, _ := s.CreateNewPipeline(
		tx,
		repoId, commitId, commitV,
		filePath, jobName, runNumber,
		[]string{"stage0", "stage1", "stage2"},
		false, 0,
	)
	s.SetStatusOfPipelineStage(tx, pipeline.Id(), 0, JobStatusRunning)

	// Update the first stage to completed -> pipeline should still be running
	err = s.SetStatusOfPipelineStage(tx, pipeline.Id(),
		0, JobStatusSuccess)
	if err != nil {
		t.Fatal(err)
	}
	checkPipelineStatus(pipeline.Id(), PipelineStatusRunning)
	// Update second stage to "failed" -> status should update
	err = s.SetStatusOfPipelineStage(tx, pipeline.Id(),
		1, JobStatusFail)
	if err != nil {
		t.Fatal(err)
	}
	checkPipelineStatus(pipeline.Id(), PipelineStatusFail)
}

func TestManualStartStageProgression(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()
	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}
	tx := w
	const (
		repoId    = 1
		commitId  = 2
		commitV   = 3
		filePath  = "path"
		jobName   = "job"
		runNumber = 4
	)
	checkPipelineStatus := func(pipelineId string, expected PipelineStatus) {
		pipeline, err := s.GetPipelineById(tx, pipelineId)
		if err != nil {
			t.Fatal(err)
		}
		if pipeline.Status != expected {
			t.Fatalf("expected status %q got %q", expected, pipeline.Status)
		}
	}
	// Create pipeline with 3 stages. First is running, others are waiting
	pipeline, _ := s.CreateNewPipeline(
		tx,
		repoId, commitId, commitV,
		filePath, jobName, runNumber,
		[]string{"stage0", "stage1", "stage2"},
		false, 0,
	)
	s.SetStatusOfPipelineStage(tx, pipeline.Id(), 0, JobStatusRunning)

	// Set first to success and next to WaitingForManual
	s.SetStatusOfPipelineStage(tx, pipeline.Id(), 0, JobStatusSuccess)
	s.SetStatusOfPipelineStage(tx, pipeline.Id(), 1, JobStatusWaitingManualStart)
	checkPipelineStatus(pipeline.Id(), PipelineStatusWaitingManualStart)

	// Enqueue and finish stage 1
	s.SetStatusOfPipelineStage(tx, pipeline.Id(), 1, JobStatusQueued)
	checkPipelineStatus(pipeline.Id(), PipelineStatusRunning)
	s.SetStatusOfPipelineStage(tx, pipeline.Id(), 1, JobStatusPosted)
	checkPipelineStatus(pipeline.Id(), PipelineStatusRunning)
	s.SetStatusOfPipelineStage(tx, pipeline.Id(), 1, JobStatusRunning)
	checkPipelineStatus(pipeline.Id(), PipelineStatusRunning)
	s.SetStatusOfPipelineStage(tx, pipeline.Id(), 1, JobStatusSuccess)
	checkPipelineStatus(pipeline.Id(), PipelineStatusRunning)

	// Start stage 2
	s.SetStatusOfPipelineStage(tx, pipeline.Id(), 2, JobStatusQueued)
	checkPipelineStatus(pipeline.Id(), PipelineStatusRunning)
	s.SetStatusOfPipelineStage(tx, pipeline.Id(), 2, JobStatusPosted)
	checkPipelineStatus(pipeline.Id(), PipelineStatusRunning)

	// Finish stage 2
	s.SetStatusOfPipelineStage(tx, pipeline.Id(), 2, JobStatusRunning)
	checkPipelineStatus(pipeline.Id(), PipelineStatusRunning)
	s.SetStatusOfPipelineStage(tx, pipeline.Id(), 2, JobStatusSuccess)
	checkPipelineStatus(pipeline.Id(), PipelineStatusSuccess)
}

func TestGetRepoPipelineNamesAndGetRepoPipelinesByName(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()
	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}
	tx := w

	emptyRefIt, err := s.GetRepoPipelineRefs(tx, 99, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if emptyRefIt.Next() {
		t.Fatalf("got non empty it when no refs existed")
	}
	emptyPipelineIt, err := s.GetRepoPipelinesByRef(tx,
		99, "non/existing", "non-existing", 0)
	if err != nil {
		t.Fatal(err)
	}
	if emptyPipelineIt.Next() {
		t.Fatalf("got non empty it when no pipelines existed")
	}

	// InternalIds are incremental, so to make this test easy to follow
	// we'll use this map to keep track of the internalId of each pipeline
	pipelineIdToInternalId := map[string]int64{}
	for repoId := uint64(1); repoId <= 2; repoId++ {
		for runNumber := 0; runNumber <= 1; runNumber++ {
			for fileId := 1; fileId <= 2; fileId++ {
				for nameId := 1; nameId <= 2; nameId++ {
					filePath := fmt.Sprintf("path/to/file%d", fileId)
					jobName := fmt.Sprintf("name%d", nameId)
					p, err := s.CreateNewPipeline(tx,
						/*repoId*/ repoId,
						/*commitId*/ 91,
						/*commitV*/ 92,
						filePath, jobName,
						int64(runNumber),
						[]string{"stage0", "stage1", "stage2"},
						/*isCreatedByUser*/ true,
						/*isCreatedByUserId*/ 78)
					if err != nil {
						t.Fatal(err)
					}
					pipelineIdToInternalId[p.Id()] = p.InternalId
				}
			}
		}
	}
	// Put a ref that has no actual pipeline running yet.
	// Run it twice to check idempotency
	checkPut := func() {
		ref, err := s.PutPipelineRef(tx, 1, "a/put/path", "put-name")
		if err != nil {
			t.Fatal(err)
		}
		expected := PipelineRef{RepoId: 1, Path: "a/put/path", Name: "put-name"}
		if ref != expected {
			t.Fatalf("unexpected ref %#v", ref)
		}
		ref, err = s.PutPipelineRef(tx, 2, "a/put/path", "put-name")
		if err != nil {
			t.Fatal(err)
		}
		expected = PipelineRef{RepoId: 2, Path: "a/put/path", Name: "put-name"}
		if ref != expected {
			t.Fatalf("unexpected ref %#v", ref)
		}
	}
	checkPut()
	checkPut()

	refsIt, err := s.GetRepoPipelineRefs(tx, 1, "", "")
	if err != nil {
		t.Fatal(err)
	}
	refs, err := iterator.GetFirstN(100, refsIt)
	if err != nil {
		t.Fatal(err)
	}
	expectedNames := []PipelineRef{
		{RepoId: 1, Path: "a/put/path", Name: "put-name"},
		{RepoId: 1, Path: "path/to/file1", Name: "name1"},
		{RepoId: 1, Path: "path/to/file1", Name: "name2"},
		{RepoId: 1, Path: "path/to/file2", Name: "name1"},
		{RepoId: 1, Path: "path/to/file2", Name: "name2"},
	}
	if !reflect.DeepEqual(refs, expectedNames) {
		t.Fatalf("got refs %#v expected %#v", refs, expectedNames)
	}
	// Read after
	refsIt, err = s.GetRepoPipelineRefs(tx, 1, "path/to/file1", "name1")
	if err != nil {
		t.Fatal(err)
	}
	refs, err = iterator.GetFirstN(100, refsIt)
	if err != nil {
		t.Fatal(err)
	}
	expectedNames = []PipelineRef{
		{RepoId: 1, Path: "path/to/file1", Name: "name2"},
		{RepoId: 1, Path: "path/to/file2", Name: "name1"},
		{RepoId: 1, Path: "path/to/file2", Name: "name2"},
	}
	if !reflect.DeepEqual(refs, expectedNames) {
		t.Fatalf("got refs %#v expected %#v", refs, expectedNames)
	}

	pipelinesIt, err := s.GetRepoPipelinesByRef(tx, 1, "path/to/file1", "name1", 0)
	if err != nil {
		t.Fatal(err)
	}
	pipelines, err := iterator.GetFirstN(100, pipelinesIt)
	if err != nil {
		t.Fatal(err)
	}
	for i := range pipelines {
		// Strip the CreatedTime for testing
		if pipelines[i].CreatedTime == "" {
			t.Fatal("got pipeline with empty CreatedAt")
		}
		pipelines[i].CreatedTime = ""
	}
	expectedPipelines := []Pipeline{
		{
			InternalId:      pipelineIdToInternalId[PipelineId(1, 91, 92, "path/to/file1", "name1", 1)],
			RepoId:          1,
			Commit:          91,
			CommitVersion:   92,
			Path:            "path/to/file1",
			Name:            "name1",
			RunNumber:       1,
			NumberOfStages:  3,
			Status:          PipelineStatusRunning,
			CreatedTime:     "",
			IsCreatedByUser: true,
			CreatedByUserId: 78,
		},
		{
			InternalId:      pipelineIdToInternalId[PipelineId(1, 91, 92, "path/to/file1", "name1", 0)],
			RepoId:          1,
			Commit:          91,
			CommitVersion:   92,
			Path:            "path/to/file1",
			Name:            "name1",
			RunNumber:       0,
			NumberOfStages:  3,
			Status:          PipelineStatusRunning,
			CreatedTime:     "",
			IsCreatedByUser: true,
			CreatedByUserId: 78,
		},
	}
	if !reflect.DeepEqual(pipelines, expectedPipelines) {
		t.Fatalf("got pipelines %#v expected %#v", pipelines, expectedPipelines)
	}

	// Query after the first one
	pipelinesIt, err = s.GetRepoPipelinesByRef(tx, 1, "path/to/file1", "name1",
		expectedPipelines[0].InternalId)
	if err != nil {
		t.Fatal(err)
	}
	pipelines, err = iterator.GetFirstN(100, pipelinesIt)
	if err != nil {
		t.Fatal(err)
	}
	for i := range pipelines {
		// Strip the CreatedTime for testing
		pipelines[i].CreatedTime = ""
	}
	expectedPipelinesAfter := []Pipeline{
		expectedPipelines[1],
	}
	if !reflect.DeepEqual(pipelines, expectedPipelinesAfter) {
		t.Fatalf("got pipelines %#v expected %#v", pipelines, expectedPipelinesAfter)
	}
}

func TestSetToPosted(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()
	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}
	tx := w
	const (
		repoId         = 1
		commitId       = 2
		commitV        = 3
		filePath       = "path"
		jobName        = "job"
		runNumber      = 4
		numberOfStages = 1
		stageName      = "build"
	)

	// Set a job to "posted"
	j, err := s.CreateNewJob(w, repoId, commitId, commitV, filePath, jobName,
		runNumber, JobStatusQueued)
	if err != nil {
		t.Fatal(err)
	}
	err = s.SetToPosted(w, j.Id())
	if err != nil {
		t.Fatal(err)
	}
	j, err = s.GetJobById(w, j.Id())
	if err != nil {
		t.Fatal(err)
	}
	if j.Status != JobStatusPosted {
		t.Fatalf("unexpected job status %s", j.Status)
	}

	// Set a PipelineStage to "posted"
	pipeline, _ := s.CreateNewPipeline(tx, repoId, commitId, commitV, filePath,
		jobName, runNumber, []string{"stage0"}, false, 0)
	stage, err := s.GetPipelineStage(tx, pipeline.Id(), 0)
	if err != nil {
		t.Fatal(err)
	}
	err = s.SetToPosted(w, stage.Id())
	if err != nil {
		t.Fatal(err)
	}
	stagesIter, err := s.GetPipelineStagesById(tx, pipeline.Id())
	if err != nil {
		t.Fatal(err)
	}
	stages, err := iterator.GetFirstN(100, stagesIter)
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 1 {
		t.Fatalf("unexpected num of stages %d", len(stages))
	}
	if stages[0].Status != JobStatusPosted {
		t.Fatalf("unexpected status %s", stages[0].Status)
	}
}

func TestGetPipelineStage(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()
	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}
	tx := w
	const (
		repoId         = 1
		commitId       = 2
		commitV        = 3
		filePath       = "path"
		jobName        = "job"
		runNumber      = 4
		numberOfStages = 2
	)
	// Create a pipeline with two stages
	pipeline, _ := s.CreateNewPipeline(tx, repoId, commitId, commitV, filePath,
		jobName, runNumber, []string{"stage0", "stage1"}, false, 0)
	// Get each stage directly with the GetPipelineStage helper
	gotStage0, err := s.GetPipelineStage(tx, pipeline.Id(), 0)
	if err != nil {
		t.Fatal(err)
	}
	gotStage0.CreatedTime = ""
	expectedStage0 := PipelineStage{
		PipelineId:  pipeline.Id(),
		Name:        "stage0",
		Stage:       0,
		CreatedTime: "",
		Status:      JobStatusWaiting,
	}
	if gotStage0 != expectedStage0 {
		t.Fatalf("bad stage0: %#v", gotStage0)
	}
	gotStage1, err := s.GetPipelineStage(tx, pipeline.Id(), 1)
	if err != nil {
		t.Fatal(err)
	}
	gotStage1.CreatedTime = ""
	expectedStage1 := PipelineStage{
		PipelineId:  pipeline.Id(),
		Name:        "stage1",
		Stage:       1,
		CreatedTime: "",
		Status:      JobStatusWaiting,
	}
	if gotStage1 != expectedStage1 {
		t.Fatalf("bad stage1: %#v", gotStage1)
	}

	// Try reading non existing
	_, err = s.GetPipelineStage(tx, pipeline.Id(), 99999)
	if err == nil {
		t.Fatal("got nil err reading non existing stage")
	}
}

func TestCanPutResumePipelineToStage(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()
	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}
	tx := w
	const (
		repoId         = 1
		commitId       = 2
		commitV        = 3
		filePath       = "path"
		jobName        = "job"
		runNumber      = 4
		numberOfStages = 2
	)
	// Create a pipeline with two stages; one at "posted" and the other at "waiting" stage
	pl, _ := s.CreateNewPipeline(tx, repoId, commitId, commitV, filePath,
		jobName, runNumber, []string{"stage0", "stage1"}, false, 0)
	err = s.SetStatusOfPipelineStage(tx, pl.Id(), 0, JobStatusPosted)
	if err != nil {
		t.Fatal(err)
	}

	// Cant ever resume to stage 0 because that's the first stage: get an err
	_, err = s.CanPutResumePipelineToStage(tx, pl.Id(), 0)
	if err == nil {
		t.Fatal("got no error for resumeToStage 0")
	}
	// Can't resume to 1 because stage 0 is posted
	canResumeStage1, err := s.CanPutResumePipelineToStage(tx, pl.Id(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if canResumeStage1 {
		t.Fatalf("got canResume from waiting stage")
	}

	// Set 0 to "success" -> this should allow pipeline to resume
	err = s.SetStatusOfPipelineStage(tx, pl.Id(), 0, JobStatusSuccess)
	if err != nil {
		t.Fatal(err)
	}
	canResumeStage1, err = s.CanPutResumePipelineToStage(tx, pl.Id(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !canResumeStage1 {
		t.Fatalf("got notCanResume when stage 0 succeded")
	}

	// Even if the stage has "succeeded" status, CanPutResumePipelineToStage
	// returns true, because the "Put" suggests that we can perform an
	// idempotent resume that would just do nothing if the pipeline is already
	// at that stage
	err = s.SetStatusOfPipelineStage(tx, pl.Id(), 1, JobStatusSuccess)
	if err != nil {
		t.Fatal(err)
	}
	canResumeStage1, err = s.CanPutResumePipelineToStage(tx, pl.Id(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !canResumeStage1 {
		t.Fatalf("got notCanResume stage 1 succeded")
	}
}

func TestSetResumerOfPipelineStage(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer clW()
	s, err := NewService(db, w)
	if err != nil {
		t.Fatal(err)
	}
	tx := w
	const (
		repoId         = 1
		commitId       = 2
		commitV        = 3
		filePath       = "path"
		jobName        = "job"
		runNumber      = 4
		numberOfStages = 2
	)
	// Create a pipeline with two stages on "waiting" status
	pl, _ := s.CreateNewPipeline(tx, repoId, commitId, commitV, filePath,
		jobName, runNumber, []string{"stage0", "stage1"}, false, 0)

	// Set the resumer of stage 0 to be user 99
	err = s.SetResumerOfPipelineStage(tx, pl.Id(), 0, 99)
	if err != nil {
		t.Fatal(err)
	}
	// Set the resumer of stage 1 to be user 98
	err = s.SetResumerOfPipelineStage(tx, pl.Id(), 1, 98)
	if err != nil {
		t.Fatal(err)
	}

	// Check getter methods work ok
	stage0, err := s.GetPipelineStage(tx, pl.Id(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !stage0.IsResumedByUser {
		t.Fatalf("stage0 not marked as resumed by user")
	}
	if stage0.ResumedByUserId != 99 {
		t.Fatalf("stage0 resumed by user %d", stage0.ResumedByUserId)
	}
	stage1, err := s.GetPipelineStage(tx, pl.Id(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !stage1.IsResumedByUser {
		t.Fatalf("stage1 not marked as resumed by user")
	}
	if stage1.ResumedByUserId != 98 {
		t.Fatalf("stage1 resumed by user %d", stage1.ResumedByUserId)
	}

	// Expect err for bad id
	err = s.SetResumerOfPipelineStage(tx, "bad id", 0, 98)
	if err == nil {
		t.Fatal("SetResumerOfPipelineStage got nil err for bad id")
	}
	// Expect err for bad stage
	err = s.SetResumerOfPipelineStage(tx, pl.Id(), 2, 98)
	if err == nil {
		t.Fatal("SetResumerOfPipelineStage got nil err for bad stage")
	}
	err = s.SetResumerOfPipelineStage(tx, pl.Id(), -1, 98)
	if err == nil {
		t.Fatal("SetResumerOfPipelineStage got nil err for bad stage")
	}
}

func TestParsing(t *testing.T) {
	p := Pipeline{
		InternalId:     1,
		RepoId:         1,
		Commit:         91,
		CommitVersion:  92,
		Path:           "path/to/file1",
		Name:           "name1",
		RunNumber:      1,
		NumberOfStages: 93,
		Status:         PipelineStatusRunning,
		CreatedTime:    "",
	}
	RepoId, Commit, CommitVersion,
		Path, Name, RunNumber, ok := ParsePipelineId(p.Id())
	if !ok {
		t.Fatal("parsing failed")
	}
	if RepoId != p.RepoId {
		t.Fatalf("bad RepoId: got %d want %d", RepoId, p.RepoId)
	}
	if Commit != p.Commit {
		t.Fatalf("bad Commit: got %d want %d", Commit, p.Commit)
	}
	if CommitVersion != p.CommitVersion {
		t.Fatalf("bad CommitVersion: got %d want %d", CommitVersion, p.CommitVersion)
	}
	if Path != p.Path {
		t.Fatalf("bad Path: got %q want %q", Path, p.Path)
	}
	if Name != p.Name {
		t.Fatalf("bad Name: got %q want %q", Name, p.Name)
	}
	if RunNumber != p.RunNumber {
		t.Fatalf("bad RunNumber: got %d want %d", RunNumber, p.RunNumber)
	}
	stageId := p.IdOfStage(8)
	RepoId, Commit, CommitVersion,
		Path, Name, RunNumber, Stage, ok := ParsePipelineStageId(stageId)
	if !ok {
		t.Fatal("parsing failed")
	}
	if RepoId != p.RepoId {
		t.Fatalf("bad RepoId: got %d want %d", RepoId, p.RepoId)
	}
	if Commit != p.Commit {
		t.Fatalf("bad Commit: got %d want %d", Commit, p.Commit)
	}
	if CommitVersion != p.CommitVersion {
		t.Fatalf("bad CommitVersion: got %d want %d", CommitVersion, p.CommitVersion)
	}
	if Path != p.Path {
		t.Fatalf("bad Path: got %q want %q", Path, p.Path)
	}
	if Name != p.Name {
		t.Fatalf("bad Name: got %q want %q", Name, p.Name)
	}
	if RunNumber != p.RunNumber {
		t.Fatalf("bad RunNumber: got %d want %d", RunNumber, p.RunNumber)
	}
	if Stage != 8 {
		t.Fatalf("bad Stage: got %d want %d", Stage, 8)
	}
}