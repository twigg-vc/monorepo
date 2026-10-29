package webdb

import (
	"errors"
	"monorepo/data/blobdb"
	"testing"
)

func Test_Reserve(t *testing.T) {
	q, err := newMemQuotaDb()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.close)
	err = q.AddQuota("a", 100)
	if err != nil {
		t.Fatal(err)
	}

	err = q.Reserve("a", 60)
	if err != nil {
		t.Fatal(err)
	}
	err = q.Reserve("a", 50)
	if !errors.Is(err, blobdb.ErrNotEnoughQuota) {
		t.Fatalf("err=%v, expected ErrNotEnoughQuota", err)
	}
	err = q.Reserve("a", -1)
	if err == nil {
		t.Fatal("expected an error reserving a negative size")
	}

	used, quotaLimitted, err := q.GetQuotaUsed("a")
	if err != nil {
		t.Fatal(err)
	}
	if used != 60 || quotaLimitted != 0 {
		t.Fatalf("used=%d quotaLimitted=%d, expected 60 and 0", used, quotaLimitted)
	}
}

func Test_FinalizeReservation(t *testing.T) {
	q, err := newMemQuotaDb()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.close)
	err = q.AddQuota("a", 100)
	if err != nil {
		t.Fatal(err)
	}

	err = q.Reserve("a", 60)
	if err != nil {
		t.Fatal(err)
	}
	err = q.FinalizeReservation("a", 60, 70)
	if err != nil {
		t.Fatal(err)
	}
	err = q.Reserve("a", 20)
	if err != nil {
		t.Fatal(err)
	}
	err = q.FinalizeReservation("a", 20, 0)
	if err != nil {
		t.Fatal(err)
	}

	used, _, err := q.GetQuotaUsed("a")
	if err != nil {
		t.Fatal(err)
	}
	if used != 70 {
		t.Fatalf("used=%d, expected 70", used)
	}
}
