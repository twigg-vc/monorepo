package owners

import (
	"context"
	"errors"
	"monorepo/twigg/client"
	"monorepo/twigg/server"
	"strings"
	"testing"
)

const testApiKey = "fake-api-key"

func TestNoOwnersFile(t *testing.T) {
	srv := server.NewTestServer(testApiKey, t)
	const repoId = 99
	root, tw, wd, clientRead := client.NewTest("owner", repoId, t)
	sp := serverProvider{srv: srv, srvRepoId: repoId}
	ownersService := New(sp)

	// Create and push one commit without any owners
	wd.WriteFile("c1.txt", "c1")
	c1, _ := tw.Commit(wd, "c1", &root, clientRead)
	tw.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)

	// Check the owners on the server
	wl, closeServerW, _ := srv.BeginWrite()
	defer closeServerW()
	has, err := ownersService.OwnersLgmtIsOk(repoId, 1,
		/*usersWhoLgtm=*/ nil,
		/*commitIdToReadOwners*/ 0,
		/*supremeLeaders*/ []string{},
		wl)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("got no-owners-lgtm")
	}
}

func TestOwnersAtRoot(t *testing.T) {
	srv := server.NewTestServer(testApiKey, t)
	const repoId = 99
	root, tw, wd, clientRead := client.NewTest("owner", repoId, t)
	sp := serverProvider{srv: srv, srvRepoId: repoId}
	ownersService := New(sp)

	// Create an OWNERS at root in the first commit
	wd.WriteFile(OwnersFileName, "aang\nappa")
	c1, _ := tw.Commit(wd, "c1", &root, clientRead)
	// Write a second commit that modifies some file.
	wd.WriteFile("a.txt", "aaa")
	c2, _ := tw.Commit(wd, "c2", &c1, clientRead)
	tw.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)

	wl, closeServerW, _ := srv.BeginWrite()
	defer closeServerW()
	// c1 requires no owners because there is no owners file yet in c0
	has, err := ownersService.OwnersLgmtIsOk(repoId, 1,
		/*usersWhoLgtm=*/ nil,
		/*commitIdToReadOwners*/ 0,
		/*supremeLeaders*/ []string{},
		wl)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("got no-owners-lgtm on commit 1")
	}
	// c2 requires no owners if we read owners at c0
	has, err = ownersService.OwnersLgmtIsOk(repoId, 2,
		/*usersWhoLgtm=*/ nil,
		/*commitIdToReadOwners*/ 0,
		/*supremeLeaders*/ []string{},
		wl)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("got owners-lgtm on commit 2 reading owners at 0")
	}
	// c2 requires owners if we read owners at c1
	has, err = ownersService.OwnersLgmtIsOk(repoId, 2,
		/*usersWhoLgtm=*/ nil,
		/*commitIdToReadOwners*/ 1,
		/*supremeLeaders*/ []string{},
		wl)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("got owners-lgtm on commit 2")
	}
	// Aang or appa should be able to approve
	has, err = ownersService.OwnersLgmtIsOk(repoId, 2,
		[]string{"aang"},
		/*commitIdToReadOwners*/ 1,
		/*supremeLeaders*/ []string{},
		wl)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("got no owners-lgtm on commit 2")
	}
	has, err = ownersService.OwnersLgmtIsOk(repoId, 2,
		[]string{"appa"},
		/*commitIdToReadOwners*/ 1,
		/*supremeLeaders*/ []string{},
		wl)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("got no owners-lgtm on commit 2")
	}
	// But not anyone else
	has, err = ownersService.OwnersLgmtIsOk(repoId, 2,
		[]string{"zuko"},
		/*commitIdToReadOwners*/ 1,
		/*supremeLeaders*/ []string{},
		wl)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("got owners-lgtm on commit 2")
	}
}

func TestRootOwnerAppliesToSubfolder(t *testing.T) {
	srv := server.NewTestServer(testApiKey, t)
	const repoId = 99
	root, tw, wd, clientRead := client.NewTest("owner", repoId, t)
	sp := serverProvider{srv: srv, srvRepoId: repoId}
	ownersService := New(sp)

	// c1 creates an OWNERS at root
	wd.WriteFile(OwnersFileName, "aang")
	c1, _ := tw.Commit(wd, "c1", &root, clientRead)
	// c2 creates an OWNERS at subfolder with another user
	wd.WriteFile("/sub/sub/"+OwnersFileName, "zuko")
	c2, _ := tw.Commit(wd, "c2", &c1, clientRead)
	// c3 modifies something in subfolder
	wd.WriteFile("/sub/sub/sub/a.txt", "hello")
	c3, _ := tw.Commit(wd, "c3", &c2, clientRead)
	// Push all
	tw.Push(&c3, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)

	wl, closeServerW, _ := srv.BeginWrite()
	defer closeServerW()
	// root owner counts as owner
	has, err := ownersService.OwnersLgmtIsOk(repoId, 3,
		/*usersWhoLgtm=*/ []string{"aang"},
		/*commitIdToReadOwners*/ 2,
		/*supremeLeaders*/ []string{},
		wl)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("root owner didnt allow subfolder owner")
	}

	// subfolder owner also counts
	has, err = ownersService.OwnersLgmtIsOk(repoId, 3,
		/*usersWhoLgtm=*/ []string{"zuko"},
		/*commitIdToReadOwners*/ 2,
		/*supremeLeaders*/ []string{},
		wl)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("subfolder owner didn't count as lgtm")
	}

	// others dont
	has, err = ownersService.OwnersLgmtIsOk(repoId, 3,
		/*usersWhoLgtm=*/ []string{"iron"},
		/*commitIdToReadOwners*/ 2,
		/*supremeLeaders*/ []string{},
		wl)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("random username counted as lgtm")
	}
}

