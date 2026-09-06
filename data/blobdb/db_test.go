package blobdb_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"monorepo/data/blobdb"
	"testing"
)

// ################################ Test fakes ################################

type memLog struct {
	data []byte
}

func (s *memLog) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(s.data)) {
		return 0, io.EOF
	}
	n := copy(p, s.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
func (s *memLog) Write(p []byte) (int, error) {
	s.data = append(s.data, p...)
	return len(p), nil
}
func (s *memLog) Size() (int64, error) { return int64(len(s.data)), nil }
func (s *memLog) Name() string         { return "mem" }

type memQuota struct {
	left          int64
	successfull   int64
	quotaLimitted int64
}

func (q *memQuota) GetQuotaLeft(quotaOwner string) (int64, error) {
	return q.left, nil
}
func (q *memQuota) IncreaseQuotaLimittedBytes(quotaOwner string, n int64) error {
	q.quotaLimitted += n
	return nil
}
func (q *memQuota) IncreaseSuccessfullBytes(quotaOwner string, n int64) error {
	q.successfull += n
	q.left -= n
	return nil
}

type memMetadata struct {
	rows []blobdb.BlobData
}

func (m *memMetadata) GetLatestMetadata(ctx context.Context, idPrefix string, id string) (blobdb.BlobData, bool, error) {
	for _, row := range m.rows {
		if row.IdPrefix == idPrefix && row.Id == id && row.IsLatest {
			return row, false, nil
		}
	}
	return blobdb.BlobData{}, true, blobdb.ErrNotFound
}
func (m *memMetadata) GetMetadataByVersion(ctx context.Context, idPrefix string, id string, v blobdb.Version) (blobdb.BlobData, bool, error) {
	for _, row := range m.rows {
		if row.IdPrefix == idPrefix && row.Id == id && row.Version == v {
			return row, false, nil
		}
	}
	return blobdb.BlobData{}, true, blobdb.ErrNotFound
}
func (m *memMetadata) InsertMetadata(ctx context.Context, b blobdb.BlobData) error {
	if !b.IsLatest {
		return errors.New("got non latest metadata for insert")
	}
	m.rows = append(m.rows, b)
	return nil
}
func (m *memMetadata) SetMetadataIsLatest(ctx context.Context, idPrefix string, id string, v blobdb.Version, isLatest bool) error {
	for i, row := range m.rows {
		if row.IdPrefix == idPrefix && row.Id == id && row.Version == v {
			m.rows[i].IsLatest = isLatest
		}
	}
	return nil
}

type bytesWriterTo []byte

func (b bytesWriterTo) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(b)
	return int64(n), err
}

// ################################## Tests ##################################

func getNewBlobDb(enforceQuota bool, quotaLeft int64) (blobdb.BlobDb, *memQuota, *memMetadata, *memLog) {
	q := &memQuota{left: quotaLeft}
	m := &memMetadata{}
	s := &memLog{}
	return blobdb.New(s, q, m, enforceQuota), q, m, s
}

