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

	got, err := iterator.GetFirstN(readAll, mustGetStages(t, b, w, pipelineId))
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

	got, err = iterator.GetFirstN(readAll, mustGetStages(t, b, w, pipelineId))
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

func mustGetStages(t *testing.T, b webdb.WebDb, ctx context.Context,
	pipelineId string) iterator.I[job.PipelineStage] {
	t.Helper()
	iter, err := b.GetPipelineStages(ctx, pipelineId)
	if err != nil {
		t.Fatal(err)
	}
	return iter
}