func TestNoOwnersAtParentFolder(t *testing.T) {
	srv := server.NewTestServer(testApiKey, t)
	const repoId = 99
	root, tw, wd, clientRead := client.NewTest("owner", repoId, t)
	sp := serverProvider{srv: srv, srvRepoId: repoId}
	ownersService := New(sp)

	// Create an OWNERS at /sub/subsub in the first commit
	wd.WriteFile("sub/subsub/"+OwnersFileName, "aang")
	c1, _ := tw.Commit(wd, "c1", &root, clientRead)
	// Write a second commit that modifies some file in a child dir of the owners
	wd.WriteFile("sub/subsub/subsubsub/subsubsubsub/a.txt", "aaa")
	c2, _ := tw.Commit(wd, "c2", &c1, clientRead)
	tw.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)

	wl, closeServerW, _ := srv.BeginWrite()
	defer closeServerW()
	// c2 requires owners
	has, err := ownersService.OwnersLgmtIsOk(repoId, 2,
		/*usersWhoLgtm=*/ nil,
		/*commitIdToReadOwners*/ 1,
		/*supremeLeaders*/ []string{},
		wl)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("got owners-lgtm ok")
	}
	has, err = ownersService.OwnersLgmtIsOk(repoId, 2,
		/*usersWhoLgtm=*/ []string{"aang"},
		/*commitIdToReadOwners*/ 1,
		/*supremeLeaders*/ []string{},
		wl)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("got owners-lgtm not ok")
	}
}

func TestSupremeLeader(t *testing.T) {
	srv := server.NewTestServer(testApiKey, t)
	const repoId = 99
	root, tw, wd, clientRead := client.NewTest("owner", repoId, t)
	sp := serverProvider{srv: srv, srvRepoId: repoId}
	ownersService := New(sp)

	// Create an OWNERS at root in the first commit
	wd.WriteFile(OwnersFileName, "aang\nappa")
	c1, _ := tw.Commit(wd, "c1", &root, clientRead)
	// Write a second commit that modifies some file.
	wd.WriteFile("a.txt", "aaa")
	c2, _ := tw.Commit(wd, "c2", &c1, clientRead)
	tw.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)

	wl, closeServerW, _ := srv.BeginWrite()
	defer closeServerW()
	// c2 is not approved when owners are read from c1, even if supremeLeaders
	// []string{} provided but they haven't LGTM'd
	has, err := ownersService.OwnersLgmtIsOk(repoId, 2,
		/*usersWhoLgtm=*/ nil,
		/*commitIdToReadOwners*/ 1,
		/*supremeLeaders*/ []string{"iroh"},
		wl)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("got ok lgtm when supremeLeaders were []string{} but haven't lgtmd")
	}
	// c2 is approved when the supremeLeader has LGTM'd
	has, err = ownersService.OwnersLgmtIsOk(repoId, 2,
		/*usersWhoLgtm=*/ []string{"iroh"},
		/*commitIdToReadOwners*/ 0,
		/*supremeLeaders*/ []string{"iroh"},
		wl)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("got not ok when supremeLeader approved")
	}
}

