package main

import (
	"encoding/json"
	"fmt"
	"io"
	"monorepo/twigg-runner/runnerlib"
	"os"
)

func Run(job io.Reader, rnStdOut io.Writer, rnErrOut io.Writer) (exitCode int) {
	wd, err := os.Getwd()
	if err != nil {
		rnErrOut.Write([]byte(fmt.Sprintf("failed to get workdir: %s", err)))
		exitCode = 1
		return
	}
	var payload runnerlib.JobPayload
	dec := json.NewDecoder(job)
	dec.DisallowUnknownFields()
	err = dec.Decode(&payload)
	if err != nil {
		rnErrOut.Write([]byte(fmt.Sprintf("failed to decode job: %s", err)))
		exitCode = 1
		return
	}
	runner := runnerlib.NewRunner(wd, rnStdOut, rnErrOut)
	exitCode = runner.Run(payload)
	return
}

func main() {
	os.Exit(Run(os.Stdin, os.Stdout, os.Stderr))
}