//go:build linux
// +build linux

package runnerlib

import (
	"bytes"
	"monorepo/twigg-runner/tse"
	"os"
	"strings"
	"testing"
)

func TestSimpleRun(t *testing.T) {
	stdOut := bytes.NewBuffer(nil)
	stdErr := bytes.NewBuffer(nil)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %s", err)
	}
	rn := NewRunner(wd, stdOut, stdErr)

	exitCode := rn.Run(JobPayload{
		Name: "Log hi and bye",
		Steps: []JobStep{
			{
				Run: "echo hi",
			},
			{
				Run: "echo bye",
			},
		},
	})
	if exitCode != 0 {
		t.Fatalf("got exit code: %d", exitCode)
	}

	stdErr_ := stdErr.String()
	if stdErr_ != "" {
		t.Fatalf("unexpected stdErr: %s", stdErr_)
	}
	stdOut_ := stdOut.String()
	if !strings.Contains(stdOut_, "hi\n") || !strings.Contains(stdOut_, "bye\n") {
		t.Fatalf("unexpected stdOut_: %q", stdOut_)
	}
}

func TestBadRun(t *testing.T) {
	stdOut := bytes.NewBuffer(nil)
	stdErr := bytes.NewBuffer(nil)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %s", err)
	}
	rn := NewRunner(wd, stdOut, stdErr)

	exitCode := rn.Run(JobPayload{
		Name: "Try to run non existing binary",
		Steps: []JobStep{
			{
				Run: "non-existing-binary",
			},
		},
	})
	// Expect an exit code > 0 (which indicates error)
	if exitCode <= 0 {
		t.Fatalf("got exit code: %d", exitCode)
	}
}

func TestScrubberOut(t *testing.T) {
	stdOut := bytes.NewBuffer(nil)
	stdErr := bytes.NewBuffer(nil)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %s", err)
	}
	rn := NewRunner(wd, stdOut, stdErr)

	const secretValue = "..123.."

	exitCode := rn.Run(JobPayload{
		Name: "Log secret",
		Steps: []JobStep{
			{
				Run:     "echo $SECRET",
				Env:     map[string]string{"SECRET": secretValue},
				Secrets: []string{"SECRET"},
			},
			{
				Run: "echo TOP-LEVEL-TOKEN",
			},
			{
				Run: "echo ENV-TOKEN",
				Env: map[string]string{TwiggTokenEnvVarName: "ENV-TOKEN"},
			},
		},
		Token: "TOP-LEVEL-TOKEN",
	})
	if exitCode != 0 {
		t.Fatalf("got exit code: %d", exitCode)
	}

	stdErr_ := stdErr.String()
	if stdErr_ != "" {
		t.Fatalf("unexpected stdErr: %s", stdErr_)
	}
	stdOut_ := stdOut.String()
	if !strings.Contains(stdOut_, string(tse.MaskPlaceholder)) {
		t.Fatalf("stdOut_ does not contain scrubber mask, got: %q", stdOut_)
	}
	if strings.Contains(stdOut_, secretValue) {
		t.Fatalf("stdOut_ contains secret value, got: %q", stdOut_)
	}
	if strings.Contains(stdOut_, "TOP-LEVEL-TOKEN") {
		t.Fatalf("stdOut_ contains JobPayload.Token, got: %q", stdOut_)
	}
	if strings.Contains(stdOut_, "ENV-TOKEN") {
		t.Fatalf("stdOut_ contains TwiggTokenEnvVarName value, got: %q", stdOut_)
	}
}

func TestScrubberErr(t *testing.T) {
	stdOut := bytes.NewBuffer(nil)
	stdErr := bytes.NewBuffer(nil)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %s", err)
	}

	rn := NewRunner(wd, stdOut, stdErr)

	exitCode := rn.Run(JobPayload{
		Name: "Log secret in err",
		Steps: []JobStep{
			{
				Run:     "echo $SECRET 1>&2 && exit 1",
				Env:     map[string]string{"SECRET": "123"},
				Secrets: []string{"SECRET"},
			},
		},
	})

	// should fail
	if exitCode <= 0 {
		t.Fatalf("expected failure, got exit code: %d", exitCode)
	}

	stdErr_ := stdErr.String()
	if !strings.Contains(stdErr_, string(tse.MaskPlaceholder)) {
		t.Fatalf("stderr does not contain scrubber mask, got: %q", stdErr_)
	}
	if strings.Contains(stdErr_, "123") {
		t.Fatalf("stderr contains secret value, got: %q", stdErr_)
	}
}