func TestIsApprovedNoSubstringBypass(t *testing.T) {
	cases := []struct {
		name      string
		owner     string
		approvers []string
		wantOk    bool
	}{
		{
			name:      "single-char prefix of owner name is rejected",
			owner:     "aang",
			approvers: []string{"a"},
			wantOk:    false,
		},
		{
			name:      "multi-char prefix of owner name is rejected",
			owner:     "aang",
			approvers: []string{"aan"},
			wantOk:    false,
		},
		{
			name:      "suffix of owner name is rejected",
			owner:     "aang",
			approvers: []string{"ang"},
			wantOk:    false,
		},
		{
			name:      "exact match is approved",
			owner:     "aang",
			approvers: []string{"aang"},
			wantOk:    true,
		},
		{
			name:      "username that contains the owner entry as a substring is rejected",
			owner:     "alice",
			approvers: []string{"alice_admin"},
			wantOk:    false,
		},
		{
			name:      "owner entry with surrounding whitespace matches exact username",
			owner:     "  aang  ",
			approvers: []string{"aang"},
			wantOk:    true,
		},
		{
			name:      "owner entry with windows CRLF ending matches exact username",
			owner:     "aang\r",
			approvers: []string{"aang"},
			wantOk:    true,
		},
		{
			name:      "first of multiple approvers matches owner",
			owner:     "aang\nappa",
			approvers: []string{"aang", "zuko"},
			wantOk:    true,
		},
		{
			name:      "second of multiple approvers matches owner",
			owner:     "aang\nappa",
			approvers: []string{"zuko", "appa"},
			wantOk:    true,
		},
		{
			name:      "none of multiple approvers match owner",
			owner:     "aang\nappa",
			approvers: []string{"zuko", "iroh"},
			wantOk:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parser := ownersFileParser{file: strings.NewReader(tc.owner)}
			got, err := parser.isApproved(tc.approvers)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantOk {
				t.Errorf("got approved=%v, want %v", got, tc.wantOk)
			}
		})
	}
}

func TestMultipleSupremeLeaders(t *testing.T) {
	srv := server.NewTestServer(testApiKey, t)
	const repoId = 99
	root, tw, wd, clientRead := client.NewTest("owner", repoId, t)
	sp := serverProvider{srv: srv, srvRepoId: repoId}
	ownersService := New(sp)

	// Create an OWNERS at root requiring aang
	wd.WriteFile(OwnersFileName, "aang")
	c1, _ := tw.Commit(wd, "c1", &root, clientRead)
	// Write a second commit that modifies some file
	wd.WriteFile("a.txt", "aaa")
	c2, _ := tw.Commit(wd, "c2", &c1, clientRead)
	tw.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)

	wl, closeServerW, _ := srv.BeginWrite()
	defer closeServerW()

	// Only one of the two supreme leaders LGTM'd — not enough
	has, err := ownersService.OwnersLgmtIsOk(repoId, 2,
		/*usersWhoLgtm=*/ []string{"iroh"},
		/*commitIdToReadOwners*/ 1,
		/*supremeLeaders*/ []string{"iroh", "zuko"},
		wl)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("got ok lgtm with only one of two supreme leaders")
	}

	// Both supreme leaders LGTM'd bypasses owners
	has, err = ownersService.OwnersLgmtIsOk(repoId, 2,
		/*usersWhoLgtm=*/ []string{"iroh", "zuko"},
		/*commitIdToReadOwners*/ 1,
		/*supremeLeaders*/ []string{"iroh", "zuko"},
		wl)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("got not ok when all supreme leaders approved")
	}
}

func TestSubmittedCommitIsAlwaysOwnersLgtmOk(t *testing.T) {
	srv := server.NewTestServer(testApiKey, t)
	const repoId = 99
	root, tw, wd, clientRead := client.NewTest("owner", repoId, t)
	sp := serverProvider{srv: srv, srvRepoId: repoId}
	ownersService := New(sp)

	// c1 creates an OWNERS file at root requiring aang
	wd.WriteFile(OwnersFileName, "aang")
	c1, _ := tw.Commit(wd, "c1", &root, clientRead)
	tw.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, clientRead)

	// Before being submitted, c1 is lgtm-ok since there is no OWNERS file yet
	wl, closeServerW, _ := srv.BeginWrite()
	has, err := ownersService.OwnersLgmtIsOk(repoId, 1,
		/*usersWhoLgtm=*/ []string{"appa"},
		/*commitIdToReadOwners*/ 0,
		/*supremeLeaders*/ []string{},
		wl)
	closeServerW()
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("expected commit be to be lgtm-ok since here are no OWNERS yet")
	}

	// After being submitted, c1 still has lgtm-ok even without aang's (OWNER)
	// LGTM, but is is a submitted commit
	srv.Submit(1)

	wl, closeServerW, _ = srv.BeginWrite()
	defer closeServerW()
	has, err = ownersService.OwnersLgmtIsOk(repoId, 1,
		/*usersWhoLgtm=*/ []string{"appa"},
		/*commitIdToReadOwners*/ 0,
		/*supremeLeaders*/ []string{},
		wl)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("expected submitted commit to be lgtm-ok")
	}
}

type serverProvider struct {
	srv       server.TestServer
	srvRepoId uint64
}

func (sp serverProvider) GetServerByRepoId(rl context.Context, repoId uint64) (server.Server, error) {
	if sp.srvRepoId != repoId {
		return nil, errors.New("not found")
	}
	return sp.srv.GetServer(), nil
}
func (sp serverProvider) GetServerRead(rl context.Context) server.Read {
	return sp.srv.BindR(rl)
}
