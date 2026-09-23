package blobdb

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"monorepo/base/limitwriter"
	"monorepo/data/deltastream"
	"sync"
	"time"
)

const maxConsecutiveDeltaEncoded = 10

var _ int = func() int {
	// Call a function at compile time just to check the
	// value of maxConsecutiveDeltaEncoded
	if maxConsecutiveDeltaEncoded < 2 {
		panic(
			fmt.Sprintf("maxConsecutiveDeltaEncoded must be >= 2, got %d",
				maxConsecutiveDeltaEncoded))
	}
	return 0
}()

// Private implementation just to enforce using the constructor
type db struct {
	log          AppendLog
	q            QuotaDb
	m            MetadataDb
	enforceQuota bool
}

func (db db) GrabBlobVersion(writeCtx context.Context,
	idPrefix, id string) (Version, error) {
	return db.m.GrabMetadataVersion(writeCtx, idPrefix, id)
}

// returns either the underlying log or a limitWriter to that log
func (db db) getSetBlobDestination(quotaOwner string) (io.Writer, error) {
	var dest io.Writer
	dest = db.log
	if db.enforceQuota {
		var bytesLeft int64
		bytesLeft, err := db.q.GetQuotaLeft(quotaOwner)
		if err != nil {
			return nil, err
		}
		dest = limitwriter.New(db.log, bytesLeft)
	}
	return dest, nil
}

func (db db) getDeltaEncodingBase(writeCtx context.Context,
	idPrefix, id string) (
	deltaEncodingBaseR io.Reader, closeDeltaEncodingBaseR func(), deltaEncodingBaseV Version,
	nextDistanceToNonDelta int64, err error) {
	// Try reading the parent. Reset error on parentNotFound errors
	parentM, parentNotFound, err := db.m.GetLatestMetadata(writeCtx, idPrefix, id)
	if err != nil && !parentNotFound {
		return
	}
	err = nil

	// We only use delta encoding if:
	// There is a previous version &&
	// the previous version is not deleted &&
	// the previous version is not a too long chain of consecutive deltas
	hasParent := !parentNotFound
	if hasParent && !parentM.IsDeleted && parentM.DistanceToNonDelta < maxConsecutiveDeltaEncoded {
		deltaEncodingBaseR, closeDeltaEncodingBaseR, err = db.getReader(writeCtx, parentM)
		if err != nil {
			return
		}
		deltaEncodingBaseV = parentM.Version
		nextDistanceToNonDelta = parentM.DistanceToNonDelta + 1
		return
	}

	deltaEncodingBaseR = nil
	closeDeltaEncodingBaseR = func() {}
	deltaEncodingBaseV = 0
	nextDistanceToNonDelta = 0
	err = nil
	return
}

func (db db) SetBlobVersion(writeCtx context.Context,
	quotaOwner string, idPrefix, id string, v Version, wt io.WriterTo) (err error) {

	offset, err := db.log.Size()
	if err != nil {
		return
	}
	dest, err := db.getSetBlobDestination(quotaOwner)
	if err != nil {
		return
	}

	deltaEncodingBaseR, closeDeltaEncodingBaseR, deltaEncodingBaseV,
		nextDistanceToNonDelta, err := db.getDeltaEncodingBase(writeCtx, idPrefix, id)
	if err != nil {
		return
	}
	defer closeDeltaEncodingBaseR()

	destWriteCounter := writeCounter{w: dest, n: 0}
	compressor, closeCompressor := deltastream.GetCompressor(deltaEncodingBaseR,
		&destWriteCounter)
	nWritten, err := wt.WriteTo(compressor)
	if err != nil && !errors.Is(err, limitwriter.ErrNotEnoughQuota) {
		closeCompressor()
		return
	}

	gotQuotaLimited := errors.Is(err, limitwriter.ErrNotEnoughQuota)
	err = closeCompressor()
	if err != nil && !errors.Is(err, limitwriter.ErrNotEnoughQuota) {
		return
	}
	gotQuotaLimited = gotQuotaLimited || errors.Is(err, limitwriter.ErrNotEnoughQuota)
	compressedSize := destWriteCounter.n
	if gotQuotaLimited {
		_ = db.q.IncreaseQuotaLimittedBytes(quotaOwner, compressedSize)
		err = ErrNotEnoughQuota
		return
	}
	err = db.m.SetMetadataVersion(writeCtx, BlobData{
		IdPrefix:             idPrefix,
		Id:                   id,
		Version:              v,
		Size:                 nWritten,
		CompressedSize:       compressedSize,
		SavedAt:              time.Now(),
		IsDeleted:            false,
		QuotaOwner:           quotaOwner,
		Datastrip:            db.log.Name(),
		Offset:               offset,
		DistanceToNonDelta:   nextDistanceToNonDelta,
		Encoding:             compressor.Data().Method,
		HasDeltaEncodingBase: deltaEncodingBaseR != nil,
		DeltaEncodingBase:    deltaEncodingBaseV,
	})
	if err != nil {
		return
	}

	err = db.q.IncreaseSuccessfullBytes(quotaOwner, compressedSize)
	if err != nil {
		err = fmt.Errorf("failed to update bytes used: %s", err)
		return
	}
	return
}

