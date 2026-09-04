package seed

import (
	"fmt"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/services/repo"
	"monorepo/twigg-web/services/user"
	"monorepo/twigg-web/webdb"
)

func createUsersIfNotExistOrDie(seedUsers []SeedUser, db webdb.WebDb, u user.Service) {
	if len(seedUsers) == 0 {
		return
	}
	w, cl, commitTx, err := db.BeginWrite()
	if err != nil {
		panic(fmt.Sprintf("failed to open tx to createUsersIfNotExistOrDie: %s", err))
	}
	defer cl()
	for _, seedUser := range seedUsers {
		_, isNotFoundErr, err := u.GetByEmail(w, seedUser.Email)
		if !isNotFoundErr && err != nil {
			panic(fmt.Sprintf("failed to get user by email: %s", err))
		}
		if isNotFoundErr {
			usr, err := u.RegisterNewUser(w,
				seedUser.Email, seedUser.Username, seedUser.Password)
			if err != nil {
				panic(fmt.Sprintf("failed to created user %s: %s",
					seedUser.Email, err))
			}
			_, err = u.HandleManualPaymentSuccess(w, usr.Id, seedUser.Sub, seedUser.SubQuantity)
			if err != nil {
				panic(fmt.Sprintf("failed to pay seedUser payment: %s", err))
			}
		}
	}
	err = commitTx()
	if err != nil {
		panic(fmt.Sprintf("failed to commit tx: %s", err))
	}
}

func createRepoIfNotExistsOrDie(seedRepos []SeedRepo, db webdb.WebDb, u user.Service, rSrv repo.Service) {
	if len(seedRepos) == 0 {
		return
	}
	dbWrite, closeDbWrite, commitDbWrite, err := db.BeginWrite()
	defer closeDbWrite()
	if err != nil {
		panic("unable to get tx to createRepoIfNotExistsOrDie: " + err.Error())
	}
	for _, seedRepo := range seedRepos {
		owner, _, err := u.GetByUsername(dbWrite, seedRepo.RepoOwnerUsername)
		if err != nil {
			panic("failed to get owner by email: " + err.Error())
		}
		_, isNotFoundErr, err := rSrv.GetByOwnerIdAndRepoName(
			dbWrite, owner.Id, seedRepo.RepoName)
		if err != nil && !isNotFoundErr {
			panic(fmt.Sprintf("unable to check if seed repo exists: %s", err))
		}
		if isNotFoundErr {
			repo, _, err := rSrv.CreateNew(dbWrite, owner.Id, seedRepo.RepoName,
				seedRepo.RepoDescription)
			if err != nil {
				panic("unable to create seed repo: " + err.Error())
			}
			usersWithWritePerm := []user.User{}
			for _, uname := range seedRepo.UsernamesWithWritePerm {
				usr, _, err := u.GetByUsername(dbWrite, uname)
				if err != nil {
					panic("failed to get user by username: " + err.Error())
				}
				usersWithWritePerm = append(usersWithWritePerm, usr)
			}
			repoAssetId := permissions.RepoAssetId(repo.Id)
			for _, usrWithWritePerm := range usersWithWritePerm {
				_, err = db.GrantPermissionIfNotExists(
					dbWrite, usrWithWritePerm.Id,
					permissions.Permission_WriteRepo, repoAssetId)
				if err != nil {
					panic("err to granting permission: " + err.Error())
				}
			}

		}
	}
	err = commitDbWrite()
	if err != nil {
		panic(fmt.Sprintf("failed to commit tx:%s", err))
	}
}
