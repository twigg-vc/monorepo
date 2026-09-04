package workdir

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"monorepo/base/textdetect"
	"monorepo/twigg/tree"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var enableFileHashCaching = true

// The "Size" of a symlink isn't really something meaningfull nor portable.
// Lstats of symlink return a size that is the length of the target, which might
// vary based on which folder the symlink is written. For that reason, we use
// size = 0.
const symlinkSize = 0

func (w *wd) getFile(path_ string, info os.FileInfo) (data tree.Data, wt io.WriterTo, err error) {
	depth := uint32(strings.Count(path_, "/") + 1)
	cleanAbsPath := w.cleanAbsPath(path_)
	isSymlink := info.Mode()&os.ModeSymlink != 0
	foundCache := false
	var contentHash [32]byte
	var isText bool
	if w.wl != nil && enableFileHashCaching && info.Size() >= w.minSizeToCacheFile {
		var isNotFoundErr bool
		var size int64
		var modTimeUnixMilli int64
		size, modTimeUnixMilli, contentHash, isText, isNotFoundErr, err = w.wl.GetWorkdirCache(path_)
		if isNotFoundErr { // Ignore not-found errs
			err = nil
		}
		if err != nil {
			return
		}
		foundCache = !isNotFoundErr &&
			size == info.Size() &&
			modTimeUnixMilli == info.ModTime().UnixMilli()
	}

	if isSymlink {
		var treeSymlinkTarget string
		treeSymlinkTarget, err = w.resolveTreeSymlinkTarget(cleanAbsPath)
		if err != nil {
			return
		}
		displayString := tree.SymlinkString(treeSymlinkTarget)
		data = tree.NewData(
			filepath.Base(cleanAbsPath),
			depth,
			/*isDir*/ false,
			/*isExecutableFile*/ false,
			/*isText*/ true,
			/*hasConflicts*/ false,
			/*hasChildWithConflicts*/ false,
			/*isSymlink*/ true,
			/*symlinkTarget*/ treeSymlinkTarget,
			/*size*/ symlinkSize,
			info.ModTime().UnixMilli(),
			tree.NewHasher().WriteBool(false).WriteString(displayString).Sum(),
			/*children*/ nil,
		)
		wt = bytes.NewBufferString(displayString)
	} else {
		isExecutable := hasExecutePermission(info)
		wt = newFileWt(cleanAbsPath)
		if !foundCache {
			hasher := tree.NewHasher().WriteBool(isExecutable)
			wrap, textDetect := textdetect.Wrap(hasher)
			_, err = wt.WriteTo(wrap)
			if err != nil {
				return
			}
			contentHash = hasher.Sum()
			isText = textDetect.ProbablyWroteText()
		}
		data = tree.NewData(
			filepath.Base(cleanAbsPath),
			depth,
			/*isDir*/ false,
			isExecutable,
			isText,
			/*hasConflicts*/ false,
			/*hasChildWithConflicts*/ false,
			/*isSymlink*/ false,
			/*symlinkTarget*/ "",
			info.Size(),
			info.ModTime().UnixMilli(),
			contentHash,
			/*children*/ nil,
		)
		if w.wl != nil && enableFileHashCaching && info.Size() >= w.minSizeToCacheFile {
			err = w.wl.SetWorkdirCache(path_, info.Size(),
				info.ModTime().UnixMilli(),
				contentHash, isText)
		}
	}
	return
}

// When creating a tree.Data for symlink files, we simply store what the os
// gives us for the link. We only modify it to turn it into a "forward slashed"
func (w *wd) resolveTreeSymlinkTarget(absPath string) (string, error) {
	osTarget, err := os.Readlink(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read link %s: %w", absPath, err)
	}
	return filepath.ToSlash(osTarget), nil
}

type writerTo struct {
	cleanAbsPathToFile string
}

func newFileWt(cleanAbsPathToFile string) writerTo {
	return writerTo{cleanAbsPathToFile: cleanAbsPathToFile}
}

func (wt writerTo) WriteTo(w io.Writer) (n int64, err error) {
	f, err := os.Open(wt.cleanAbsPathToFile)
	if err != nil {
		return 0, err
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()
	buff := buffPool.Get().(*[]byte)
	defer buffPool.Put(buff)
	n, err = io.CopyBuffer(w, f, *buff)
	return
}

const buffSize = 32 * 1024 // 32 KB, tweak based on profiling
// provides *[]byte
var buffPool = sync.Pool{
	New: func() any {
		b := make([]byte, buffSize)
		return &b
	},
}

func hasExecutePermission(info os.FileInfo) bool {
	return info.Mode()&0o111 != 0
}
