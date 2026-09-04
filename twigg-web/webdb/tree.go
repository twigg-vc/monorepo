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

func (db webDb) GetLastVersionOfRootTree(ctx context.Context, repoId uint64) (v uint64, isNotFoundErr bool, err error) {
	m, _, closeR, err := db.blobs.GetBlob(ctx, treeDataBlobsIdPrefix, treeDataBlobId(repoId, tree.RootPath))
	closeR()
	if errors.Is(err, blobdb.ErrNotFound) {
		err = ErrNotFound
		isNotFoundErr = true
		return
	}
	if err != nil {
		return
	}
	v = m.Version
	return
}

func (db webDb) SetTreeData(ctx context.Context, quotaOwner string, repoId uint64, treePath string, td treev.TreeDataV) (uint64, error) {
	return db.setBlob(ctx, quotaOwner, treeDataBlobsIdPrefix, treeDataBlobId(repoId, treePath), gobencoding.StructWriterTo(td))
}

func (db webDb) SetTreeBlob(ctx context.Context, quotaOwner string, repoId uint64, treePath string, wt io.WriterTo) (uint64, error) {
	return db.setBlob(ctx, quotaOwner, treeBlobsIdPrefix, treeBlobsId(repoId, treePath), wt)
}
