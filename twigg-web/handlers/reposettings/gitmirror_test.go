package reposettings

import (
	"context"
	"fmt"
	"monorepo/twigg-runner/runnerlib"
	reposervice "monorepo/twigg-web/services/repo"
	"monorepo/twigg-web/services/sign"
	"monorepo/twigg-web/services/twiggtoken"
	"monorepo/twigg/cli"
	"monorepo/twigg/server"
	"reflect"
	"strings"
	"testing"
)

// repoId of the server built by server.NewTestServer
const testServerRepoId = 1

const testCommitMsg = "test commit"

type fakeTrackClient struct {
	nCalls     int
	jobId      string
	jobPayload runnerlib.JobPayload
}

func (f *fakeTrackClient) PutSkipWebhook(jobId string, jobPayload runnerlib.JobPayload) error {
	f.nCalls += 1
	f.jobId = jobId
	f.jobPayload = jobPayload
	return nil
}

// Builds a handler over a server with one submitted commit and runs the
// git mirror queue handler on it.
func runQueuePushToGitMirror(t *testing.T) (
	testSrv server.TestServer, track *fakeTrackClient, signer sign.Signer) {

	const testApiKey = "key"
	testSrv = server.NewTestServer(testApiKey, t)
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(testSrv.RootUrl())
	tw.Run("init")
	tw.Run("server", testSrv.ServerPath())
	tw.Run("key", testApiKey)
	tw.WriteFile("d.txt", "ddd")
	tw.Run("commit", testCommitMsg)
	tw.Run("push")
	testSrv.Submit(1)

	signer = sign.NewSigner([]byte("test-signer-secret"))
	track = &fakeTrackClient{}
	h := handler{
		db: mockRepoSettingsDb{
			beginRead: func() (context.Context, func(), error) {
				return nil, func() {}, nil
			},
		},
		repoS: mockRepoSettingsRepoService{
			getServerByRepoId: func(repoId uint64) (server.Server, error) {
				if repoId != testServerRepoId {
					return nil, fmt.Errorf("unexpected repoId %d", repoId)
				}
				return testSrv.GetServer(), nil
			},
		},
		track:  track,
		signer: signer,
	}

	_, payload, err := pushToGitMirrorPayload(testServerRepoId,
		"https://token@github.com/user/repo.git")
	if err != nil {
		t.Fatalf("failed to build payload: %v", err)
	}
	err = h.handleQueuePushToGitMirror(payload)
	if err != nil {
		t.Fatalf("handleQueuePushToGitMirror failed: %v", err)
	}
	if track.nCalls != 1 {
		t.Fatalf("expected 1 put, got %d", track.nCalls)
	}
	return testSrv, track, signer
}

func TestQueuePushToGitMirrorPutsJobSteps(t *testing.T) {
	testSrv, track, _ := runQueuePushToGitMirror(t)
	top := testSrv.Top()

	expectedJobId := fmt.Sprintf("push-repo-%d-c%d", testServerRepoId, top.L)
	if track.jobId != expectedJobId {
		t.Fatalf("expected jobId %q, got %q", expectedJobId, track.jobId)
	}

	pl := track.jobPayload
	if pl.ImageName != runnerlib.GitMirrorImage {
		t.Fatalf("expected image %q, got %q", runnerlib.GitMirrorImage, pl.ImageName)
	}
	if pl.Token == "" {
		t.Fatal("expected the payload to carry a token")
	}

	expectedSteps := []runnerlib.JobStep{
		{Run: "tw init"},
		{Run: fmt.Sprintf("tw key %s", pl.Token)},
		{Run: fmt.Sprintf("tw server %d/%d", testServerRepoId, testServerRepoId)},
		{Run: "tw pull top"},

		{Run: "git init -q"},
		{Run: "git config user.name Twigg"},
		{Run: "git config user.email twigg@twigg.vc"},
		{Run: "echo .twigg >> .git/info/exclude"},
		{Run: `git remote add origin "$GIT_MIRROR_SECRET_URL"`,
			Secrets: []string{reposervice.GitMirrorUrlSecretName}},
		{Run: reuseMirrorTwiggBranchOrCreateItStep},
		{Run: "git add -A"},
		{Run: gitMirrorCommitStep,
			Env: map[string]string{gitMirrorCommitMsgEnvVar: testCommitMsg}},
		{Run: "git push -q origin twigg"},
	}
	if len(pl.Steps) != len(expectedSteps) {
		t.Fatalf("expected %d steps, got %d", len(expectedSteps), len(pl.Steps))
	}
	// Compared one by one so a failure points at a single step. The token is
	// long enough to make a whole-slice diff unreadable.
	for i := range expectedSteps {
		if !reflect.DeepEqual(pl.Steps[i], expectedSteps[i]) {
			t.Fatalf("unexpected Steps[%d].\ngot:  %#v\nwant: %#v",
				i, pl.Steps[i], expectedSteps[i])
		}
	}
}

func TestQueuePushToGitMirrorTokenGrantsPullAndSecret(t *testing.T) {
	testSrv, track, signer := runQueuePushToGitMirror(t)
	top := testSrv.Top()

	token, isExpiredErr, err := twiggtoken.ParseToken(track.jobPayload.Token, signer)
	if isExpiredErr {
		t.Fatal("token is already expired")
	}
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}
	if token.RepoId != testServerRepoId {
		t.Fatalf("expected token repoId %d, got %d",
			testServerRepoId, token.RepoId)
	}
	if token.CommitServerId != top.ServerL {
		t.Fatalf("expected token commit %d, got %d",
			top.ServerL, token.CommitServerId)
	}
	if token.CommitVersion != top.ServerV {
		t.Fatalf("expected token commit version %d, got %d",
			top.ServerV, token.CommitVersion)
	}
	if !token.Supports(twiggtoken.TokenActionPull, "") {
		t.Fatal("expected the token to support pull")
	}
	// The runner needs the mirror url, which is stored as a repo secret
	if !token.Supports(twiggtoken.TokenActionGetSecret,
		reposervice.GitMirrorUrlSecretName) {
		t.Fatalf("expected the token to support getting %q",
			reposervice.GitMirrorUrlSecretName)
	}
}

// The mirror url is a secret and the commit message is written by users;
// neither may end up inside the script, which sh would then interpret.
func TestQueuePushToGitMirrorScriptHasNoInterpolatedData(t *testing.T) {
	_, track, _ := runQueuePushToGitMirror(t)

	script := track.jobPayload.Steps[len(track.jobPayload.Steps)-1].Run
	for _, mustNotContain := range []string{
		"github.com", testCommitMsg, "Twigg mirror c/",
	} {
		if strings.Contains(script, mustNotContain) {
			t.Fatalf("script must not contain %q", mustNotContain)
		}
	}
}
