package featureflags

import "context"

type Flags struct {
	RepoSecretsIsEnabled         bool
	CreateCdJobs                 bool // If set, CD jobs will be posted to the track server
	ShowCdJobs                   bool // If set, CD jobs will be shown in the UI
	OrganizationFeatureIsEnabled bool
	ShowCommitSize               bool
	EnableUserEducation          bool
	DummyFlag                    bool // Example flag just to serve as an example
	UseVSCodeDiff                bool // If set, the diff viewer uses the vendored VS Code diff engine
}

func GetFlags(configName string, repoOwnerUsername string, currentUsername string) Flags {
	return Flags{
		RepoSecretsIsEnabled:         true,
		CreateCdJobs:                 true,
		ShowCdJobs:                   true,
		OrganizationFeatureIsEnabled: true,
		ShowCommitSize:               true,
		EnableUserEducation:          true,
		DummyFlag: EnabledOutsideProd(configName) ||
			EnabledForReposOfTwiggers(repoOwnerUsername) ||
			EnabledForReposOfFriends(repoOwnerUsername),
		UseVSCodeDiff: EnabledOutsideProd(configName) ||
			EnabledForReposOfTwiggers(repoOwnerUsername),
	}
}

func EnabledOutsideProd(configName string) bool {
	return configName == "local" || configName == "mock" || configName == "lab"
}

func EnabledForReposOfTwiggers(repoOwnerUsername string) bool {
	return repoOwnerUsername == "andre" || repoOwnerUsername == "joao" || repoOwnerUsername == "marlon"
}
func EnabledForTwiggers(currentUsername string) bool {
	return currentUsername == "andre" || currentUsername == "joao" || currentUsername == "marlon"
}
func EnabledForReposOfFriends(repoOwnerUsername string) bool {
	// Not implemented for now but allows rolling out to partners
	return false
}

// MUST BE CREATED WITH NewFlagsHelper
type FlagsHelper struct {
	configName string
	u          UserService
}

func NewFlagsHelper(configName string, u UserService) FlagsHelper {
	return FlagsHelper{configName, u}
}

func (f FlagsHelper) GetFlagsByRepoOwnerUserId(repoOwnerUserId int64, tx context.Context) (Flags, error) {
	repoOwnerUsername, err := f.u.GetUsername(repoOwnerUserId, tx)
	if err != nil {
		return Flags{}, err
	}
	return GetFlags(f.configName, repoOwnerUsername, ""), nil
}
func (f FlagsHelper) GetFlagsByRepoOwnerUsername(repoOwnerUsername string, tx context.Context) (Flags, error) {
	return GetFlags(f.configName, repoOwnerUsername, ""), nil
}

type UserService interface {
	GetUsername(userId int64, tx context.Context) (string, error)
}