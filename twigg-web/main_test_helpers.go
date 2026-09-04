package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"monorepo/base/serverutils"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/trackclient"
	"monorepo/twigg-track/trackserver"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/server"
	"monorepo/twigg-web/services/jobs"
	"monorepo/twigg-web/services/oauthclient"
	"monorepo/twigg-web/services/stripeclient"
	"monorepo/twigg-web/srvconfig"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Represents a "browser" for tests.
// It's really just an http client but that sends cookies like a browser would,
// and has some methods to make requests and test the responses.
type TestBrowser struct {
	t            testing.TB
	serverUrl    string
	currentUrl   string
	lastResponse []byte
	c            *http.Client
	cookieJar    http.CookieJar
}

func NewTestBrowser(serverUrl string, t testing.TB) *TestBrowser {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("failed to get cookie jar %s", err)
	}
	return &TestBrowser{
		t:            t,
		serverUrl:    serverUrl,
		currentUrl:   "",
		lastResponse: []byte{},
		c: &http.Client{
			Jar: jar,
		},
		cookieJar: jar,
	}
}

// Returns the current path the browser is in
func (b TestBrowser) CurrentPath() string {
	return strings.TrimPrefix(b.currentUrl, b.serverUrl)
}

// Returns the current url the browser is in
func (b TestBrowser) CurrentUrl() string {
	return b.currentUrl
}

// Makes a get request to the provided path.
func (b *TestBrowser) Get(path string) {
	b.t.Helper()
	resp, err := b.c.Get(b.serverUrl + path)
	if err != nil {
		b.t.Fatalf("get %s failed: %s", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b.t.Fatalf("get %s expected 200, got %d", path, resp.StatusCode)
	}
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		b.t.Fatalf("get %s failed to read body: %s", path, err)
	}
	b.currentUrl = resp.Request.URL.String()
	b.lastResponse = respBytes
}

