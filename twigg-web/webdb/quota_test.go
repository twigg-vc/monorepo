package webdb_test

import (
	"errors"
	"monorepo/data/blobdb"
	"monorepo/twigg-web/webdb"
	"strings"
	"testing"
)

func Test_QuotaUsed(t *testing.T) {
	db := getNewDb(t)
	w, closeW, commitW, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	db.SetBlob(w, "a", "", "", bytesWriterTo(strings.Repeat("a", 100)))
	db.SetBlob(w, "a", "", "id2", bytesWriterTo(strings.Repeat("a", 50)))
	db.SetBlob(w, "b", "", "", bytesWriterTo("12"))

	n, l, err := db.GetQuotaUsed("a")
	if err != nil {
		t.Fatal(err)
	}
	if !(n > 10 && n < 150) {
		t.Fatalf("got %d bytes for a", n)
	}
	if l != 0 {
		t.Fatalf("got %d quotaLimitted for a", l)
	}

	n, l, err = db.GetQuotaUsed("non existing")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("got %d bytes for non existing quota owner", n)
	}
	if l != 0 {
		t.Fatalf("got %d quotaLimitted for non existing quota owner", l)
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}

func Test_QuotaLeft(t *testing.T) {
	db := getNewDb(t)
	w, closeW, commitW, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	n, err := db.GetQuotaLeft("unknown")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("got %d bytes left", n)
	}

	err = db.AddQuota("a", 50)
	if err != nil {
		t.Fatal(err)
	}
	n, err = db.GetQuotaLeft("a")
	if err != nil {
		t.Fatal(err)
	}
	if n != 50 {
		t.Fatalf("got %d bytes left", n)
	}

	err = db.AddQuota("a", 50)
	if err != nil {
		t.Fatal(err)
	}
	n, err = db.GetQuotaLeft("a")
	if err != nil {
		t.Fatal(err)
	}
	if n != 100 {
		t.Fatalf("got %d bytes left", n)
	}

	err = db.SetQuota("a", 200)
	if err != nil {
		t.Fatal(err)
	}
	n, err = db.GetQuotaLeft("a")
	if err != nil {
		t.Fatal(err)
	}
	if n != 200 {
		t.Fatalf("got %d bytes left", n)
	}

	db.SetBlob(w, "a", "", "", bytesWriterTo(strings.Repeat("a", 20)))
	n, err = db.GetQuotaLeft("a")
	if err != nil {
		t.Fatal(err)
	}
	if !(n > 0 && n < 200) {
		t.Fatalf("got %d bytes left", n)
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}

func Test_CantReduceQuotaBelowUsage(t *testing.T) {
	db := getNewDb(t)
	w, closeW, commitW, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	err = db.AddQuota("a", 50)
	if err != nil {
		t.Fatal(err)
	}
	// Ok to reduce if not used
	err = db.SetQuota("a", 0)
	if err != nil {
		t.Fatal(err)
	}
	// Error if reducing below usage
	db.SetBlob(w, "a", "", "", bytesWriterTo("abc"))
	err = db.SetQuota("a", 1)
	if err == nil {
		t.Fatal("got no error setting quota below current usage")
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}

func Test_FreezeQuota(t *testing.T) {
	db := getNewDb(t)
	w, closeW, commitW, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	err = db.AddQuota("a", 1000)
	if err != nil {
		t.Fatal(err)
	}
	db.SetBlob(w, "a", "", "", bytesWriterTo(strings.Repeat("a", 20)))
	used, _, err := db.GetQuotaUsed("a")
	if err != nil {
		t.Fatal(err)
	}

	err = db.FreezeQuota("a")
	if err != nil {
		t.Fatal(err)
	}
	quota, err := db.GetQuota("a")
	if err != nil {
		t.Fatal(err)
	}
	if quota != used {
		t.Fatalf("quota=%d, expected frozen at usage=%d", quota, used)
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}

func Test_QuotaEnforcement(t *testing.T) {
	db, closeDb, err := webdb.NewMemWithQuotaEnforcement()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDb()

	w, closeW, commitW, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	err = db.AddQuota("a", 200)
	if err != nil {
		t.Fatal(err)
	}
	quotaLeft, err := db.GetQuotaLeft("a")
	if err != nil {
		t.Fatal(err)
	}
	nWritten := 0
	// Doesn't use strict quota values because, depending on compression, more
	// than 1 byte might be needed to store each byte of tiny writes.
	for quotaLeft > 20 {
		nWritten++
		_, err = db.SetBlob(w, "a", "id-prefix", "id", bytesWriterTo("x"))
		if err != nil {
			t.Fatal(err)
		}
		quotaLeft, err = db.GetQuotaLeft("a")
		if err != nil {
			t.Fatal(err)
		}
	}
	if nWritten < 3 {
		t.Fatal("too few written")
	}

	// A write bigger than what's left must fail with ErrNotEnoughQuota and
	// must not silently succeed.
	_, err = db.SetBlob(w, "a", "id-prefix", "id2", bytesWriterTo(strings.Repeat("x", 100)))
	if !errors.Is(err, blobdb.ErrNotEnoughQuota) {
		t.Fatalf("err=%v, expected ErrNotEnoughQuota", err)
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}

func Test_DoesntChargeQuotaOnFailedWrite(t *testing.T) {
	db, closeDb, err := webdb.NewMemWithQuotaEnforcement()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDb()

	w, closeW, commitW, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	err = db.AddQuota("a", 20)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.SetBlob(w, "a", "id-prefix", "id", bytesWriterTo("x"))
	if err != nil {
		t.Fatal(err)
	}
	used, limitted, err := db.GetQuotaUsed("a")
	if err != nil {
		t.Fatal(err)
	}
	if !(used > 0 && used < 15) {
		t.Fatalf("used: %d", used)
	}
	if limitted != 0 {
		t.Fatalf("limitted: %d", limitted)
	}
	left, err := db.GetQuotaLeft("a")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.SetBlob(w, "a", "id-prefix", "id2", bytesWriterTo(strings.Repeat("x", 100)))
	if !errors.Is(err, blobdb.ErrNotEnoughQuota) {
		t.Fatalf("err=%v, expected ErrNotEnoughQuota", err)
	}

	// Usage must not change; limitted must reflect what was left.
	used2, limitted, err := db.GetQuotaUsed("a")
	if err != nil {
		t.Fatal(err)
	}
	if used2 != used {
		t.Fatalf("used changed from %d to %d", used, used2)
	}
	if limitted != left {
		t.Fatalf("got limitted %d expected %d", limitted, left)
	}

	// Limited quota can be reset
	err = db.ClearQuotaLimitted("a")
	if err != nil {
		t.Fatal(err)
	}
	_, limitted, err = db.GetQuotaUsed("a")
	if err != nil {
		t.Fatal(err)
	}
	if limitted != 0 {
		t.Fatalf("limitted=%d after clear, expected 0", limitted)
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}
