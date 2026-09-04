package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"monorepo/twigg/ansi"
	"monorepo/twigg/cli/links"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type helper struct {
	t             testing.TB
	out           *testOut
	in            *bytes.Buffer
	originalPath  string
	path          string
	serverRootUrl string
}

func newTestHelperAt(testDirName string, cleanup bool, t testing.TB) *helper {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %v", err)
	}

	testDir := filepath.Join(originalDir, testDirName)
	h := &helper{
		out:           newTestOut(),
		in:            bytes.NewBuffer(nil),
		t:             t,
		originalPath:  originalDir,
		path:          testDir,
		serverRootUrl: "!!YOU FORGOT TO CALL SetServerRootUrl!!",
	}
	if cleanup {
		os.RemoveAll(testDir)
		t.Cleanup(func() {
			os.RemoveAll(testDir)
		})
	}
	os.MkdirAll(testDir, 0755)

	return h
}

func (h *helper) SetServerRootUrl(rootUrl string) {
	h.serverRootUrl = rootUrl
}

func (h *helper) Run(args ...string) {
	h.out.Reset()
	os.Args = append([]string{"tw.bin"}, args...)

	err := os.Chdir(h.path)
	if err != nil {
		h.t.Fatalf("could not cd to test directory: %v", err)
	}
	defer func() {
		err := os.Chdir(h.originalPath)
		if err != nil {
			h.t.Fatalf("could not cd to back to original directory: %v", err)
		}
	}()

	Run(&h.originalPath, h.serverRootUrl, h.out, h.in)
}

func (h *helper) Out() string {
	return h.out.String()
}

func (h *helper) PrepareInput(input string) {
	h.in.WriteString(input)
}

func (h *helper) Cd(path string) {
	h.path = h.path + "/" + path
}

