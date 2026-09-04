package appendlog

import (
	"monorepo/data/appendlog/tiered"
)

// Represents an infinite append-only data store
type AppendLog struct {
	al *appendLog
}

// Reads data at a given offset
func (d AppendLog) ReadAt(p []byte, off int64) (n int, err error) {
	return d.al.ReadAt(p, off)
}

// Appends p at the end of the data
func (d AppendLog) Write(p []byte) (n int, err error) {
	return d.al.Write(p)
}

// Sync the file to ensure it's flushed to disk
func (d AppendLog) Sync() error {
	return d.al.Sync()
}

// Returns the total size
func (d AppendLog) Size() (int64, error) {
	return d.al.Size()
}

// Identifies this instance
func (d AppendLog) Name() string {
	return d.al.Name()
}

// Creates an instance in the provided directory.
func New(bp tiered.Provider) AppendLog {
	return AppendLog{al: new(bp)}
}
