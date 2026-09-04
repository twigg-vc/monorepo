package clidb

import (
	"context"
	"errors"
	"monorepo/data/blobdb"
	"monorepo/twigg/clistate"
)

const cliStateBlobIdPrefix = "clistate-blob"

func (db cliDb) GetCliState(ctx context.Context) (st clistate.State, isNotFoundErr bool, err error) {
	_, r, closeR, err := db.blobs.GetBlob(ctx, cliStateBlobIdPrefix, "")
	if errors.Is(err, blobdb.ErrNotFound) {
		closeR()
		err = ErrNotFound
		isNotFoundErr = true
		return
	}
	if err != nil {
		closeR()
		return
	}
	st, err = readIntoStruct[clistate.State](r, closeR)
	return
}

func (db cliDb) SetCliState(ctx context.Context, st clistate.State) error {
	const quotaOwner = ""
	_, err := db.setBlob(ctx, quotaOwner, cliStateBlobIdPrefix, "", structWriterTo(st))
	return err
}
