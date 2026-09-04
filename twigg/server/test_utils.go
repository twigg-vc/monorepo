package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"monorepo/base/iterator"
	"monorepo/twigg/cli/clidb"
	"monorepo/twigg/client"
	"monorepo/twigg/commit"
	"monorepo/twigg/tree"
	"monorepo/twigg/workdir"
	"monorepo/twigg/xchange"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"
)

type testServer struct {
	s              Server
	db             clidb.CliDb
	t              *testing.T
	url            string
	apiKeyVerifier simpleApiKeyVerifier
	pushObs        PushObserver
}

type simpleApiKeyVerifier struct {
	expectedKey string
}

func (s simpleApiKeyVerifier) PushIsOk(r *http.Request) (bool, string, error) {
	k := xchange.GetApiKeyHeader(r)
	if k == s.expectedKey {
		return true, "", nil
	}
	return false, xchange.BadApiKeyErrMsg, nil
}
func (s simpleApiKeyVerifier) PullIsOk(r *http.Request) (bool, string, error) {
	k := xchange.GetApiKeyHeader(r)
	if k == s.expectedKey {
		return true, "", nil
	}
	return false, xchange.BadApiKeyErrMsg, nil
}

const quotaOwner = "q-owner"

const mockRandomResponsePath = "mock-random-resp"

func newTestServer(apiKey string, t *testing.T) TestServer {
	db, closeDb, err := clidb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDb)

	l, ul, commitTx, err := db.BeginWrite()
	defer ul()
	if err != nil {
		t.Fatal(err)
	}
	sl := db.Bind(l)

	s, err := NewServer(quotaOwner, 1, sl)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Init(sl)
	if err != nil {
		t.Fatal(err)
	}
	err = commitTx()
	if err != nil {
		t.Fatal(err)
	}

	port := getFreePort(t)
	ts := &testServer{
		db:             db,
		s:              s,
		t:              t,
		url:            fmt.Sprintf("http://:%d", port),
		apiKeyVerifier: simpleApiKeyVerifier{expectedKey: apiKey},
	}

	// Register the handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/{owner}/{repo}/push", func(w http.ResponseWriter, r *http.Request) {
		l, ul, commitTx, err := ts.db.BeginWrite()
		defer ul()
		if err != nil {
			t.Fatal(err)
			return
		}
		sl := ts.db.Bind(l)

		// Only commit if ok
		ok := ts.s.HandlePush(w, r, ts.apiKeyVerifier, ts.pushObs, sl)
		if !ok {
			return
		}
		err = commitTx()
		if err != nil {
			t.Fatal(err)
			return
		}
	})
	mux.HandleFunc("/{owner}/{repo}/pull", func(w http.ResponseWriter, r *http.Request) {
		l, ul, err := ts.db.BeginRead()
		defer ul()
		if err != nil {
			t.Fatal(err)
			return
		}
		sl := ts.db.Bind(l)
		_ = ts.s.HandlePull(w, r, ts.apiKeyVerifier, sl)
	})
	mux.HandleFunc("/{owner}/{repo}"+client.SetServerIdEndpoint, func(w http.ResponseWriter, r *http.Request) {
		xchange.SetTwiggHeaderInResponse(w)
		ok, notOkMsg, err := ts.apiKeyVerifier.PushIsOk(r)
		if err != nil {
			t.Fatal(err)
			return
		}
		if !ok {
			http.Error(w, notOkMsg, http.StatusForbidden)
			return
		}
		id, err := strconv.ParseUint(r.URL.Query().Get(client.SetServerIdQueryParam), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		l, ul, commitTx, err := ts.db.BeginWrite()
		defer ul()
		if err != nil {
			t.Fatal(err)
			return
		}
		err = ts.s.SetNextServerId(id, ts.db.Bind(l))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = commitTx()
		if err != nil {
			t.Fatal(err)
			return
		}
	})
	mux.HandleFunc(fmt.Sprintf("/%s", mockRandomResponsePath), func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("random response"))
	})
	mux.HandleFunc(fmt.Sprintf("/%s%s", mockRandomResponsePath, client.SetServerIdEndpoint), func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("random response"))
	})
	mux.HandleFunc(fmt.Sprintf("/%s/pull", mockRandomResponsePath), func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("random response"))
	})
	mux.HandleFunc(fmt.Sprintf("/%s/push", mockRandomResponsePath), func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("random response"))
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	// Create an httpserver
	httpSrv := http.Server{Addr: ":" + strconv.Itoa(port), Handler: mux}
	go httpSrv.ListenAndServe()
	start := time.Now()
	for {
		resp, err := http.Get(ts.url + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		if time.Since(start) > time.Second {
			t.Fatalf("twigg test server took to long to start")
		}
	}
	t.Cleanup(func() { httpSrv.Close() })
	return ts
}

