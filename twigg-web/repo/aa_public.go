package repo

const DemoRepoName = "Example-repository"
const DemoRepoDescription = "Example repository"

// MUST BE INITIALIZED WITH NewRepo.
// Repo contains all the data related to a repository.
type Repo struct {
	Id                    uint64
	OwnerId               int64 // Id of user that created repo.
	DisplayName           string
	Description           string
	IsGitMirrorEnabled    bool
	SanitizedGitMirrorUrl string // Safe to display for users.
	IsPublic              bool
}

// NewRepo constructs a Repo instance with all the required fields
func NewRepo(
	id uint64,
	ownerId int64,
	displayName string,
	description string,
	isGitMirrorEnabled bool,
	sanitizedGitMirrorUrl string,
	isPublic bool) Repo {
	return Repo{
		Id:                    id,
		OwnerId:               ownerId,
		DisplayName:           displayName,
		Description:           description,
		IsGitMirrorEnabled:    isGitMirrorEnabled,
		SanitizedGitMirrorUrl: sanitizedGitMirrorUrl,
		IsPublic:              isPublic,
	}
}