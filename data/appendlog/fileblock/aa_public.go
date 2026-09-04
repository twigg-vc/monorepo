package fileblock

type ReadBlock struct{ r *readBlock }

func NewFileReadBlock(filePath string) (ReadBlock, func(), error) {
	r, closeFunc, err := newRead(filePath)
	return ReadBlock{r}, closeFunc, err
}
func (b ReadBlock) ReadAt(p []byte, off int64) (int, error) { return b.r.ReadAt(p, off) }
func (b ReadBlock) Size() (int64, error)                    { return b.r.Size() }

type WriteBlock struct{ w *writeBlock }

func NewFileWriteBlock(filePath string, maxFileSize int64, truncate bool) (WriteBlock, func() error, error) {
	w, closeFunc, err := newWrite(filePath, maxFileSize, truncate)
	return WriteBlock{w}, closeFunc, err
}
func (b WriteBlock) Write(p []byte) (int, error)             { return b.w.Write(p) }
func (b WriteBlock) Sync() error                             { return b.w.Sync() }
func (b WriteBlock) ReadAt(p []byte, off int64) (int, error) { return b.w.ReadAt(p, off) }
func (b WriteBlock) Size() (int64, error)                    { return b.w.Size() }
func (b WriteBlock) SizeLeft() (int64, error)                { return b.w.SizeLeft() }
func (b WriteBlock) Trucate() error                          { return b.w.Trucate() }
