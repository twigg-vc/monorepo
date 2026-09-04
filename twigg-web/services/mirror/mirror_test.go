package mirror

import (
	"errors"
	"fmt"
	"monorepo/twigg/cli"
	"monorepo/twigg/server"
	"os"
	"strings"
	"testing"
)

func TestPush(t *testing.T) {
	// This test should be disabled most of the times bc it acually posts to gh
	const runMirrorTest = false
	const githubToken = "github_pat_ CHANGE ME"
	const githubUser = "myuser_ CHANGE ME"
	const githubRepo = "CHANGE_ME.git"
	gitUrl := fmt.Sprintf("https://%s@github.com/%s/%s", githubToken, githubUser, githubRepo)
	if !runMirrorTest {
		return
	}

	const testApiKey = "key"
	testServer := server.NewTestServer(testApiKey, t)
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(testServer.RootUrl())
	tw.Run("init")
	tw.Run("server", testServer.ServerPath())
	tw.Run("key", testApiKey)
	tw.WriteFile("d.txt", "ddd")
	tw.Run("commit", "test commit 6")
	tw.Run("push")
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	testServer.Submit(1)
	emptyDirAbsPath := wd + "/test-server-mirror-wd"
	os.RemoveAll(emptyDirAbsPath)
	t.Cleanup(func() { os.RemoveAll(emptyDirAbsPath) })
	mirrorService, err := New(emptyDirAbsPath)
	if err != nil {
		t.Fatal(err)
	}

	w, closeW, _ := testServer.BeginWrite()
	defer closeW()
	serverRead := testServer.BindR(w)

	const maxWorkdirSize = 2000
	err = mirrorService.PushTopCommit(
		serverRead, testServer.GetServer(), gitUrl, maxWorkdirSize)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCantPushIfTooBig(t *testing.T) {
	const testApiKey = "key"
	testServer := server.NewTestServer(testApiKey, t)
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(testServer.RootUrl())
	tw.Run("init")
	tw.Run("server", testServer.ServerPath())
	tw.Run("key", testApiKey)
	tw.WriteFile("d.txt", "ddd")
	tw.Run("commit", "test commit")
	tw.Run("push")
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	testServer.Submit(1)
	emptyDirAbsPath := wd + "/test-server-mirror-wd"
	os.RemoveAll(emptyDirAbsPath)
	t.Cleanup(func() { os.RemoveAll(emptyDirAbsPath) })
	mirrorService, err := New(emptyDirAbsPath)
	if err != nil {
		t.Fatal(err)
	}

	w, closeW, _ := testServer.BeginWrite()
	defer closeW()
	serverRead := testServer.BindR(w)

	// Use a small maxWorkdirSize to force fail the push
	const maxWorkdirSize = 2
	err = mirrorService.PushTopCommit(
		serverRead, testServer.GetServer(), "", maxWorkdirSize)
	if !errors.Is(err, errCommitIsTooBig) {
		t.Fatalf("expected too big err, got %s", err)
	}
}

func TestHasMirrorMarker(t *testing.T) {
	const marker = "Twigg mirror c/7v2"
	tests := []struct {
		name      string
		commitMsg string
		expect    bool
	}{
		{"Marker is the whole message", "Twigg mirror c/7v2", true},
		{"Marker with trailing newline from git log", "Twigg mirror c/7v2\n", true},
		{"Marker as trailer of a custom message", "my change\n\nTwigg mirror c/7v2", true},
		{"Different version", "Twigg mirror c/7v1", false},
		{"Different commit", "Twigg mirror c/8v2", false},
		{"Marker mentioned mid-sentence", "reverts Twigg mirror c/7v2 changes", false},
		{"Empty message", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasMirrorMarker(tt.commitMsg, marker)
			if got != tt.expect {
				t.Errorf("hasMirrorMarker(%q) expected: %v, got: %v", tt.commitMsg, tt.expect, got)
			}
		})
	}
}

func TestIsValidGitCommitMessage(t *testing.T) {
	tests := []struct {
		name              string
		msg               string
		expectIsValidToBe bool
	}{
		// --- Safe Valid Cases ---
		{"Standard Commit Message", "feat: implement secure input validation", true},
		{"Commit message with special characters", "fix: resolve issues with && and ; inline symbols", true},
		{"Multi-word description", "chore: setup automated execution pipeline runner matrix", true},

		// --- Malicious Injection Cases ---
		{"Option Injection Attack Vector", "--cleanup=strip", false},
		{"Option Injection Shortcut Prefix", "-F/etc/passwd", false},
		{"Option Injection White Space Trimmed", "   -m malicious", false},
		{"Null Byte Poisoning String", "commit text\x00 hidden trailing data", false},
		{"Invalid UTF-8 Byte Sequence", "invalid text \xff\xfe bytes", false},
		{"Excessive Payload Size DoS", strings.Repeat("a", 70000), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid, _ := isValidGitCommitMessage(tt.msg)
			if tt.expectIsValidToBe != isValid {
				t.Errorf("IsValidGitCommitMessage(%q) expected: %v, got: %v", tt.msg, tt.expectIsValidToBe, isValid)
			}
		})
	}
}