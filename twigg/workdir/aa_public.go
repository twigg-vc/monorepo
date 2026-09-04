package workdir

import (
	"errors"
	"io"
	"monorepo/twigg/tree"
	"testing"
)

const IgnoreFileName = ".gitignore"

// All `path` arguments are use the *correct* format
// (i.e. "forward slashed", because Windows is always objectvelly wrong).
type Workdir interface {
	// Respects the .ignore files.
	// Note that the ignores are populated on demand as they are read.
	// Call `ClearIgnores` to keeping past ignores.
	tree.Root

	// Reset the existing ignores
	ClearIgnores()
	// Manually ignore a pattern
	Ignore(pattern string) error

	// Delete a file/directory from the workdir
	Delete(path string) error
	// Write a file to the workdir.
	Write(path string, isExecutable bool, wt io.WriterTo) error
	// Write a symlink from `path` to `target`
	WriteSymlink(path, target string) error
	// Set the last modified time of a file
	SetModTime(path string, unixMillis int64) error

	// Creates folder with dir name. If already exist return err.
	CreateFolder(dirName string) error

	// Returns true if there is a folder at the current directory.
	// Panics if absolute path is used.
	HasFolder(path string) bool

	// Returns true if the file has conflict markers.
	// Returns `false, nil` if the file doesn't exist.
	FileHasConflict(path string) (bool, error)

	// Equivalent to cd ..
	// Only returns error (ErrCantGoUp) if it can no longer go up.
	GoUp() error

	// Equivalent to cd "dirName'
	// If dir not found, it panics.
	GoDown(dirName string)

	// Returns the absolute path to the workdir
	Path() string

	// Returns true if there are no entries inside the root folder
	IsEmpty() (bool, error)

	// Delete all contents inside the root folder.
	// The root folder itself is not deleted.
	// Be very carefull as this is irreversible.
	Purge() error
}

var (
	ErrCantGoUp = errors.New("cant go up")
	ErrNotFound = errors.New("not found")
)

type TestWorkdir interface {
	Workdir
	WriteFile(path string, content string)
	WriteExecutableFile(path string, content string)
	ReadFile(path string) (content string)
	HasFile(path string) bool
	RootDirHash() [32]byte
}

type Write interface {
	SetWorkdirCache(path string, size int64, modTimeUnixMilli int64, hash [32]byte, isText bool) error
	GetWorkdirCache(path string) (size int64, modTimeUnixMilli int64, hash [32]byte, isText bool, isNotFoundError bool, err error)
}

// It's ok to use `cache=nil, minSizeToCacheFile=0` if you don't want any caching
func New(absPathSlashForward string, cache Write, minSizeToCacheFile int64) (
	wd Workdir, err error) {
	return newWd(absPathSlashForward, cache, minSizeToCacheFile)
}

func NewTest(path string, t testing.TB) TestWorkdir {
	const minSizeToCacheFile = 100 * 1024
	return newTestWd(path, true, minSizeToCacheFile, t)
}

func NewTestWithCustomMinFileToCache(path string, minSizeToCacheFile int64, t testing.TB) TestWorkdir {
	return newTestWd(path, true, minSizeToCacheFile, t)
}

// Same as NewTest, but doesn't automatically cleanup.
func NewTestNoCleanup(path string, t testing.TB) TestWorkdir {
	const minSizeToCacheFile = 100 * 1024
	return newTestWd(path, false, minSizeToCacheFile, t)
}