func (b *TestBrowser) CheckGetErrors(path string, expectedHttpErr int) {
	b.t.Helper()
	resp, err := b.c.Get(b.serverUrl + path)
	if err != nil {
		b.t.Fatalf("get %s failed: %s", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != expectedHttpErr {
		b.t.Fatalf("get %s expected %d, got %d", path,
			expectedHttpErr, resp.StatusCode)
	}
}

// Makes a post request to the provided path with the provided form values.
func (b *TestBrowser) Post(path string,
	parameterNameToParameterValue map[string]string) {
	b.t.Helper()

	tryPost := func() error {
		form := url.Values{}
		for name, val := range parameterNameToParameterValue {
			form.Set(name, val)
		}
		resp, err := b.c.PostForm(b.serverUrl+path, form)
		if err != nil {
			return fmt.Errorf("post %s failed: %s", path, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			respBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("post %s expected 200, got %d: %s with body: %q", path, resp.StatusCode, resp.Status, respBytes)
		}
		respBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("get %s failed to read body: %s", path, err)
		}
		b.currentUrl = resp.Request.URL.String()
		b.lastResponse = respBytes
		return nil
	}
	const retries = 3
	var err error
	for i := 0; i < retries; i++ {
		err = tryPost()
		if err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	b.t.Fatal(err)
}

func (b *TestBrowser) CheckCookiesAreSecure(urlS string) {
	u, err := url.Parse(urlS)
	if err != nil {
		b.t.Fatalf("bad url: %s", err)
	}
	cookies := b.c.Jar.Cookies(u)
	for _, cookie := range cookies {
		if !cookie.Secure {
			b.t.Fatal("got insecure cookie")
		}
	}
}

// Makes a put request to the provided path with the provided form values.
func (b *TestBrowser) Put(path string,
	parameterNameToParameterValue map[string]string,
	expectedHttpStatusCode int) {
	b.t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for k, v := range parameterNameToParameterValue {
		if err := writer.WriteField(k, v); err != nil {
			b.t.Fatalf("write field %q failed: %v", k, err)
		}
	}
	writer.Close()

	req, err := http.NewRequest(http.MethodPut, b.serverUrl+path, &body)
	if err != nil {
		b.t.Fatalf("create put req failed got err: %q", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	putResp, err := b.c.Do(req)
	if err != nil {
		b.t.Fatalf("do: %v", err)
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != expectedHttpStatusCode {
		b.t.Fatalf("unexpected http status code, got: %d, expected: %d", putResp.StatusCode, expectedHttpStatusCode)
	}

	respBytes, err := io.ReadAll(putResp.Body)
	if err != nil {
		b.t.Fatalf("get %s failed to read body: %s", path, err)
	}
	b.currentUrl = putResp.Request.URL.String()
	b.lastResponse = respBytes
}

// Makes a delete request to the provided path with the provided query values.
func (b *TestBrowser) Delete(
	path string,
	queryParameterNameToValue map[string]string,
	expectedHttpStatusCode int) {
	b.t.Helper()

	u, err := url.Parse(b.serverUrl + path)
	if err != nil {
		b.t.Fatalf("parse url failed: %v", err)
	}
	q := u.Query()
	for name, val := range queryParameterNameToValue {
		q.Set(name, val)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
	if err != nil {
		b.t.Fatalf("create delete req failed: %v", err)
	}
	resp, err := b.c.Do(req)
	if err != nil {
		b.t.Fatalf("delete %s failed: %v", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedHttpStatusCode {
		b.t.Fatalf("unexpected http status code, got: %d, expected: %d", resp.StatusCode, expectedHttpStatusCode)
	}
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		b.t.Fatalf("delete %s failed to read body: %v", path, err)
	}
	b.currentUrl = resp.Request.URL.String()
	b.lastResponse = respBytes
}
func (b *TestBrowser) CheckPostErrors(path string,
	parameterNameToParameterValue map[string]string) {
	b.t.Helper()
	form := url.Values{}
	for name, val := range parameterNameToParameterValue {
		form.Set(name, val)
	}
	resp, err := b.c.PostForm(b.serverUrl+path, form)
	if err != nil {
		b.t.Fatalf("post %s failed: %s", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		b.t.Fatalf("post %s expected error but got %d", path, resp.StatusCode)
	}
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		b.t.Fatalf("failed to read body: %d", err)
	}
	b.currentUrl = resp.Request.URL.String()
	b.lastResponse = respBytes
}

// Sets all the env vars that srvconfig.ProdConfig reads with GetEnvOrDie.
// Tests that build a prod config must call this first.
func setProdConfigEnvVars(t testing.TB) {
	t.Setenv("TWIGG_SERVER_KEY", testTwiggServerKey)
	t.Setenv("TWIGG_TRACK_KEY", "test-track-key")
	t.Setenv("TWIGG_TOKEN_SIGNING_KEY", "sig_key")
	t.Setenv("TWIGG_DO_SPACES_ACCESS_KEY_ID", "test-do-spaces-key-id")
	t.Setenv("TWIGG_DO_SPACES_ACCESS_KEY_SECRET", "test-do-spaces-key-secret")
	t.Setenv("TWIGG_STRIPE_SECRET_KEY", "sk_test_123")
	t.Setenv("TWIGG_STRIPE_ENDPOINT_SECRET", "whsec_123")
	t.Setenv("TWIGG_GOOGLE_CLIENT_ID", "test-google-client-id")
	t.Setenv("TWIGG_GOOGLE_CLIENT_SECRET", "test-google-client-secret")
	t.Setenv("TWIGG_MS_AZURE_CLIENT_ID", "test-ms-azure-client-id")
	t.Setenv("TWIGG_MS_AZURE_CLIENT_SECRET", "test-ms-azure-client-secret")
	t.Setenv("TWIGG_PASSWORD_SALT", "salty")
	t.Setenv("TWIGG_MASTER_KEY", "master-key")
}

// Returns a twigg server with the mock config
func GetMockServer(t testing.TB, options ...MockServerOption) server.Srv {
	t.Helper()
	startTrackServer := false
	useMockTrackServer := false
	port, storageFolder, _, _ := setupTest(startTrackServer, useMockTrackServer, t)
	cfg := srvconfig.MockConfig(port, storageFolder, testTwiggServerKey, "", "")
	for _, opt := range options {
		opt(&cfg)
	}
	srv := server.NewSrv(cfg)
	const runInMaintenanceMode = false
	go srv.Run(runInMaintenanceMode)
	for !srv.IsReady {
		time.Sleep(10 * time.Microsecond)
	}
	return srv
}

// Returns a server with the mock config, and starts a Track server with the test
// config. Returns the twigg server and an observer to watch for webhooks that
// will be posted by the track server
// Requires docker to be installed
func GetMockServerAndStartTrackServer(t testing.TB, options ...MockServerOption) (server.Srv, *trackObserver) {
	t.Helper()
	startTrackServer := true
	useMockTrackServer := false
	port, storageFolder, trackServer, trackServerKey := setupTest(
		startTrackServer, useMockTrackServer, t)
	cfg := srvconfig.MockConfig(port, storageFolder, testTwiggServerKey, trackServer, trackServerKey)
	for _, opt := range options {
		opt(&cfg)
	}
	srv := server.NewSrv(cfg)

	trackObs := trackObserver{t: t, mu: &sync.Mutex{}}
	srv.SetTrackObserver(&trackObs)
	const runInMaintenanceMode = false
	go srv.Run(runInMaintenanceMode)
	for !srv.IsReady {
		time.Sleep(10 * time.Microsecond)
	}
	return srv, &trackObs
}

// Same as `GetMockServerAndStartTrackServer`, but runs a mock track server
func GetMockServerAndStartMockTrackServer(t testing.TB, options ...MockServerOption) (server.Srv, *trackObserver) {
	t.Helper()
	startTrackServer := true
	useMockTrackServer := true
	port, storageFolder, trackServer, trackServerKey := setupTest(startTrackServer, useMockTrackServer, t)
	cfg := srvconfig.MockConfig(port, storageFolder, testTwiggServerKey, trackServer, trackServerKey)
	for _, opt := range options {
		opt(&cfg)
	}
	srv := server.NewSrv(cfg)

	trackObs := trackObserver{t: t, mu: &sync.Mutex{}}
	srv.SetTrackObserver(&trackObs)
	const runInMaintenanceMode = false
	go srv.Run(runInMaintenanceMode)
	for !srv.IsReady {
		time.Sleep(10 * time.Microsecond)
	}
	return srv, &trackObs
}

type MockServerOption = func(*srvconfig.SrvConfig)

func WithRateLimit(rateLimitQps float64, rateLimitQpsBurts int) MockServerOption {
	return func(c *srvconfig.SrvConfig) {
		c.RateLimitMaxQps = rateLimitQps
		c.RateLimitMaxQpsBurst = rateLimitQpsBurts
	}
}

func WithQueueRunnerSleep(d time.Duration) MockServerOption {
	return func(c *srvconfig.SrvConfig) {
		c.QueueRunnerSleep = d
	}
}

func (b TestBrowser) CheckCurrentPath(expectedPath string) {
	b.t.Helper()

	if b.CurrentPath() != expectedPath {
		b.t.Fatalf("expected current path %s got %s",
			expectedPath, b.CurrentPath())
	}
}

// Check the base path - i.e. the last part of the url excluding query params.
// E.g.:
// currentUrl = "http://me.com/abc?p=1" -> base path = `abc`
func (b TestBrowser) CheckCurrentPathBase(expectedBasePath string) {
	b.t.Helper()
	u, _ := url.Parse(b.CurrentPath())
	currentBasePath := path.Base(u.Path)
	if currentBasePath != expectedBasePath {
		b.t.Fatalf("expected current base path %s got %s",
			expectedBasePath, currentBasePath)
	}
}

func (b TestBrowser) CheckLastResponseWasHtml() {
	respString := strings.ToLower(string(b.lastResponse))
	probablyHtml := strings.Contains(respString, "<html")
	if !probablyHtml {
		b.t.Fatalf("last response %s is not html", b.lastResponse)
	}
}

func (b TestBrowser) CheckCurrentPageContains(subStrings ...string) {
	b.t.Helper()

	b.CheckLastResponseWasHtml()

	respString := strings.ToLower(string(b.lastResponse))
	for _, substring := range subStrings {

		containsSubstring := strings.Contains(respString, substring)
		if !containsSubstring {
			b.t.Fatalf("last response doesn't contain %s", substring)
		}
	}
}

func (b TestBrowser) CheckCurrentPageNotContains(subStrings ...string) {
	b.t.Helper()

	b.CheckLastResponseWasHtml()

	respString := strings.ToLower(string(b.lastResponse))
	for _, substring := range subStrings {

		containsSubstring := strings.Contains(respString, substring)
		if containsSubstring {
			b.t.Fatalf("last response contains %s", substring)
		}
	}
}

func (b TestBrowser) CheckLastResponseWasNotHtml() {
	respString := strings.ToLower(string(b.lastResponse))
	probablyHtml := strings.Contains(respString, "<html")
	if probablyHtml {
		b.t.Fatalf("last response %s is html", b.lastResponse)
	}
}

// Helper to mock an OAuth sign-in of a user with the provided email
func MockUserOAuthSignIn(srv server.Srv, b *TestBrowser, Email string) {
	b.t.Helper()

	// Fake a redirect url from google OAuth.
	// The redirect url has to return an ok response, else the browser won't
	// follow it.
	if srv.GoogleOAuthClientMock == nil {
		b.t.Fatalf("server's GoogleOAuthClientMock is nil")
	}
	srv.GoogleOAuthClientMock.SetRedirectUrl("/fake-oauth")
	b.Get(routes.StartLoginWithGoogleOAuth)
	b.CheckCurrentPathBase("fake-oauth")

	// Mock the response that google will give to the server
	srv.GoogleOAuthClientMock.AddGetUserInfo(oauthclient.UserInfo{Email: Email})
	// Now the browser hits the callback. The OAuth mock will give the
	// server the response that we mocked
	b.Get(routes.CallbackLoginWithGoogleOAuth +
		"?state=" + srv.GoogleOAuthClientMock.GetLastAuthCodeURLState())
}

// Mocks a user registering via oauth
func MockUserOAuthRegistration(srv server.Srv, b *TestBrowser,
	Email string, Username string) {
	b.t.Helper()

	// First they log in
	MockUserOAuthSignIn(srv, b, Email)
	// They are redirected to a page to choose a username
	b.CheckCurrentPath(routes.SetUsernamePath)
	// Hitting other paths will still take them to the same page
	b.Get("/home")
	b.CheckCurrentPath(routes.SetUsernamePath)

	b.Post(
		routes.SetUsernamePath,
		map[string]string{
			routes.SetUsernameParamName: Username,
		})
}

// Mock what happens when the users chooses to pay via stripe
func MockUserChoosesToPayViaStripe(srv server.Srv, b *TestBrowser,
	plan stripeclient.PriceId, quantity int64) {
	// The bowser will make a post request to the url to start paying.
	// The server will use the mockedStripeClient to tell the browser which URL to go
	b.Post(
		routes.SubscribeWithStripeUrl,
		map[string]string{
			routes.StripePriceIdParamName:  string(plan),
			routes.StripeQuantityParamName: strconv.FormatInt(quantity, 10),
		})
	_, _, stripeSessionUrl := srv.StripeClientMock.GetLastStripeSession()
	if srv.C.PublicUrl+stripeSessionUrl != b.CurrentUrl() {
		b.t.Fatalf("expected redirect to %s got %s",
			srv.C.PublicUrl+stripeSessionUrl, b.CurrentUrl())
	}
}

// Helper that mocks what happens when user completes a payment in stripe
func MockUserFinishedPayingInStripe(srv server.Srv, b *TestBrowser,
	plan stripeclient.PriceId, quantity int64, postWebhookEvent bool) {
	b.t.Helper()

	// Get the mock client and ask it data about the last session it created
	_, stripeSessionId, _ := srv.StripeClientMock.GetLastStripeSession()
	// Mock that the session was paid
	srv.StripeClientMock.MockSessionPaid(stripeSessionId, plan, quantity)
	if postWebhookEvent {
		// Make a an empty post request as if it were Stripe sending a webhook
		// event to our server. Once the stripeclient is called to parse the
		// webhook event, it'll say that the session was paid
		makeEmptyPostRequest(srv.C.PublicUrl+routes.StripeWebhook, b.t)
	}
}

// Mock what happens when the users chooses to pay for a org via stripe
func MockUserChoosesToPayForOrgViaStripe(srv server.Srv, b *TestBrowser, orgName string,
	plan stripeclient.PriceId, quantity int64) {
	// The bowser will make a post request to the url to start paying.
	// The server will use the mockedStripeClient to tell the browser which URL to go
	b.Post(routes.SubscribeWithStripeUrl, map[string]string{
		routes.IsChoosingPlanForOrgParamName: "true",
		routes.OrganizationNameParamName:     orgName,
		routes.StripePriceIdParamName:        string(plan),
		routes.StripeQuantityParamName:       strconv.FormatInt(quantity, 10),
	})
	_, _, stripeSessionUrl := srv.StripeClientMock.GetLastStripeSession()
	if srv.C.PublicUrl+stripeSessionUrl != b.CurrentUrl() {
		b.t.Fatalf("expected redirect to %s got %s",
			srv.C.PublicUrl+stripeSessionUrl, b.CurrentUrl())
	}
}

// Helper that mocks what happens when user grants permission to a user
func MockUserGrantsPermission(srv server.Srv, b *TestBrowser, repoOwner, repoName, usernameToGrantPermission string) {
	b.t.Helper()

	b.Post(
		"/"+repoOwner+"/"+repoName+"/settings/add",
		map[string]string{
			routes.UsernameParameterName: usernameToGrantPermission,
		})
	if string(b.lastResponse) != "ok" {
		b.t.Fatalf("expected last response \"ok\" got: %q", b.lastResponse)
	}
}

// Helper that mocks what happens when user grants permission to a user
func MockUserRevokePermission(srv server.Srv, b *TestBrowser, repoOwner, repoName, usernameToRevokePermission string) {
	b.t.Helper()

	b.Post(
		"/"+repoOwner+"/"+repoName+"/settings/remove",
		map[string]string{
			routes.UsernameParameterName: usernameToRevokePermission,
		})
	if string(b.lastResponse) != "ok" {
		b.t.Fatalf("expected last response \"ok\" got: %q", b.lastResponse)
	}
}

// Creates an org with the given name and pays for it via
// stripe (including posting the webhook). The browser must already be signed
// in as the user who will own the org.
func MockSetupOrgAndPay(srv server.Srv, b *TestBrowser, orgName string, plan stripeclient.PriceId, quantity int64) {
	b.t.Helper()
	b.Post(routes.NewOrganizationPattern, map[string]string{
		routes.NewOrganizationNameParamName: orgName,
	})
	MockUserChoosesToPayForOrgViaStripe(srv, b, orgName, plan, quantity)
	MockUserFinishedPayingInStripe(srv, b, plan, quantity, true)
}

func makeEmptyPostRequest(url string, t testing.TB) {
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		t.Fatalf("failed to make empty POST request: %v", err)
	}
	defer resp.Body.Close()
}

// Get a port, serverStorageDir and backupFolderAbsPath that can be used to
// instantiate and start a server. If `startTrackServer` is used, a track
// server is also started so that jobs can be posted to it and
// it can post webhooks back - only use this if needed on the test as it
// makes the tests more heavyweight. Automatically sets up the cleanups for the
// test as well.
func setupTest(startTrackServer bool, useMockTrackServer bool, t testing.TB) (port int, serverStorageDir, trackServerUrl, trackServerKey string) {
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("unable to create test instance: %s", err)
	}
	serverStorageDir = filepath.Join(currentDir, "main-test")
	os.MkdirAll(serverStorageDir, os.ModePerm)
	os.RemoveAll(serverStorageDir)
	t.Cleanup(
		func() {
			os.RemoveAll(serverStorageDir)
		},
	)

	// Get a free port under which ther server will run
	port = serverutils.GetFreePort(t)
	if startTrackServer {
		var trackServer trackserver.Srv
		if useMockTrackServer {
			trackServer = trackserver.GetMockTestServer(
				fmt.Sprintf("http://localhost:%d", port),
				routes.TrackWebhooksPath, testTwiggServerKey, t)
		} else {
			trackServer = trackserver.GetTestServer(
				fmt.Sprintf("http://localhost:%d", port),
				routes.TrackWebhooksPath, testTwiggServerKey, t)
		}
		// Start a track server that will post the webhooks back to this one
		trackServerUrl = trackServer.Url()
		trackServerKey = trackServer.TrackKey()

		trackClient := trackclient.NewClient(trackServerUrl, trackServerKey)
		ok, err := trackClient.HealthIsOk()
		if !ok {
			t.Fatalf("track server HealthCheck failed: %s", err)
		}
	}
	return
}

const testTwiggServerKey = "test-api-key"

// Simple observer for testing that saves all jobs/payloads received
type trackObserver struct {
	t        testing.TB
	Jobs     []trackclient.TrackJob
	Payloads []runnerlib.JobPayload
	mu       *sync.Mutex
}

func (obs *trackObserver) OnTrackWebhookReceived(job trackclient.TrackJob, payload runnerlib.JobPayload) {
	obs.mu.Lock()
	defer obs.mu.Unlock()
	obs.Jobs = append(obs.Jobs, job)
	obs.Payloads = append(obs.Payloads, payload)
}

func (obs *trackObserver) WaitForNWebhooksWithStatus(st trackclient.TrackJobStatus, n int) {
	start := time.Now()
	for {
		gotCount := 0
		for i := range obs.Jobs {
			if obs.Jobs[i].Status == st {
				gotCount += 1
			}
		}
		if gotCount >= n {
			return
		}
		time.Sleep(50 * time.Millisecond)
		if time.Since(start) > 25*time.Second {
			obs.t.Fatalf("spend too long waiting for %d webhooks with status=%s", n, st)
		}
	}
}

func (obs *trackObserver) WaitForWebhooksWithStatus(st trackclient.TrackJobStatus) {
	obs.WaitForNWebhooksWithStatus(st, 1)
}

func (obs *trackObserver) WaitForWebhooksForPipelineStage(pipelineStage int32, st trackclient.TrackJobStatus) {
	start := time.Now()
	for {
		for i := range obs.Jobs {
			_, _, _, _, _, _, stage, ok := jobs.ParsePipelineStageId(obs.Jobs[i].Id)
			if ok && stage == pipelineStage && obs.Jobs[i].Status == st {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
		if time.Since(start) > 25*time.Second {
			obs.t.Fatalf("spend too long waiting for webhook for pipeline stage %d with status=%s", pipelineStage, st)
		}
	}
}

func (obs *trackObserver) ClearObservedWebhooks() {
	obs.mu.Lock()
	defer obs.mu.Unlock()
	obs.Jobs = []trackclient.TrackJob{}
	obs.Payloads = []runnerlib.JobPayload{}
}

// PostJsonExpectNoContent sends a POST request with a JSON body and
// asserts that the response status is 204 No Content.
// The test fails immediately if the request cannot be built,
// the request fails, or the status code is not 204.
func (b TestBrowser) CheckPostJsonReturnsNoContent(path string, body string) {
	b.t.Helper()

	req, err := http.NewRequest("POST", b.serverUrl+path, strings.NewReader(body))
	if err != nil {
		b.t.Fatalf("failed to build POST request %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.c.Do(req)
	if err != nil {
		b.t.Fatalf("post %s failed: %v", path, err)
	}
	defer resp.Body.Close()

	b.lastResponse, _ = io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusNoContent {
		b.t.Fatalf(
			"post %s expected 204, got %d body=%q",
			path,
			resp.StatusCode,
			string(b.lastResponse),
		)
	}
}

// PostJson makes a POST request marshaling the provided value as JSON body to the provided path.
// Returns the HTTP status code. Fails the test if the request itself fails or if marshaling fails.
func (b *TestBrowser) PostJson(path string, body any) int {
	b.t.Helper()
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		b.t.Fatalf("failed to marshal body: %v", err)
	}
	req, err := http.NewRequest("POST", b.serverUrl+path, bytes.NewReader(bodyBytes))
	if err != nil {
		b.t.Fatalf("failed to build POST request %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.c.Do(req)
	if err != nil {
		b.t.Fatalf("post %s failed: %v", path, err)
	}
	defer resp.Body.Close()
	b.lastResponse, err = io.ReadAll(resp.Body)
	if err != nil {
		b.t.Fatalf("post %s failed to read body: %v", path, err)
	}
	b.currentUrl = resp.Request.URL.String()
	return resp.StatusCode
}
