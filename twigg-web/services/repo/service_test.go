package repo

import (
	"bytes"
	"log"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/services/secrets"
	"monorepo/twigg-web/webdb"
	"testing"
)

func TestGetNonExisting(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	secretsSrv, err := secrets.NewService(b, bytes.Repeat([]byte{1}, 32))
	if err != nil {
		log.Fatalf("unexpected err instantiating secrets srv: %s", err)
		return
	}
	s, err := NewService(b, secretsSrv)
	if err != nil {
		t.Fatal("unexpected instantiating err")
	}

	_, isNotFoundErr, err := s.GetByOwnerIdAndRepoName(w, 10, "non-existing")
	if !isNotFoundErr || err == nil {
		t.Fatal("expected is not found err")
	}
	_, isNotFoundErr, err = s.GetServer(w, 10, "non-existing")
	if !isNotFoundErr || err == nil {
		t.Fatal("expected is not found err")
	}
	iter, err := s.GetAllByOwnerId(w, 10)
	if err != nil {
		t.Fatal(err)
	}
	if iter.Next() {
		t.Fatal("expected no repos")
	}
	err = iter.Err()
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateNewAndGet(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	secretsSrv, err := secrets.NewService(b, bytes.Repeat([]byte{1}, 32))
	if err != nil {
		log.Fatalf("unexpected err instantiating secrets srv: %s", err)
		return
	}
	s, err := NewService(b, secretsSrv)
	if err != nil {
		t.Fatal("unexpected instantiating err")
	}

	r, _, err := s.CreateNew(w, 400, "new-repo", "repo description")
	if err != nil {
		t.Fatal(err)
	}
	expected := repo.Repo{
		Id:                    1,
		OwnerId:               400,
		DisplayName:           "new-repo",
		Description:           "repo description",
		IsGitMirrorEnabled:    false,
		SanitizedGitMirrorUrl: "",
	}
	if r != expected {
		t.Fatal("unexpected repo")
	}
	// Read back by id
	gotById, err := s.GetById(w, 1)
	if err != nil {
		t.Fatal(err)
	}
	if gotById != expected {
		t.Fatal("GetById gotById unexpected result")
	}

	iter, err := s.GetAllByOwnerId(w, 400)
	if err != nil {
		t.Fatal(err)
	}
	if !iter.Next() {
		t.Fatal("expected 1 entry")
	}
	r, err = iter.Get()
	if err != nil {
		t.Fatal(err)
	}
	if r != expected {
		t.Fatal("iterator returned wrong data")
	}
	if iter.Next() {
		t.Fatal("should be done iterating")
	}
	err = iter.Err()
	if err != nil {
		t.Fatal(err)
	}

	srv, isNotFoundErr, err := s.GetServer(w, 400, "new-repo")
	if isNotFoundErr {
		t.Fatal("got is not found err")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !srv.WasInit() {
		t.Fatal("got non initialized server")
	}
	srv, err = s.GetServerByRepoId(w, r.Id)
	if err != nil {
		t.Fatal(err)
	}
	if !srv.WasInit() {
		t.Fatal("got non initialized server")
	}
}

func TestCantCreateWithSameName(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	secretsSrv, err := secrets.NewService(b, bytes.Repeat([]byte{1}, 32))
	if err != nil {
		log.Fatalf("unexpected err instantiating secrets srv: %s", err)
		return
	}
	s, err := NewService(b, secretsSrv)
	if err != nil {
		t.Fatal("unexpected instantiating err")
	}

	s.CreateNew(w, 5, "new-repo", "repo description")
	_, isAlreadyExistsErr, err := s.CreateNew(w, 5, "new-repo", "")
	if !isAlreadyExistsErr || err == nil {
		t.Fatal("expected isAlreadyExists err")
	}
}

func TestArchiveAndNonArchivedRepoCountIsGreaterThan(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	secretsSrv, err := secrets.NewService(b, bytes.Repeat([]byte{1}, 32))
	if err != nil {
		log.Fatalf("unexpected err instantiating secrets srv: %s", err)
		return
	}
	s, err := NewService(b, secretsSrv)
	if err != nil {
		t.Fatal("unexpected instantiating err")
	}

	// Archiving non existing should error
	err = s.ArchiveRepo(w, 200, 199)
	if err == nil {
		t.Fatal("got no error deleting non existing")
	}

	// Create new and archive
	const userId = 100
	const repoName = "new-repo"
	r, _, err := s.CreateNew(w, userId, repoName, "repo description")
	if err != nil {
		t.Fatal(err)
	}
	// Check repo count
	isGreaterThanZero, err := s.NonArchivedRepoCountIsGreaterThan(w, userId, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !isGreaterThanZero {
		t.Fatalf("count not > 0")
	}
	isGreaterThan1, err := s.NonArchivedRepoCountIsGreaterThan(w, userId, 1)
	if err != nil {
		t.Fatal(err)
	}
	if isGreaterThan1 {
		t.Fatalf("count > 1")
	}
	err = s.ArchiveRepo(w, userId, r.Id)
	if err != nil {
		t.Fatal(err)
	}
	isGreaterThanZero, err = s.NonArchivedRepoCountIsGreaterThan(w, userId, 0)
	if err != nil {
		t.Fatal(err)
	}
	if isGreaterThanZero {
		t.Fatalf("count > 0 after archive")
	}
	// Try reading. Expect error.
	_, err = s.GetById(w, r.Id)
	if err == nil {
		t.Fatal("got no error getting archived")
	}
	_, isNotFoundErr, err := s.GetByOwnerIdAndRepoName(w, userId, repoName)
	if err == nil || !isNotFoundErr {
		t.Fatal("got no not-found error getting archived")
	}

	// Ok to create new one with same name
	r2, _, err := s.CreateNew(w, userId, repoName, "new repo description")
	if err != nil {
		t.Fatal(err)
	}
	if r2.Id == r.Id {
		t.Fatal("got same id")
	}
	_, _, err = s.GetByOwnerIdAndRepoName(w, userId, repoName)
	if err != nil {
		t.Fatal(err)
	}

	// Test iterator
	repoIds, err := s.GetArchivedRepoIds(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	if !repoIds.Next() {
		t.Fatal("expected 1 iter")
	}
	id, err := repoIds.Get()
	if err != nil {
		t.Fatal(err)
	}
	if id != r.Id {
		t.Fatalf("got archived id %d", id)
	}
	if repoIds.Next() {
		t.Fatal("expected only 1 iter")
	}
	err = repoIds.Err()
	if err != nil {
		t.Fatal(err)
	}

}

func TestSetGitMirrorUrl(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	secretsSrv, err := secrets.NewService(b, bytes.Repeat([]byte{1}, 32))
	if err != nil {
		log.Fatalf("unexpected err instantiating secrets srv: %s", err)
		return
	}
	s, err := NewService(b, secretsSrv)
	if err != nil {
		t.Fatal("unexpected instantiating err")
	}

	// Create new and archive
	const userId = 100
	const repoName = "new-repo"
	const gitMirrorUrl = "https://secret@useless-git-server.com/my/repo.git"
	const sanitizedGitMirrorUrl = "https://<token>@useless-git-server.com/my/repo.git"
	r, _, err := s.CreateNew(w, userId, repoName, "repo description")
	if err != nil {
		t.Fatal(err)
	}
	if r.SanitizedGitMirrorUrl != "" {
		t.Fatalf("new repo with SanitizedGitMirrorUrl=%q", r.SanitizedGitMirrorUrl)
	}
	err = s.SetGitMirrorUrl(w, r.Id, userId, repoName, gitMirrorUrl)
	if err != nil {
		t.Fatal(err)
	}
	r, err = s.GetById(w, r.Id)
	if err != nil {
		t.Fatal(err)
	}
	if r.SanitizedGitMirrorUrl != sanitizedGitMirrorUrl {
		t.Fatalf("got SanitizedGitMirrorUrl %s", r.SanitizedGitMirrorUrl)
	}

	// Test reading from all repos of user
	repos, err := s.GetAllByOwnerId(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	repos.Next()
	gotRepo, err := repos.Get()
	if err != nil {
		t.Fatal(err)
	}
	if gotRepo.SanitizedGitMirrorUrl != sanitizedGitMirrorUrl {
		t.Fatalf("got SanitizedGitMirrorUrl %s", r.SanitizedGitMirrorUrl)
	}

	// Archiving with mirror url should be ok
	err = s.ArchiveRepo(w, userId, r.Id)
	if err != nil {
		t.Fatal(err)
	}
}
func TestGetGitMirrorUrl(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	secretsSrv, err := secrets.NewService(b, bytes.Repeat([]byte{1}, 32))
	if err != nil {
		log.Fatalf("unexpected err instantiating secrets srv: %s", err)
		return
	}
	s, err := NewService(b, secretsSrv)
	if err != nil {
		t.Fatal("unexpected instantiating err")
	}
	// Create new and archive
	const userId = 100
	const repoName = "new-repo"
	const gitMirrorUrl = "https://secret@useless-git-server.com/my/repo.git"
	r, _, err := s.CreateNew(w, userId, repoName, "repo description")
	if err != nil {
		t.Fatal(err)
	}
	err = s.SetGitMirrorUrl(w, r.Id, userId, repoName, gitMirrorUrl)
	if err != nil {
		t.Fatal(err)
	}
	url, isNotFoundErr, err := s.GetGitMirrorUrl(w, r.Id)
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("should be false")
	}
	if url != gitMirrorUrl {
		t.Fatalf("got url %q wanted %q", url, gitMirrorUrl)
	}
}

func TestSetGitMirrorEnabled(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := b.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()
	secretsSrv, err := secrets.NewService(b, bytes.Repeat([]byte{1}, 32))
	if err != nil {
		log.Fatalf("unexpected err instantiating secrets srv: %s", err)
		return
	}
	s, err := NewService(b, secretsSrv)
	if err != nil {
		t.Fatal("unexpected instantiating err")
	}

	// Create repo
	const userId = 100
	const repoName = "enabled-repo"
	r, isAlreadyExistsErr, err := s.CreateNew(w, userId, repoName, "repo description")
	if err != nil {
		t.Fatal(err)
	}
	if isAlreadyExistsErr {
		t.Fatal("should not give this error")
	}

	if r.IsGitMirrorEnabled {
		t.Fatalf("expected new repo IsGitMirrorEnabled=false, got %v", r.IsGitMirrorEnabled)
	}

	// Enable mirror
	err = s.SetGitMirrorEnabled(w, userId, repoName, true)
	if err != nil {
		t.Fatal(err)
	}

	// Read back
	r, err = s.GetById(w, r.Id)
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsGitMirrorEnabled {
		t.Fatalf("expected IsGitMirrorEnabled=true, got false")
	}

	// Disable mirror
	err = s.SetGitMirrorEnabled(w, userId, repoName, false)
	if err != nil {
		t.Fatal(err)
	}

	// Read back again
	r, err = s.GetById(w, r.Id)
	if err != nil {
		t.Fatal(err)
	}
	if r.IsGitMirrorEnabled {
		t.Fatalf("expected IsGitMirrorEnabled=false after disabling, got true")
	}

	// Test inside GetAllByOwnerId iterator
	repos, err := s.GetAllByOwnerId(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	repos.Next()
	gotRepo, err := repos.Get()
	if err != nil {
		t.Fatal(err)
	}
	if gotRepo.IsGitMirrorEnabled {
		t.Fatalf("iterator repo expected IsGitMirrorEnabled=false, got %v", gotRepo.IsGitMirrorEnabled)
	}
}

func TestStripTokenAndSanitizedGitMirrorUrl(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		sanitized string
		valid     bool
	}{
		{
			name:      "valid https with token",
			input:     "https://github_pat_123@github.com/user/repo.git",
			sanitized: "https://<token>@github.com/user/repo.git",
			valid:     true,
		},
		{
			name:      "valid http with token",
			input:     "http://abc123@server.org/repo.git",
			sanitized: "http://<token>@server.org/repo.git",
			valid:     true,
		},
		{
			name:      "missing @ = invalid",
			input:     "https://github.com/user/repo.git",
			sanitized: "",
			valid:     false,
		},
		{
			name:      "non-http scheme = invalid",
			input:     "git@github.com:user/repo.git",
			sanitized: "",
			valid:     false,
		},
		{
			name:      "empty input = invalid",
			input:     "",
			sanitized: "",
			valid:     false,
		},
	}

	for _, tc := range tests {
		sanitized, valid := sanitizeGitMirrorUrl(tc.input)

		if sanitized != tc.sanitized {
			t.Errorf("expected sanitized %q, got %q", tc.sanitized, sanitized)
		}
		if valid != tc.valid {
			t.Errorf("expected valid=%v, got %v", tc.valid, valid)
		}
	}
}

func TestIsValidGitMirrorUrl(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"https with user:token", "https://x-token:abc123@github.com/owner/repo.git", true},
		{"http with user:token", "http://x-token:abc123@github.com/owner/repo.git", true},
		{"https with token only", "https://ghp_xxx@github.com/owner/repo.git", true},
		{"http with token only", "http://ghp_xxx@github.com/owner/repo.git", true},
		{"no credential", "https://github.com/owner/repo.git", false},
		{"empty", "", false},
		{"not a url", "just some text", false},
		{"http allowed (cleartext, but a valid transport)", "http://tok@github.com/owner/repo.git", true},
		{"git scheme rejected", "git://tok@github.com/owner/repo.git", false},
		{"ssh scheme rejected", "ssh://git@github.com/owner/repo.git", false},
		{"file scheme rejected", "file://tok@host/etc/passwd", false},
		{"ext transport rejected", "ext::sh -c 'touch /tmp/pwned'", false},
		{"ssh proxycommand host rejected", "ssh://-oProxyCommand=payload@host/path", false},
		{"dash host rejected even with token", "https://tok@-oProxyCommand=payload/path", false},
		{"missing host", "https:///path", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidGitMirrorUrl(tt.url)
			if got != tt.want {
				t.Fatalf("isValidGitMirrorUrl(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestGitMirrorUrlSecretName(t *testing.T) {
	if GitMirrorUrlSecretName != "git-mirror-secret-ulr" {
		t.Fatalf("webcomponent also use this value hardcoded, change in both places")
	}
}