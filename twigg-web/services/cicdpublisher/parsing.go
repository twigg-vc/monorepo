package cicdpublisher

import (
	"bytes"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg/tree"
	"sync"
)

// Simple helper to parse a ciJobFile
func (s publisher) parseCiFile(jobFile tree.Tree) (
	jobPayloads []runnerlib.CiJob, jobFileSizeIsOk bool, jobFileIsOk bool, err error) {
	jobFileSizeIsOk = jobFile.Data().Size <= MaxCiFileSize
	if !jobFileSizeIsOk {
		return
	}
	buff := getBuff()
	defer putBuff(buff)
	wt, err := jobFile.GetFile()
	if err != nil {
		return
	}
	_, err = wt.WriteTo(buff)
	if err != nil {
		return
	}
	jobPayloads, jobFileIsOk,
		/*notOkMsg*/ _ = s.pr.ParseCiFile(buff.Bytes())
	return
}

// Simple helper to parse a cdJobFile
func (s publisher) parseCdFile(jobFile tree.Tree) (
	jobPayloads []runnerlib.CdJob, jobFileSizeIsOk bool, jobFileIsOk bool, err error) {
	jobFileSizeIsOk = jobFile.Data().Size <= MaxCdFileSize
	if !jobFileSizeIsOk {
		return
	}
	buff := getBuff()
	defer putBuff(buff)
	wt, err := jobFile.GetFile()
	if err != nil {
		return
	}
	_, err = wt.WriteTo(buff)
	if err != nil {
		return
	}
	jobPayloads, jobFileIsOk,
		/*notOkMsg*/ _ = s.pr.ParseCdFile(buff.Bytes())
	return
}

var buffPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(nil)
	},
}

func getBuff() *bytes.Buffer {
	b := buffPool.Get().(*bytes.Buffer)
	b.Reset()
	return b
}

func putBuff(b *bytes.Buffer) {
	buffPool.Put(b)
}