func (ts *testServer) SetPushObserver(p PushObserver) {
	ts.pushObs = p
}

func (ts *testServer) GetServer() Server {
	return ts.s
}

func (ts *testServer) BeginWrite() (context.Context, func(), func() error) {
	w, closeW, commitW, err := ts.db.BeginWrite()
	if err != nil {
		ts.t.Fatal(err)
	}
	return w, closeW, commitW
}
func (ts *testServer) BindW(w context.Context) Write {
	return ts.db.Bind(w)
}
func (ts *testServer) BindR(r context.Context) Read {
	return ts.db.Bind(r)
}

func (ts *testServer) Restart() {
	l, ul, commitTx, err := ts.db.BeginWrite()
	defer ul()
	if err != nil {
		ts.t.Fatal(err)
	}
	sl := ts.db.Bind(l)

	s, err := NewServer(quotaOwner, 1, sl)
	if err != nil {
		ts.t.Fatal(err)
	}
	if !s.WasInit() {
		err = s.Init(sl)
		if err != nil {
			ts.t.Fatal(err)
		}
	}
	err = commitTx()
	if err != nil {
		ts.t.Fatal(err)
	}
	ts.s = s
}

const testRepoOwnerName = "bilbo"
const testRepoName = "red-book"

// Root server
func (s testServer) RootUrl() string {
	return s.url
}

// Clients can send push/pull to RootUrl()/ServerPath()
func (s testServer) ServerPath() string {
	return testRepoOwnerName + "/" + testRepoName
}
func (s testServer) RandomResponsePath() string {
	return mockRandomResponsePath
}

func (s testServer) Url() string {
	return s.url
}
func (s testServer) Pending() (iterator.I[commit.Commit], func()) {
	l, ul, err := s.db.BeginRead()
	if err != nil {
		ul()
		s.t.Fatal(err)
	}
	sl := s.db.Bind(l)
	it, err := s.s.Pending(
		/*ascending*/ true, sl)
	if err != nil {
		ul()
		s.t.Fatal(err)
	}
	return it, ul
}
func (s testServer) IsPending(n uint64) bool {
	l, ul, err := s.db.BeginRead()
	defer ul()
	if err != nil {
		s.t.Fatal(err)
	}
	sl := s.db.Bind(l)
	c, err := s.s.GetLatest(n, sl)
	if err != nil {
		s.t.Fatal(err)
	}
	return !c.IsSubmitted
}
func (s testServer) NPending() int {
	l, ul, err := s.db.BeginRead()
	defer ul()
	if err != nil {
		s.t.Fatal(err)
	}
	sl := s.db.Bind(l)
	it, err := s.s.Pending(
		/*ascending*/ true, sl)
	if err != nil {
		s.t.Fatal(err)
	}
	count := 0
	for it.Next() {
		count++
	}
	err = it.Err()
	if err != nil {
		s.t.Fatal(err)
	}
	return count
}

// Descending order
func (s testServer) PendingAfter(afterId commit.LocalId) (iterator.I[commit.Commit], func()) {
	l, ul, err := s.db.BeginRead()
	if err != nil {
		ul()
		s.t.Fatal(err)
	}
	sl := s.db.Bind(l)
	it, err := s.s.PendingAfter(afterId, sl)
	if err != nil {
		ul()
		s.t.Fatal(err)
	}
	return it, ul
}
func (s testServer) Top() commit.Commit {
	return s.s.Top()
}
func (s testServer) Diff(a, b commit.Commit) []tree.Diff {
	l, ul, err := s.db.BeginRead()
	defer ul()
	if err != nil {
		s.t.Fatal(err)
	}

	sl := s.db.Bind(l)

	it, err := s.s.Diff(a, b, sl)
	if err != nil {
		s.t.Fatal(err)
	}

	diffs := []tree.Diff{}
	for it.CanGet() {
		diff := it.GetDiff()
		diffs = append(diffs, diff)
		err = it.Next()
		if err != nil {
			s.t.Fatal(err)
		}
	}
	return diffs
}

func (s testServer) WriteFile(c commit.Commit, filename string, w io.Writer) {
	l, ul, err := s.db.BeginRead()
	defer ul()
	if err != nil {
		s.t.Fatal(err)
	}
	sl := s.db.Bind(l)
	err = s.s.WriteFile(c, filename, w, sl)
	if err != nil {
		s.t.Fatal(err)
	}
}

func (s testServer) GetCommitWorkdirSize(c commit.Commit) int64 {
	l, ul, err := s.db.BeginRead()
	defer ul()
	if err != nil {
		s.t.Fatal(err)
	}
	sl := s.db.Bind(l)
	n, err := s.s.GetCommitWorkdirSize(c, sl)
	if err != nil {
		s.t.Fatal(err)
	}
	return n
}