func (db db) GetBlob(readCtx context.Context, idPrefix, id string) (
	BlobData, io.Reader, func(), error) {
	m, isNotFoundErr, err := db.m.GetLatestMetadata(readCtx, idPrefix, id)
	if isNotFoundErr {
		return BlobData{}, nil, func() {}, ErrNotFound
	}
	if err != nil {
		return BlobData{}, nil, func() {}, err
	}
	if m.IsDeleted {
		return BlobData{}, nil, func() {}, ErrNotFound
	}
	r, closeR, err := db.getReader(readCtx, m)
	return m, r, closeR, err
}

func (db db) GetBlobVersion(readCtx context.Context, idPrefix, id string, v Version) (
	BlobData, io.Reader, func(), error) {
	m, isNotFoundErr, err := db.m.GetMetadataByVersion(readCtx, idPrefix, id, v)
	if isNotFoundErr {
		return BlobData{}, nil, func() {}, ErrNotFound
	}
	if err != nil {
		return BlobData{}, nil, func() {}, err
	}
	r, closeR, err := db.getReader(readCtx, m)
	return m, r, closeR, err
}

func (db db) getReader(readCtx context.Context, m BlobData) (io.Reader, func(), error) {
	sectionReader := io.NewSectionReader(db.log, m.Offset, m.CompressedSize)
	bufferedSectionReader := getBufferedReader(sectionReader)
	closeBuffered := func() {
		putBufferedReader(bufferedSectionReader)
	}
	var err error
	var parentM BlobData
	var parentReader io.Reader
	closeParentReader := func() {}
	if m.Encoding != deltastream.CompressionMethodSpeedFlate {
		// Delta encoded blobs need the parent version to be decompressed
		var parentVersion Version
		if m.HasDeltaEncodingBase {
			parentVersion = m.DeltaEncodingBase
		} else {
			parentVersion = m.Version - 1
		}
		var isNotFoundErr bool
		parentM, isNotFoundErr, err = db.m.GetMetadataByVersion(readCtx,
			m.IdPrefix, m.Id, parentVersion)
		if err != nil {
			if isNotFoundErr {
				err = ErrNotFound
			}
			closeBuffered()
			return nil, func() {}, err
		}
		parentReader, closeParentReader, err = db.getReader(readCtx, parentM)
		if err != nil {
			if closeParentReader != nil {
				closeParentReader()
			}
			closeBuffered()
			return nil, func() {}, err
		}
	}
	r, closeDecomp := deltastream.GetDecompressor(parentReader,
		bufferedSectionReader, m.Encoding)
	closeAll := func() {
		closeDecomp()
		closeParentReader()
		closeBuffered()
	}
	return r, closeAll, nil
}

type writeCounter struct {
	w io.Writer
	n int64
}

func (wc *writeCounter) Write(p []byte) (int, error) {
	n, err := wc.w.Write(p)
	wc.n += int64(n)
	return n, err
}

var bufferedReaderPool = sync.Pool{
	New: func() any {
		return bufio.NewReaderSize(nil, 512*1024) // 512KB
	},
}

func getBufferedReader(r io.Reader) *bufio.Reader {
	br := bufferedReaderPool.Get().(*bufio.Reader)
	br.Reset(r)
	return br
}

func putBufferedReader(br *bufio.Reader) {
	// Clear source to avoid holding references
	br.Reset(nil)
	bufferedReaderPool.Put(br)
}
