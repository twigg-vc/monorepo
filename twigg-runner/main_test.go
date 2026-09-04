//go:build linux
// +build linux

package main

import (
	"bytes"
	"encoding/json"
	"monorepo/twigg-runner/runnerlib"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSayHi(t *testing.T) {
	payload := runnerlib.JobPayload{
		Name: "say hi",
		Steps: []runnerlib.JobStep{
			{
				Run: "echo hi",
			},
		},
		TimeoutMilliSeconds: 5000,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	jobFile := bytes.NewBuffer(b)
	stdOut := bytes.NewBuffer(nil)
	stdErr := bytes.NewBuffer(nil)
	code := Run(jobFile, stdOut, stdErr)
	if code != 0 {
		t.Fatalf("exit code: %d", code)
	}

	stdErr_ := stdErr.String()
	if stdErr_ != "" {
		t.Fatalf("unexpected stdErr: %q", stdErr_)
	}
	stdOut_ := stdOut.String()
	if !strings.Contains(stdOut_, "hi\n") {
		t.Fatalf("unexpected stdOut_: %q", stdOut_)
	}
}

func TestRunFromDir(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	const subdirName = "test-subdir"
	err = os.Mkdir(filepath.Join(wd, subdirName), os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.RemoveAll(filepath.Join(wd, subdirName))
	})
	err = os.WriteFile(filepath.Join(wd, subdirName, "a.txt"), []byte("abc"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	payload := runnerlib.JobPayload{
		Name: "hello and ls",
		Steps: []runnerlib.JobStep{
			{
				Run: "echo hello",
			},
			{
				Dir: subdirName,
				Run: "ls",
			},
		},
		TimeoutMilliSeconds: 2000,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	jobFile := bytes.NewBuffer(b)
	out := bytes.NewBuffer(nil)
	code := Run(jobFile, out, out)
	if code != 0 {
		t.Fatalf("exit code: %d", code)
	}

	outString := out.String()
	if !strings.Contains(outString, "a.txt") {
		t.Fatalf("outString doesnt contain a.txt: %s", outString)
	}
}
