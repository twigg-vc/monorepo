package cicdparser

import "monorepo/twigg-runner/runnerlib"

type CiCdFileParser struct{}

func (c CiCdFileParser) ParseCiFile(ciFile []byte) (payloads []runnerlib.CiJob, ok bool, notOkMsg string) {
	return runnerlib.ParseCiJobs(ciFile)
}

func (c CiCdFileParser) ParseCdFile(cdFile []byte) (payloads []runnerlib.CdJob, ok bool, notOkMsg string) {
	return runnerlib.ParseCdJobs(cdFile)
}