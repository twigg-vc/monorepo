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

func (db db) SetBlob(writeCtx context.Context,
	quotaOwner string, idPrefix, id string, wt io.WriterTo) (v Version, err error) {
	parentM, parentNotFound, err := db.m.GetLatestMetadata(writeCtx, idPrefix, id)
	if err != nil && !parentNotFound {
		return
	}
	err = nil
	hasParent := !parentNotFound
	var parentR io.Reader
	if hasParent {
		v = parentM.Version + 1
	} else {
		v = 0
		// Set to -1 bc then we can always say that the new DistanceToNonDelta
		// is just DistanceToNonDelta + 1 without needing an extra variable
		// for this.
		parentM.DistanceToNonDelta = -1
	}
	if hasParent && !parentM.IsDeleted {
		// Only get the parentReader if the parent is not already too distant
		// from a non-delta encoded. This is done to not have endless chains
		// of delta encoded data. By leaving parentR=nil, we force delta
		// encoding not to be used
		if parentM.DistanceToNonDelta < maxConsecutiveDeltaEncoded {
			var closeParentR func()
			parentR, closeParentR, err = db.getReader(writeCtx, parentM)
			defer closeParentR()
			if err != nil {
				return
			}
		} else {
			// We use parentM.DistanceToNonDelta+1 for the DistanceToNonDelta
			// of this new entry, so we set it to -1 here to use the value 0
			parentM.DistanceToNonDelta = -1
		}
	}

	offset, err := db.log.Size()
	if err != nil {
		return
	}

	var dest io.Writer
	dest = db.log
	if db.enforceQuota {
		var bytesLeft int64
		bytesLeft, err = db.q.GetQuotaLeft(quotaOwner)
		if err != nil {
			return
		}
		dest = limitwriter.New(db.log, bytesLeft)
	}

	destWriteCounter := writeCounter{w: dest, n: 0}
	compressor, closeCompressor := deltastream.GetCompressor(parentR,
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
	err = db.m.InsertMetadata(writeCtx, BlobData{
		IdPrefix:           idPrefix,
		Id:                 id,
		Version:            v,
		Size:               nWritten,
		CompressedSize:     compressedSize,
		SavedAt:            time.Now(),
		IsDeleted:          false,
		QuotaOwner:         quotaOwner,
		IsLatest:           true,
		Datastrip:          db.log.Name(),
		Offset:             offset,
		DistanceToNonDelta: parentM.DistanceToNonDelta + 1,
		Encoding:           compressor.Data().Method,
	})
	if err != nil {
		return
	}

	if hasParent {
		err = db.m.SetMetadata(writeCtx, idPrefix, id, parentM.Version, false)
		if err != nil {
			return
		}
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
		var isNotFoundErr bool
		parentM, isNotFoundErr, err = db.m.GetMetadataByVersion(readCtx,
			m.IdPrefix, m.Id, m.Version-1)
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
