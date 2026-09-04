package repo

import (
	"io"
	"monorepo/twigg/tree"
	"monorepo/twigg/workdir"
)

func (r repo) Load(v TreeVersion, wd workdir.Workdir, l Read) (err error) {
	di, err := tree.Walk2(wd, r.Root(v, l))
	if err != nil {
		return
	}

	for di.CanGet() {
		var d tree.Diff
		var p string
		var visit tree.VisitStatus
		d = di.GetDiff()
		// Continue while undefined
		if d.Type == tree.DiffTypeUndefined {
			err = di.Next()
			if err != nil {
				return
			}
			continue
		}
		// Skip no changes
		if d.Type == tree.DiffTypeNoChange {
			di.SkipChildrenOnNext()
			err = di.Next()
			if err != nil {
				return
			}
			continue
		}

		p, _, visit, _ = di.Get()
		// Delete stuff that was created
		if d.Type == tree.DiffTypeCreated {
			// Only delete files; not directories.
			// Deleting directories is tricky because they can have "invisible"
			// children (i.e. ignored children)
			if !d.Data.IsDir {
				err = wd.Delete(p)
				if err != nil {
					return
				}
			}
			err = di.Next()
			if err != nil {
				return
			}
			continue
		}

		// If the the tree became a directory in the workdir, we delete the
		// directory and overwrite it with the plain file. Then skip
		// the children of that directory and continue.
		becameDir := false
		if di.CanGetA() && di.CanGetB() {
			_, _, _, aTr := di.GetA()
			_, _, _, bTr := di.GetB()
			becameDir = aTr.Data().IsDir && !bTr.Data().IsDir
		}
		if becameDir {
			err = wd.Delete(p)
			if err != nil {
				return
			}
			var tr tree.Tree
			var pTr string
			pTr, _, _, tr = di.GetB()
			var wt io.WriterTo
			wt, err = tr.GetFile()
			if err != nil {
				return
			}
			err = wd.Write(pTr, tr.Data().IsExectuableFile, wt)
			if err != nil {
				return
			}
			err = wd.SetModTime(
				pTr, tr.Data().LastModifiedUnixMillis)
			if err != nil {
				return
			}
			di.SkipChildrenOnNext()
			err = di.Next()
			if err != nil {
				return
			}
			continue
		}

		// If the the tree became a file in the workdir, we just delete that
		// file and continue. Once the children are processed, the directory
		// will be created.
		// Note that we must check if this is the first visit of the
		// iteration because becameFile/becameDir diffs will visit the same
		// node twice. Only delete on the first visit: on the second visit the
		// directory was already recreated by processing the children,
		// and deleting would throw it away.
		becameFile := false
		if di.CanGetA() && di.CanGetB() {
			_, _, _, aTr := di.GetA()
			_, _, _, bTr := di.GetB()
			becameFile = !aTr.Data().IsDir && bTr.Data().IsDir
		}
		if becameFile {
			if visit == tree.FirstVisit {
				err = wd.Delete(p)
				if err != nil {
					return
				}
			}
			err = di.Next()
			if err != nil {
				return
			}
			continue
		}

		// Recurse into directories for other kinds of diffs of folders
		if d.Data.IsDir {
			err = di.Next()
			if err != nil {
				return
			}
			continue
		}
		// Now for simple changes to files, overwrite the file.
		var tr tree.Tree
		var pTr string
		pTr, _, _, tr = di.GetB()
		if tr.Data().IsSymlink {
			err = wd.WriteSymlink(pTr, tr.Data().SymlinkTarget)
			if err != nil {
				return
			}
		} else {
			var wt io.WriterTo
			wt, err = tr.GetFile()
			if err != nil {
				return
			}
			err = wd.Write(pTr, tr.Data().IsExectuableFile, wt)
			if err != nil {
				return
			}
			err = wd.SetModTime(
				pTr, tr.Data().LastModifiedUnixMillis)
			if err != nil {
				return
			}
		}
		err = di.Next()
		if err != nil {
			return
		}
	}

	return nil
}