func (h helper) WriteFile(filename string, content string) {
	os.Remove(h.pathTo(filename))
	os.MkdirAll(filepath.Dir(h.pathTo(filename)), 0755)
	err := os.WriteFile(h.pathTo(filename), []byte(content), 0644)
	if err != nil {
		h.t.Fatalf("filed to write file %s: %s", filename, err)
	}
}
func (h helper) WriteExecutable(filename string, content string) {
	os.Remove(h.pathTo(filename))
	os.MkdirAll(filepath.Dir(h.pathTo(filename)), 0755)
	err := os.WriteFile(h.pathTo(filename), []byte(content), 0755)
	if err != nil {
		h.t.Fatalf("filed to write executable file %s: %s", filename, err)
	}
}
func (h helper) WriteSymlink(filename, target string) {
	absPathToFile := h.pathTo(filename)
	err := os.RemoveAll(absPathToFile)
	if err != nil {
		h.t.Fatalf("failed to delete %s: %s", filename, err)
	}
	err = os.Symlink(target, absPathToFile)
	if err != nil {
		h.t.Fatalf("failed to symlink %s -> %s: %s", filename, target, err)
	}
}
func (h helper) WriteSymlinkWithAbsPath(filename, absTargetPath string) {
	absPathToFile := h.pathTo(filename)
	err := os.RemoveAll(absPathToFile)
	if err != nil {
		h.t.Fatalf("failed to delete %s: %s", filename, err)
	}
	err = os.Symlink(absTargetPath, absPathToFile)
	if err != nil {
		h.t.Fatalf("failed to symlink %s -> %s: %s", filename, absTargetPath, err)
	}
}
func (h helper) DeleteFolder(name string) {
	err := os.RemoveAll(h.pathTo(name))
	if err != nil {
		h.t.Fatalf("filed to delete folder %s: %s", name, err)
	}
}
func (h helper) DeleteFile(filename string) {
	err := os.RemoveAll(h.pathTo(filename))
	if err != nil {
		h.t.Fatalf("filed to delete file %s: %s", filename, err)
	}
}
func (h helper) CheckOutContains(substring string) {
	h.t.Helper()
	if !h.out.Contains(substring) {
		h.t.Fatalf("CheckOutContains failed.\nOut doesnt't contain %q:\n%s",
			substring,
			h.out.String(),
		)
	}
}
func (h helper) CheckOutContainsN(substring string, nTimes int) {
	h.t.Helper()
	if !h.out.ContainsN(substring, nTimes) {
		h.t.Fatalf("CheckOutContainsN failed.\nOut doesnt't contain %q %d times:\n%s",
			substring, nTimes, h.out.String())
	}
}
func (h helper) CheckOutDoesntContain(substring string) {
	h.t.Helper()
	if h.out.Contains(substring) {
		h.t.Fatalf("CheckOutDoesntContain failed.\nOut contains %q:\n%s",
			substring, h.out.String())
	}
}
func (h helper) CheckOutIsEmpty() {
	h.t.Helper()
	if !h.out.isEmpty {
		h.t.Fatal("CheckOutIsEmpty failed.\nOut is not empty:\n" + h.out.String())
	}
}
func (h helper) CheckOutJsonDiffFile(expected []JsonDiffFile) {
	h.t.Helper()
	var d JsonDiff
	err := json.Unmarshal([]byte(h.Out()), &d)
	if err != nil {
		h.t.Fatalf("bad json output: %s", err)
	}
	if !reflect.DeepEqual(d.Files, expected) {
		h.t.Fatalf("bad json files:\n%+v\nexpected:\n%+v", d.Files, expected)
	}
}
func (h helper) CheckFile(filename string, expected string) {
	h.t.Helper()
	b, err := os.ReadFile(h.pathTo(filename))
	if err != nil {
		h.t.Fatalf("filed to read file %s: %s", filename, err)
	}
	contents := string(b)
	if len(contents) != len(expected) {
		h.t.Fatalf("CheckFile failed.\nExpected len(%v)=%v\nGot %v\n", filename, len(expected), len(contents))
	}
	if contents != expected {
		h.t.Fatalf("CheckFile failed.\nExpected %s=%s\nGot %s\n", filename, expected, contents)
	}
}
func (h helper) CheckFileLine(filename string, lineIndex int, expected string) {
	h.t.Helper()
	b, err := os.ReadFile(h.pathTo(filename))
	if err != nil {
		h.t.Fatalf("filed to read file %s: %s", filename, err)
	}
	lines := strings.Split(string(b), "\n")
	for i, line := range lines {
		if i == lineIndex && line == expected {
			return
		}
		if i == lineIndex && line != expected {
			h.t.Fatalf("CheckFileHasLine failed.\nExpected %q got %q\n", expected, line)
		}
	}
	h.t.Fatalf("CheckFileHasLine failed.\nCould not find %q in %q\n", expected, string(b))
}
func (h helper) CheckSymlink(filename string, expectedTarget string) {
	h.t.Helper()
	path := h.pathTo(filename)
	info, err := os.Lstat(path)
	if err != nil {
		h.t.Fatalf("failed to stat %s: %v", filename, err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		h.t.Fatalf("%s is not a symlink", filename)
	}

	target, err := os.Readlink(path)
	if err != nil {
		h.t.Fatalf("failed to read symlink target: %v", err)
	}

	if filepath.IsAbs(target) {
		expectedTarget = h.pathTo(expectedTarget)
	}
	if target != expectedTarget {
		h.t.Fatalf("symlink target mismatch: expected %s, got %s", expectedTarget, target)
	}
}
func (h helper) CheckHasNoFile(filename string) {
	h.t.Helper()
	_, err := os.Stat(h.pathTo(filename))
	if err != nil && !os.IsNotExist(err) {
		h.t.Fatalf("CheckHasNoFile got error: %s", err)
	}
	exists := !os.IsNotExist(err)
	if exists {
		h.t.Fatalf("CheckHasNoFile failed.\nWorking directory has %s", filename)
	}
}

func (h helper) CheckDirectoryExists(relativePath string) {
	h.t.Helper()
	info, err := os.Stat(h.pathTo(relativePath))
	if err != nil && !os.IsNotExist(err) {
		h.t.Fatalf("CheckDirectoryExists got error: %s", err)
	}
	exists := !os.IsNotExist(err)
	if !exists {
		h.t.Fatalf("CheckDirectoryExists failed.\nWorking directory doesn't contain %s", relativePath)
	}
	if !info.IsDir() {
		h.t.Fatalf("CheckDirectoryExists failed.\n%s is not a directory", relativePath)
	}
}
func (h helper) CheckDirectoryDoesntExist(relativePath string) {
	h.t.Helper()
	_, err := os.Stat(h.pathTo(relativePath))
	if err != nil && !os.IsNotExist(err) {
		h.t.Fatalf("CheckDirectoryDoesntExist got error: %s", err)
	}
	exists := !os.IsNotExist(err)
	if exists {
		h.t.Fatalf("CheckDirectoryDoesntExist failed.\nWorking directory contains %s", relativePath)
	}
}

const defaultNumberOfCommitsToLog = 10

func (h helper) LogAll() []LoggedCommit {
	return h.runLog(true, defaultNumberOfCommitsToLog)
}
func (h helper) Log() []LoggedCommit {
	return h.runLog(false, defaultNumberOfCommitsToLog)
}
func (h helper) LogN(numberOfCommitsToLog int) []LoggedCommit {
	return h.runLog(false, numberOfCommitsToLog)
}

// Matches each commit "block"
// * #<id> v<version> <etc>
// <following line>
// Captures the commit id and version
var loggedCommitReg = regexp.MustCompile(
	`(?:@ |\* )#(\d+)v(\d+)[^\n]*\n\S*\s*(.*)`)

// Matches #<id> v<version> c/<serverId> v<serverV>
// captures the serverId and serverV
var loggedCommitServerIdAndVersionReg = regexp.MustCompile(
	`#\d+v\d+\s+c/(\d+)v(\d+)`)

// Matches #<id> v<version> c/<serverId>
// captures the serverId
var loggedCommitServerIdReg = regexp.MustCompile(
	`#\d+v\d+\s+c/(\d+)`)

func (h helper) runLog(all bool, numberOfCommitsToLog int) []LoggedCommit {
	num := strconv.Itoa(numberOfCommitsToLog)
	if all {
		h.Run("log", num, allFlag)
	} else {
		h.Run("log", num)
	}
	results := []LoggedCommit{}
	s := ansi.Remove(h.out.String())
	s = links.RemoveHyperlinks(s)
	// The regex was written considering @ and * for commit markers, so I'll
	// add a panic in case I change those and forget to update the regex.
	// We shouldn't interpolate the marker to create the regex bc that might
	// not always work
	if activeCommitMarker != '@' {
		panic("regex expects activeCommitMarker = @")
	}
	if inactiveCommitMarker != '*' {
		panic("regex expects inactiveCommitMarker = *")
	}
	matches := loggedCommitReg.FindAllStringSubmatch(s, -1)
	for _, match := range matches {
		isActive := strings.Contains(match[0], string(activeCommitMarker))
		idString := match[1]
		id, err := strconv.Atoi(idString)
		if err != nil {
			h.t.Fatal(err)
		}
		v, err := strconv.Atoi(match[2])
		if err != nil {
			h.t.Fatal(err)
		}

		var serverId int
		var serverV int
		hasServerId := false
		hasServerV := false
		serverIdAndVersMatches := loggedCommitServerIdAndVersionReg.FindStringSubmatch(match[0])
		if serverIdAndVersMatches != nil {
			hasServerId = true
			hasServerV = true
			// matches[0] is the full match
			serverId, err = strconv.Atoi(serverIdAndVersMatches[1])
			if err != nil {
				h.t.Fatal(err)
			}
			serverV, err = strconv.Atoi(serverIdAndVersMatches[2])
			if err != nil {
				h.t.Fatal(err)
			}
		}
		serverIdMatches := loggedCommitServerIdReg.FindStringSubmatch(match[0])
		if serverIdMatches != nil {
			hasServerId = true
			// matches[0] is the full match
			serverId, err = strconv.Atoi(serverIdMatches[1])
			if err != nil {
				h.t.Fatal(err)
			}
		}

		isSub := strings.Contains(match[0], submittedSuffix)
		obsoleteReason := ""
		if strings.Contains(match[0], obsoleteByAmendSuffix) {
			obsoleteReason = "amend"
		}
		if strings.Contains(match[0], obsoleteByPullOverwriteSuffix) {
			obsoleteReason = "pull"
		}
		if strings.Contains(match[0], obsoleteByManualRebaseSuffix) {
			obsoleteReason = "manual-rebase"
		}
		if strings.Contains(match[0], obsoleteByAutoRebaseSuffix) {
			obsoleteReason = "auto-rebase"
		}
		if strings.Contains(match[0], obsoleteBySubmitSuffix) {
			obsoleteReason = "submit"
		}
		if strings.Contains(match[0], obsoleteByRestoreSuffixSubstring) {
			obsoleteReason = "restore"
		}
		isObsolete := obsoleteReason != ""

		hasConflicts := strings.Contains(match[0], conflictsSuffix)
		isUploaded := strings.Contains(match[0], pushedSufffix)
		results = append(results, LoggedCommit{
			Id:             id,
			IdString:       idString,
			Version:        v,
			HasServerId:    hasServerId,
			HasServerV:     hasServerV,
			ServerId:       serverId,
			ServerVersion:  serverV,
			IsSubmitted:    isSub,
			IsObsolete:     isObsolete,
			IsActive:       isActive,
			HasConflicts:   hasConflicts,
			IsUploaded:     isUploaded,
			IsNotUploaded:  !isUploaded,
			ObsoleteReason: obsoleteReason,
		})
	}
	return results
}
func (h helper) NonObsoletLog() []LoggedCommit {
	allCommits := h.Log()
	filtered := []LoggedCommit{}
	for _, c := range allCommits {
		if c.IsObsolete {
			continue
		}
		filtered = append(filtered, c)
	}
	return filtered
}
func (h helper) ActiveCommit() LoggedCommit {
	cs := h.Log()
	for _, c := range cs {
		if c.IsActive {
			return c
		}
	}
	h.t.Fatal("active commit not found")
	return LoggedCommit{}
}
func (h helper) checkLog(commits []LoggedCommit, expectedIds ...int) {
	sort.Slice(expectedIds, func(i, j int) bool {
		return expectedIds[i] < expectedIds[j]
	})
	sort.Slice(commits, func(i, j int) bool {
		return commits[i].Id < commits[j].Id
	})
	if len(commits) != len(expectedIds) {
		h.t.Fatalf("expected %d commits, got %d", len(expectedIds), len(commits))
	}
	for i := 0; i < len(commits); i++ {
		if commits[i].Id != expectedIds[i] {
			h.t.Fatalf(
				"expected commit with id %d, got %d",
				commits[i].Id,
				expectedIds[i])
		}
	}
}
func (h helper) CheckLog(expectedIds ...int) {
	h.checkLog(h.Log(), expectedIds...)
}
func (h helper) CheckLogAll(expectedIds ...int) {
	h.checkLog(h.LogAll(), expectedIds...)
}
func (h helper) CheckLogAllVersions(expectedVersions ...IdVersionAndConflict) {
	c := h.LogAll()
	sort.Slice(c, func(i, j int) bool {
		if c[i].Id == c[j].Id {
			return c[i].Version < c[j].Version
		}
		return c[i].Id < c[j].Id
	})
	sort.Slice(expectedVersions, func(i, j int) bool {
		if expectedVersions[i].Id == expectedVersions[j].Id {
			return expectedVersions[i].Version < expectedVersions[j].Version
		}
		return expectedVersions[i].Id < expectedVersions[j].Id
	})
	if len(c) != len(expectedVersions) {
		h.t.Fatalf("expected %d commits, got %d", len(c), len(expectedVersions))
	}
	for i := 0; i < len(c); i++ {
		if c[i].Id != expectedVersions[i].Id {
			h.t.Fatalf(
				"expected commit with id %d, got %d",
				c[i].Id,
				expectedVersions[i].Id)
		}
		if c[i].Version != expectedVersions[i].Version {
			h.t.Fatalf(
				"expected commit with v %d, got %d",
				c[i].Version,
				expectedVersions[i].Version)
		}
		if c[i].HasConflicts != expectedVersions[i].HasConflicts {
			h.t.Fatalf(
				"expected hasConflict %v, got %v",
				c[i].HasConflicts,
				expectedVersions[i].HasConflicts)
		}
	}
}
func (h helper) CheckActiveCommit(arg CheckCommitArg) {
	h.t.Helper()
	a := h.ActiveCommit()
	if a.Id != arg.Id {
		h.t.Fatalf(
			"expected commit with id %d, got %d", arg.Id, a.Id)
	}
	if a.Version != arg.Version {
		h.t.Fatalf(
			"expected commit with version %d, got %d", arg.Version, a.Version)
	}
	if arg.HasServerId != a.HasServerId {
		h.t.Fatalf(
			"expected hasServerId %v, got %v", arg.HasServerId,
			a.HasServerId)
	}
	if arg.HasServerV != a.HasServerV {
		h.t.Fatalf(
			"expected hasServerV %v, got %v", arg.HasServerV,
			a.HasServerV)
	}
	if arg.HasConflicts != a.HasConflicts {
		h.t.Fatalf(
			"expected hasConflict %v, got %v", arg.HasConflicts,
			a.HasConflicts)
	}
	if arg.IsSubmitted != a.IsSubmitted {
		h.t.Fatalf(
			"expected isSubmitted %v, got %v", arg.IsSubmitted,
			a.IsSubmitted)
	}
	if a.ObsoleteReason != arg.ObsoleteReason {
		h.t.Fatalf(
			"expected obsoleteness reason %s, got %s", arg.ObsoleteReason,
			a.ObsoleteReason)
	}
	if !arg.HasServerId {
		return
	}
	if a.ServerId != arg.ServerId {
		h.t.Fatalf(
			"expected commit with server id %d, got %d", arg.ServerId, a.ServerId)
	}
	if a.ServerVersion != arg.ServerV {
		h.t.Fatalf(
			"expected commit with server version %d, got %d", arg.ServerV,
			a.ServerVersion)
	}
}
func (h helper) CheckActiveCommitLocalId(n int) {
	h.t.Helper()
	a := h.ActiveCommit()
	if a.Id != n {
		h.t.Fatalf(
			"expected commit with id %d, got %d", n, a.Id)
	}
}
func (h helper) CheckLogN(number int, expectedIds []int) {
	h.checkLog(h.LogN(number), expectedIds...)
}

func (h helper) pathTo(filename string) string {
	return filepath.Join(h.path, filename)
}

type testOut struct {
	out         *bytes.Buffer
	logToStdOut bool
	isEmpty     bool
}

func newTestOut() *testOut {
	return &testOut{
		out:         bytes.NewBuffer(nil),
		logToStdOut: false,
		isEmpty:     true,
	}
}
func (tl *testOut) Write(p []byte) (int, error) {
	tl.isEmpty = false
	if tl.logToStdOut {
		// When testing, ansi escape codes are not displayed correctly
		fmt.Println(ansi.Remove(string(p)))
	}
	return tl.out.Write(p)
}
func (tl *testOut) Reset() {
	tl.out.Reset()
	tl.isEmpty = true
}

func (tl testOut) String() string {
	return tl.out.String()
}

func (tl testOut) Contains(s string) bool {
	// In some environments, the ansi color codes are not shown (
	// e.g. whenever NO_COLOR env var is set).
	// To make testing portable, the tests strip all ansi codes
	return strings.Contains(ansi.Remove(tl.out.String()), ansi.Remove(s))
}

func (tl testOut) ContainsN(s string, n int) bool {
	// See `Contains` for why we remove the ansi codes
	return strings.Count(ansi.Remove(tl.out.String()), ansi.Remove(s)) == n
}