func (s testServer) GetLatest(n commit.LocalId) commit.Commit {
	l, ul, err := s.db.BeginRead()
	defer ul()
	if err != nil {
		s.t.Fatal(err)
	}
	sl := s.db.Bind(l)

	c, err := s.s.GetLatest(n, sl)
	if err != nil {
		s.t.Fatal(err)
	}
	return c
}

func (s testServer) GetVersion(cl commit.LocalId, v uint64) commit.Commit {
	l, ul, err := s.db.BeginRead()
	defer ul()
	if err != nil {
		s.t.Fatal(err)
	}
	sl := s.db.Bind(l)

	c, err := s.s.GetVersion(cl, v, sl)
	if err != nil {
		s.t.Fatal(err)
	}
	return c
}

func (s testServer) CheckNotFound(cId commit.LocalId) {
	l, ul, err := s.db.BeginRead()
	defer ul()
	if err != nil {
		s.t.Fatal(err)
	}
	sl := s.db.Bind(l)
	_, err = s.s.GetLatest(cId, sl)
	if !errors.Is(err, ErrNotFound) {
		s.t.Fatalf("expected not found err got %s", err)
	}
}
func (s testServer) CheckVersionNotFound(cId commit.LocalId, version uint64) {
	l, ul, err := s.db.BeginRead()
	defer ul()
	if err != nil {
		s.t.Fatal(err)
	}
	sl := s.db.Bind(l)
	_, err = s.s.GetVersion(cId, version, sl)
	if !errors.Is(err, ErrNotFound) {
		s.t.Fatalf("expected not found err got %s", err)
	}
}

func (s testServer) Submit(cl commit.LocalId) {
	l, ul, commitTx, err := s.db.BeginWrite()
	defer ul()
	if err != nil {
		s.t.Fatal(err)
	}
	sl := s.db.Bind(l)

	c, err := s.s.GetLatest(cl, sl)
	if err != nil {
		s.t.Fatal(err)
	}
	err = s.s.Submit(c, sl)
	if err != nil {
		s.t.Fatalf("failed to submit: %s", err)
	}
	err = commitTx()
	if err != nil {
		s.t.Fatal(err)
	}
}

func (s testServer) CanSubmit(cl commit.LocalId) bool {
	l, ul, _, err := s.db.BeginWrite()
	defer ul()
	if err != nil {
		s.t.Fatal(err)
	}
	sl := s.db.Bind(l)
	c, err := s.s.GetLatest(cl, sl)
	if err != nil {
		s.t.Fatal(err)
	}
	ok, _, err := s.s.CanSubmit(c, sl)
	if err != nil {
		s.t.Fatalf("failed to test can-submit: %s", err)
	}
	return ok
}

func (s testServer) CreateRollback(cId commit.LocalId, authorUserId int64) commit.Commit {
	s.t.Helper()
	l, ul, commitTx, err := s.db.BeginWrite()
	defer ul()
	if err != nil {
		s.t.Fatal(err)
	}
	sl := s.db.Bind(l)
	c, err := s.s.CreateRollback(cId, authorUserId, sl)
	if err != nil {
		s.t.Fatal(err)
	}
	err = commitTx()
	if err != nil {
		s.t.Fatal(err)
	}
	return c
}

func (s testServer) RenameCommit(cId commit.LocalId, newMessage string, authorUserId int64) commit.Commit {
	s.t.Helper()
	l, ul, commitTx, err := s.db.BeginWrite()
	defer ul()
	if err != nil {
		s.t.Fatal(err)
	}
	sl := s.db.Bind(l)
	c, err := s.s.RenameCommit(cId, newMessage, authorUserId, sl)
	if err != nil {
		s.t.Fatal(err)
	}
	err = commitTx()
	if err != nil {
		s.t.Fatal(err)
	}
	return c
}

func (s testServer) Load(c commit.LocalId, wd workdir.Workdir) {
	l, ul, err := s.db.BeginRead()
	defer ul()
	if err != nil {
		s.t.Fatal(err)
	}
	sl := s.db.Bind(l)

	err = s.s.Load(c, wd, sl)
	if err != nil {
		s.t.Fatalf("failed to load commit: %s", err)
	}
}

func (s testServer) CheckLoadErrors(c commit.LocalId, wd workdir.Workdir) {
	l, ul, err := s.db.BeginRead()
	defer ul()
	if err != nil {
		s.t.Fatal(err)
	}
	sl := s.db.Bind(l)

	err = s.s.Load(c, wd, sl)
	if err == nil {
		s.t.Fatalf("expected load to error")
	}
}

// Get a port that's free from the OS
func getFreePort(t *testing.T) int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}