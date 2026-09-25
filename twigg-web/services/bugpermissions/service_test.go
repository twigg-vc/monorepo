package bugpermissions_test

import (
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/services/bugpermissions"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/webdb"
	"testing"
)

func TestOnlyRepoWritersCanCreateAndWriteBugs(t *testing.T) {
	db, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()
	owner, writer, reader, stranger := user.User{Id: 1}, user.User{Id: 2}, user.User{Id: 3}, user.User{Id: 4}
	rp := repo.Repo{Id: 10, OwnerId: owner.Id}
	for u, p := range map[int64]permissions.Permission{
		writer.Id: permissions.Permission_WriteRepo,
		reader.Id: permissions.Permission_ReadRepo,
	} {
		if _, err := db.GrantPermissionIfNotExists(w, u, p, permissions.RepoAssetId(rp.Id)); err != nil {
			t.Fatal(err)
		}
	}
	s := bugpermissions.NewService(db)
	check := func(name string, u *user.User, want bool) {
		t.Helper()
		canCreate, err := s.CanCreateBugs(w, u, rp)
		if err != nil {
			t.Fatal(err)
		}
		canWrite, err := s.CanWriteBug(w, u, rp, bug.Bug{Number: 1})
		if err != nil {
			t.Fatal(err)
		}
		if canCreate != want || canWrite != want {
			t.Fatalf("%s: expected %v, got canCreate=%v canWrite=%v", name, want, canCreate, canWrite)
		}
	}

	check("owner", &owner, true)
	check("writer", &writer, true)
	check("reader", &reader, false)
	check("stranger", &stranger, false)
	check("anonymous", nil, false)
}
