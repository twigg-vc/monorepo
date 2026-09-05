package webdb_test

import (
	"context"
	"monorepo/base/iterator"
	"monorepo/twigg-web/job"
	"monorepo/twigg-web/webdb"
	"slices"
	"testing"
)

func TestInsertAndGetPipelineStages(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const (
		pipelineId  = "p-1.2.3.cGF0aA.bmFtZQ.4"
		createdTime = "2026-09-05T00:00:00Z"
	)
	const readAll = 10

	got, err := iterator.GetFirstN(readAll, getStagesOrDie(t, b, w, pipelineId))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no stages before the inserts, got %+v", got)
	}

	// Insert out of order to check the ordering
	err = b.InsertPipelineStage(w, pipelineId, 1, "say bye", createdTime, job.JobStatusWaiting)
	if err != nil {
		t.Fatal(err)
	}
	err = b.InsertPipelineStage(w, pipelineId, 0, "say hi", createdTime, job.JobStatusQueued)
	if err != nil {
		t.Fatal(err)
	}
	err = b.InsertPipelineStage(w, "p-other", 0, "other", createdTime, job.JobStatusWaiting)
	if err != nil {
		t.Fatal(err)
	}

	got, err = iterator.GetFirstN(readAll, getStagesOrDie(t, b, w, pipelineId))
	if err != nil {
		t.Fatal(err)
	}
	expected := []job.PipelineStage{
		{PipelineId: pipelineId, Stage: 0, Name: "say hi", CreatedTime: createdTime, Status: job.JobStatusQueued},
		{PipelineId: pipelineId, Stage: 1, Name: "say bye", CreatedTime: createdTime, Status: job.JobStatusWaiting},
	}
	if !slices.Equal(got, expected) {
		t.Fatalf("expected the stages of the pipeline ordered by stage, got %+v", got)
	}
}

func TestInsertPipelineStageMissingFields(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const createdTime = "2026-09-05T00:00:00Z"
	err = b.InsertPipelineStage(w, "", 0, "name", createdTime, job.JobStatusWaiting)
	if err == nil {
		t.Fatal("expected an error when the pipelineId is missing")
	}
	err = b.InsertPipelineStage(w, "p-1", 0, "name", createdTime, "")
	if err == nil {
		t.Fatal("expected an error when the status is missing")
	}
	err = b.InsertPipelineStage(w, "p-1", 0, "name", "", job.JobStatusWaiting)
	if err == nil {
		t.Fatal("expected an error when the createdTime is missing")
	}
}

func getStagesOrDie(t *testing.T, b webdb.WebDb, ctx context.Context,
	pipelineId string) iterator.I[job.PipelineStage] {
	t.Helper()
	iter, err := b.GetPipelineStages(ctx, pipelineId)
	if err != nil {
		t.Fatal(err)
	}
	return iter
}

func TestSetPipelineStageStatusAndResumer(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const (
		pipelineId  = "p-1.2.3.cGF0aA.bmFtZQ.4"
		createdTime = "2026-09-05T00:00:00Z"
	)

	isNotFoundErr, err := b.SetPipelineStageStatus(w, pipelineId, 0, job.JobStatusRunning)
	if !isNotFoundErr || err == nil {
		t.Fatal("expected is not found err")
	}
	isNotFoundErr, err = b.SetPipelineStageResumer(w, pipelineId, 0, 99)
	if !isNotFoundErr || err == nil {
		t.Fatal("expected is not found err")
	}

	err = b.InsertPipelineStage(w, pipelineId, 0, "say hi", createdTime, job.JobStatusWaiting)
	if err != nil {
		t.Fatal(err)
	}

	isNotFoundErr, err = b.SetPipelineStageStatus(w, pipelineId, 0, job.JobStatusSuccess)
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("got is not found err")
	}
	isNotFoundErr, err = b.SetPipelineStageResumer(w, pipelineId, 0, 99)
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("got is not found err")
	}

	got, err := iterator.GetFirstN(10, getStagesOrDie(t, b, w, pipelineId))
	if err != nil {
		t.Fatal(err)
	}
	expected := []job.PipelineStage{{
		PipelineId:      pipelineId,
		Stage:           0,
		Name:            "say hi",
		CreatedTime:     createdTime,
		Status:          job.JobStatusSuccess,
		IsResumedByUser: true,
		ResumedByUserId: 99,
	}}
	if !slices.Equal(got, expected) {
		t.Fatalf("expected the stage updated, got %+v", got)
	}

	// Another stage of the same pipeline is untouched
	isNotFoundErr, err = b.SetPipelineStageStatus(w, pipelineId, 1, job.JobStatusFail)
	if !isNotFoundErr || err == nil {
		t.Fatal("expected is not found err")
	}

	_, err = b.SetPipelineStageStatus(w, pipelineId, 0, "")
	if err == nil {
		t.Fatal("expected an error when the status is missing")
	}
}