// This package contains some helper functions to seed the server's DB
// with some users/repos for testing/demos
package seed

import (
	"monorepo/twigg-web/services/repo"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/webdb"
)

// SeedUser represents a user we want to manually create in the server.
// All fields must be populated.
type SeedUser struct {
	Email       string
	Username    string
	Password    string
	Sub         user.SubscriptionPlan
	SubQuantity int64
}

// CreateUsersIfNotExist creates the users if they don't exist yet.
// Panics if anything goes wrong.
func CreateUsersIfNotExistOrDie(users []SeedUser, db webdb.WebDb, u userservice.Service) {
	createUsersIfNotExistOrDie(users, db, u)
}

// SeedRepo represents a repository we want to manually create in the server.
// All fields must be populated.
type SeedRepo struct {
	RepoOwnerUsername      string
	RepoName               string
	RepoDescription        string
	UsernamesWithWritePerm []string
}

// CreateRepoIfNotExistsOrDie creates repositories if they don't exist.
// Panics if anything goes wrong.
func CreateRepoIfNotExistsOrDie(seedRepos []SeedRepo, db webdb.WebDb, u userservice.Service, rSrv repo.Service) {
	createRepoIfNotExistsOrDie(seedRepos, db, u, rSrv)
}
