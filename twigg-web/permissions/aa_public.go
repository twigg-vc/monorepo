package permissions

import (
	"fmt"
	"strconv"
	"strings"
)

type Permission int64

const (
	Permission_None Permission = iota
	// asset id = Use RepoAssAssetId()
	Permission_ReadRepo
	// asset id = Use RepoAssAssetId()
	Permission_WriteRepo
	// asset id = Use OrganizationAssetId()
	Permission_OrganizationOwner
	// asset id = Use OrganizationAssetId()
	Permission_OrganizationMember
)

func OrganizationAssetId(organizationId int64) string {
	return fmt.Sprintf("%s%d", organizationPrefix, organizationId)
}

func ParseOrganizationAssetIdOrDie(organizationAssetId string) int64 {
	if !strings.HasPrefix(organizationAssetId, organizationPrefix) {
		panic(fmt.Sprintf("unexpected organizationAssetId: %s", organizationAssetId))
	}
	orgId, err := strconv.ParseInt(
		organizationAssetId[len(organizationPrefix):], // trim prefix
		10,
		64,
	)
	if err != nil {
		panic(fmt.Sprintf("unexpected organizationAssetId: %s", organizationAssetId))
	}
	return orgId
}

func RepoAssetId(repoId uint64) string {
	return strconv.FormatUint(repoId, 10)
}

func ParseRepoAssetIdOrDie(repoAssetId string) uint64 {
	u, err := strconv.ParseUint(repoAssetId, 10, 64)
	if err != nil {
		panic(fmt.Sprintf("unexpected repoAssetId: %s", repoAssetId))
	}
	return u
}

var MaxNumberOfOwnersInOrg int = 1_000
var MaxNumberOfMembersInOrg int = 1_000

const organizationPrefix = "org-"
