package webdb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"monorepo/data/blobdb"
	"monorepo/twigg-web/services/gobencoding"
	"monorepo/twigg/tree"
	"monorepo/twigg/treev"
)

const treeDataBlobsIdPrefix = "twigg-internal-tree-data"

func treeDataBlobId(repoId uint64, treePath string) string {
	return fmt.Sprintf("%d-%s", repoId, treePath)
}

const treeBlobsIdPrefix = "twigg-internal-tree-blobs"

// Identical body to treeDataBlobId, kept separate since they're different namespaces.
func treeBlobsId(repoId uint64, treePath string) string {
	return fmt.Sprintf("%d-%s", repoId, treePath)
}

func (db webDb) GetTreeData(ctx context.Context, repoId uint64, treePath string, v uint64) (td treev.TreeDataV, isNotFoundErr bool, err error) {
	_, r, closeR, err := db.blobs.GetBlobVersion(ctx, treeDataBlobsIdPrefix, treeDataBlobId(repoId, treePath), v)
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
	td, err = gobencoding.ReadIntoStruct[treev.TreeDataV](r, closeR)
	return
}

func (db webDb) GetTreeBlob(ctx context.Context, repoId uint64, treePath string, v uint64) (r io.Reader, closeR func(), isNotFoundErr bool, err error) {
	_, r, closeR, err = db.blobs.GetBlobVersion(ctx, treeBlobsIdPrefix, treeBlobsId(repoId, treePath), v)
	if errors.Is(err, blobdb.ErrNotFound) {
		closeR()
		closeR = func() {}
		err = ErrNotFound
		isNotFoundErr = true
		return
	}
	return
}

func (db webDb) GrabRootTreeVersion(ctx context.Context, repoId uint64) (uint64, error) {
	return db.GrabBlobVersion(ctx, treeDataBlobsIdPrefix,
		treeDataBlobId(repoId, tree.RootPath))
}

func (db webDb) SetTreeData(ctx context.Context, quotaOwner string, repoId uint64, treePath string, v uint64, td treev.TreeDataV) error {
	return db.SetBlobVersion(ctx, quotaOwner, treeDataBlobsIdPrefix,
		treeDataBlobId(repoId, treePath), v, gobencoding.StructWriterTo(td))
}

func (db webDb) SetTreeBlob(ctx context.Context, quotaOwner string, repoId uint64, treePath string, v uint64, wt io.WriterTo) error {
	return db.SetBlobVersion(ctx, quotaOwner, treeBlobsIdPrefix,
		treeBlobsId(repoId, treePath), v, wt)
}
