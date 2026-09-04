package webdb_test

import (
	"errors"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/webdb"
	"testing"
)

func TestCreateAndGetRepo(t *testing.T) {
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

	repoId, err := b.CreateRepo(w, 400, "new-repo", "repo description")
	if err != nil {
		t.Fatal(err)
	}
	if repoId != 1 {
		t.Fatalf("expected first repo id 1, got %d", repoId)
	}
	expected := repo.Repo{
		Id:                    1,
		OwnerId:               400,
		DisplayName:           "new-repo",
		Description:           "repo description",
		IsGitMirrorEnabled:    false,
		SanitizedGitMirrorUrl: "",
		IsPublic:              false,
	}

	gotById, err := b.GetRepoById(w, repoId)
	if err != nil {
		t.Fatal(err)
	}
	if gotById != expected {
		t.Fatalf("GetRepoById got %+v", gotById)
	}

	gotByName, isNotFoundErr, err := b.GetRepoByOwnerIdAndName(w, 400, "new-repo")
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("got is not found err")
	}
	if gotByName != expected {
		t.Fatalf("GetRepoByOwnerIdAndName got %+v", gotByName)
	}

	it, err := b.GetReposByOwnerId(w, 400)
	if err != nil {
		t.Fatal(err)
	}
	if !it.Next() {
		t.Fatal("expected 1 entry")
	}
	gotFromIter, err := it.Get()
	if err != nil {
		t.Fatal(err)
	}
	if gotFromIter != expected {
		t.Fatal("iterator returned wrong data")
	}
	if it.Next() {
		t.Fatal("should be done iterating")
	}
	err = it.Err()
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreatedRepoIsPrivate(t *testing.T) {
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

	const ownerId = 100
	const repoName = "new-repo"
	repoId, err := b.CreateRepo(w, ownerId, repoName, "repo description")
	if err != nil {
		t.Fatal(err)
	}

	r, err := b.GetRepoById(w, repoId)
	if err != nil {
		t.Fatal(err)
	}
	if r.IsPublic {
		t.Fatal("expected a newly created repo to be private")
	}

	// Archiving a repo must not fail because of the isPublic column.
	err = b.ArchiveRepo(w, ownerId, repoId)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSetRepoPublic(t *testing.T) {
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

	const ownerId = 100
	const repoName = "public-repo"
	repoId, err := b.CreateRepo(w, ownerId, repoName, "repo description")
	if err != nil {
		t.Fatal(err)
	}

	err = b.SetRepoPublic(w, ownerId, repoName)
	if err != nil {
		t.Fatal(err)
	}
	r, err := b.GetRepoById(w, repoId)
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsPublic {
		t.Fatal("expected IsPublic=true, got false")
	}

	// Setting an already public repo to public must be a no-op.
	err = b.SetRepoPublic(w, ownerId, repoName)
	if err != nil {
		t.Fatal(err)
	}
	r, err = b.GetRepoById(w, repoId)
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsPublic {
		t.Fatal("expected IsPublic=true after setting public twice, got false")
	}

	// Repos of other owners must not be affected.
	otherRepoId, err := b.CreateRepo(w, ownerId+1, repoName, "other description")
	if err != nil {
		t.Fatal(err)
	}
	other, err := b.GetRepoById(w, otherRepoId)
	if err != nil {
		t.Fatal(err)
	}
	if other.IsPublic {
		t.Fatal("expected repo of another owner to stay private")
	}
}

func TestSetRepoPrivate(t *testing.T) {
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

	const ownerId = 100
	const repoName = "private-again-repo"
	repoId, err := b.CreateRepo(w, ownerId, repoName, "repo description")
	if err != nil {
		t.Fatal(err)
	}
	err = b.SetRepoPublic(w, ownerId, repoName)
	if err != nil {
		t.Fatal(err)
	}

	err = b.SetRepoPrivate(w, ownerId, repoName)
	if err != nil {
		t.Fatal(err)
	}
	r, err := b.GetRepoById(w, repoId)
	if err != nil {
		t.Fatal(err)
	}
	if r.IsPublic {
		t.Fatal("expected IsPublic=false, got true")
	}

	// Setting an already private repo to private must be a no-op.
	err = b.SetRepoPrivate(w, ownerId, repoName)
	if err != nil {
		t.Fatal(err)
	}
	r, err = b.GetRepoById(w, repoId)
	if err != nil {
		t.Fatal(err)
	}
	if r.IsPublic {
		t.Fatal("expected IsPublic=false after setting private twice, got true")
	}
}

func TestSetRepoDescription(t *testing.T) {
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

	const ownerId = 100
	const repoName = "described-repo"
	const newDescription = "a new description"
	const otherDescription = "other description"
	repoId, err := b.CreateRepo(w, ownerId, repoName, "repo description")
	if err != nil {
		t.Fatal(err)
	}

	err = b.SetRepoDescription(w, ownerId, repoName, newDescription)
	if err != nil {
		t.Fatal(err)
	}
	r, err := b.GetRepoById(w, repoId)
	if err != nil {
		t.Fatal(err)
	}
	if r.Description != newDescription {
		t.Fatalf("expected description %q, got %q", newDescription, r.Description)
	}

	// Setting the same description again must be a no-op.
	err = b.SetRepoDescription(w, ownerId, repoName, newDescription)
	if err != nil {
		t.Fatal(err)
	}
	r, err = b.GetRepoById(w, repoId)
	if err != nil {
		t.Fatal(err)
	}
	if r.Description != newDescription {
		t.Fatalf("expected description %q after setting it twice, got %q", newDescription, r.Description)
	}

	// Repos of other owners must not be affected.
	otherRepoId, err := b.CreateRepo(w, ownerId+1, repoName, otherDescription)
	if err != nil {
		t.Fatal(err)
	}
	err = b.SetRepoDescription(w, ownerId, repoName, "changed again")
	if err != nil {
		t.Fatal(err)
	}
	other, err := b.GetRepoById(w, otherRepoId)
	if err != nil {
		t.Fatal(err)
	}
	if other.Description != otherDescription {
		t.Fatalf("expected repo of another owner to keep description %q, got %q", otherDescription, other.Description)
	}
}

func TestGetNonExistingRepo(t *testing.T) {
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

	_, err = b.GetRepoById(w, 199)
	if !errors.Is(err, webdb.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	_, isNotFoundErr, err := b.GetRepoByOwnerIdAndName(w, 10, "non-existing")
	if !isNotFoundErr || err == nil {
		t.Fatal("expected is not found err")
	}
	it, err := b.GetReposByOwnerId(w, 10)
	if err != nil {
		t.Fatal(err)
	}
	if it.Next() {
		t.Fatal("expected no repos")
	}
	err = it.Err()
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateRepoMissingDisplayName(t *testing.T) {
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

	_, err = b.CreateRepo(w, 400, "", "repo description")
	if err == nil {
		t.Fatal("expected error creating repo with empty displayName")
	}
}

func TestArchiveRepo(t *testing.T) {
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

	// Archiving non existing should error
	err = b.ArchiveRepo(w, 200, 199)
	if err == nil {
		t.Fatal("got no error archiving non existing")
	}

	const ownerId = 100
	repoId, err := b.CreateRepo(w, ownerId, "new-repo", "repo description")
	if err != nil {
		t.Fatal(err)
	}
	err = b.ArchiveRepo(w, ownerId, repoId)
	if err != nil {
		t.Fatal(err)
	}

	// Try reading. Expect not found.
	_, err = b.GetRepoById(w, repoId)
	if !errors.Is(err, webdb.ErrNotFound) {
		t.Fatalf("expected ErrNotFound getting archived, got %v", err)
	}
	_, isNotFoundErr, err := b.GetRepoByOwnerIdAndName(w, ownerId, "new-repo")
	if err == nil || !isNotFoundErr {
		t.Fatal("got no not-found error getting archived")
	}

	// Ok to create new one with same name; ids must not be reused
	repoId2, err := b.CreateRepo(w, ownerId, "new-repo", "new repo description")
	if err != nil {
		t.Fatal(err)
	}
	if repoId2 == repoId {
		t.Fatal("got same id")
	}

	// Test iterator
	repoIds, err := b.GetArchivedRepoIds(w, ownerId)
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
	if id != repoId {
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

func TestSetRepoGitMirrorEnabled(t *testing.T) {
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

	const ownerId = 100
	const repoName = "enabled-repo"
	repoId, err := b.CreateRepo(w, ownerId, repoName, "repo description")
	if err != nil {
		t.Fatal(err)
	}

	err = b.SetRepoGitMirrorEnabled(w, ownerId, repoName, true)
	if err != nil {
		t.Fatal(err)
	}
	r, err := b.GetRepoById(w, repoId)
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsGitMirrorEnabled {
		t.Fatal("expected IsGitMirrorEnabled=true, got false")
	}

	err = b.SetRepoGitMirrorEnabled(w, ownerId, repoName, false)
	if err != nil {
		t.Fatal(err)
	}
	r, err = b.GetRepoById(w, repoId)
	if err != nil {
		t.Fatal(err)
	}
	if r.IsGitMirrorEnabled {
		t.Fatal("expected IsGitMirrorEnabled=false after disabling, got true")
	}
}

func TestSetRepoSanitizedGitMirrorUrl(t *testing.T) {
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

	const ownerId = 100
	const repoName = "new-repo"
	const sanitizedUrl = "https://<token>@useless-git-server.com/my/repo.git"
	repoId, err := b.CreateRepo(w, ownerId, repoName, "repo description")
	if err != nil {
		t.Fatal(err)
	}

	err = b.SetRepoSanitizedGitMirrorUrl(w, ownerId, repoName, sanitizedUrl)
	if err != nil {
		t.Fatal(err)
	}
	r, err := b.GetRepoById(w, repoId)
	if err != nil {
		t.Fatal(err)
	}
	if r.SanitizedGitMirrorUrl != sanitizedUrl {
		t.Fatalf("got SanitizedGitMirrorUrl %s", r.SanitizedGitMirrorUrl)
	}

	// Test reading from all repos of owner
	it, err := b.GetReposByOwnerId(w, ownerId)
	if err != nil {
		t.Fatal(err)
	}
	it.Next()
	gotRepo, err := it.Get()
	if err != nil {
		t.Fatal(err)
	}
	if gotRepo.SanitizedGitMirrorUrl != sanitizedUrl {
		t.Fatalf("got SanitizedGitMirrorUrl %s", gotRepo.SanitizedGitMirrorUrl)
	}

	// Archiving with mirror url should be ok
	err = b.ArchiveRepo(w, ownerId, repoId)
	if err != nil {
		t.Fatal(err)
	}
}