func readAll(t *testing.T, r io.Reader, closeR func()) []byte {
	defer closeR()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func Test_SetGetBlob(t *testing.T) {
	db, q, _, _ := getNewBlobDb(false, 0)
	ctx := context.Background()

	_, _, closeR, err := db.GetBlob(ctx, "prefix", "id")
	closeR()
	if !errors.Is(err, blobdb.ErrNotFound) {
		t.Fatalf("err=%v, expected ErrNotFound", err)
	}
	_, _, closeR, err = db.GetBlobVersion(ctx, "prefix", "id", 0)
	closeR()
	if !errors.Is(err, blobdb.ErrNotFound) {
		t.Fatalf("err=%v, expected ErrNotFound", err)
	}

	// First write must create version 0
	v, err := db.SetBlob(ctx, "owner", "prefix", "id", bytesWriterTo("v0-data"))
	if err != nil {
		t.Fatal(err)
	}
	if v != 0 {
		t.Fatalf("v=%d, expected 0", v)
	}

	// Second write must create version 1
	v, err = db.SetBlob(ctx, "owner", "prefix", "id", bytesWriterTo("v1-data"))
	if err != nil {
		t.Fatal(err)
	}
	if v != 1 {
		t.Fatalf("v=%d, expected 1", v)
	}

	// GetBlob must return the latest version
	m, r, closeR, err := db.GetBlob(ctx, "prefix", "id")
	if err != nil {
		closeR()
		t.Fatal(err)
	}
	if m.Version != 1 {
		t.Fatalf("m.Version=%d, expected 1", m.Version)
	}
	data := readAll(t, r, closeR)
	if string(data) != "v1-data" {
		t.Fatalf("data=%q, expected %q", data, "v1-data")
	}

	// GetBlobVersion must return the requested version
	m, r, closeR, err = db.GetBlobVersion(ctx, "prefix", "id", 0)
	if err != nil {
		closeR()
		t.Fatal(err)
	}
	if m.Version != 0 {
		t.Fatalf("m.Version=%d, expected 0", m.Version)
	}
	data = readAll(t, r, closeR)
	if string(data) != "v0-data" {
		t.Fatalf("data=%q, expected %q", data, "v0-data")
	}

	if q.successfull <= 0 {
		t.Fatalf("q.successfull=%d, expected > 0", q.successfull)
	}
	if q.quotaLimitted != 0 {
		t.Fatalf("q.quotaLimitted=%d, expected 0", q.quotaLimitted)
	}
}

// Writes more versions than maxConsecutiveDeltaEncoded to exercise both the
// delta encoded chains and the forced non-delta resets, then reads every
// version back.
func Test_ManyVersions(t *testing.T) {
	db, _, metadata, _ := getNewBlobDb(false, 0)
	ctx := context.Background()

	var contents []string
	for i := range 30 {
		content := fmt.Sprintf("this is the content of version %d."+
			" It shares most of its bytes with the other versions"+
			" so delta encoding kicks in.", i)
		contents = append(contents, content)
		v, err := db.SetBlob(ctx, "owner", "prefix", "id", bytesWriterTo(content))
		if err != nil {
			t.Fatal(err)
		}
		if v != uint64(i) {
			t.Fatalf("v=%d, expected %d", v, i)
		}
	}

	for i := range 30 {
		_, r, closeR, err := db.GetBlobVersion(ctx, "prefix", "id", uint64(i))
		if err != nil {
			closeR()
			t.Fatal(err)
		}
		data := readAll(t, r, closeR)
		if string(data) != contents[i] {
			t.Fatalf("version %d: data=%q, expected %q", i, data, contents[i])
		}
	}

	// Check DeltaEncodingBase
	v0, _, _ := metadata.GetMetadataByVersion(ctx, "prefix", "id", 0)
	if v0.HasDeltaEncodingBase {
		t.Fatal("v0 is delta encoded")
	}
	v1, _, _ := metadata.GetMetadataByVersion(ctx, "prefix", "id", 1)
	if !v1.HasDeltaEncodingBase {
		t.Fatal("v1 not delta encoded")
	}
	if v1.DeltaEncodingBase != 0 {
		t.Fatal("v1 has bad encoding base")
	}
	v2, _, _ := metadata.GetMetadataByVersion(ctx, "prefix", "id", 2)
	if !v2.HasDeltaEncodingBase {
		t.Fatal("v2 not delta encoded")
	}
	if v2.DeltaEncodingBase != 1 {
		t.Fatal("v2 has bad encoding base")
	}
}

func Test_QuotaEnforcement(t *testing.T) {
	db, q, m, _ := getNewBlobDb(true, 5)
	ctx := context.Background()

	// Content big enough to not fit the 5 bytes quota even compressed
	var content []byte
	for i := range 10000 {
		content = append(content, byte(i%251))
	}
	_, err := db.SetBlob(ctx, "owner", "prefix", "id", bytesWriterTo(content))
	if !errors.Is(err, blobdb.ErrNotEnoughQuota) {
		t.Fatalf("err=%v, expected ErrNotEnoughQuota", err)
	}
	if len(m.rows) != 0 {
		t.Fatalf("len(m.rows)=%d, expected no metadata for refused blob", len(m.rows))
	}
	if q.successfull != 0 {
		t.Fatalf("q.successfull=%d, expected 0", q.successfull)
	}

	// With enough quota the same write must succeed
	q.left = 1000000
	v, err := db.SetBlob(ctx, "owner", "prefix", "id", bytesWriterTo(content))
	if err != nil {
		t.Fatal(err)
	}
	if v != 0 {
		t.Fatalf("v=%d, expected 0", v)
	}
	_, r, closeR, err := db.GetBlob(ctx, "prefix", "id")
	if err != nil {
		closeR()
		t.Fatal(err)
	}
	data := readAll(t, r, closeR)
	if string(data) != string(content) {
		t.Fatalf("read back data differs from written content")
	}
}
