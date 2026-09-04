package newrepo

import (
	"context"
	"log"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/repo"
	twiggwc "monorepo/twigg-web/webcomponents"
	"monorepo/twigg-web/wrappers"
	"net/http"
	"regexp"
)

var repoNameRE = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type handler struct {
	canCreate CanCreateRepo
	rSrv      RepoService
	permSrv   PermissionsService
}

func newHandler(canCreate CanCreateRepo, rSrv RepoService, permSrv PermissionsService) handler {
	return handler{
		canCreate: canCreate,
		rSrv:      rSrv,
		permSrv:   permSrv,
	}
}

func (hl handler) handleGet(w http.ResponseWriter,
	r wrappers.UserWithSubMuxRequest, dbRead context.Context) {
	orgName := ""
	if r.HaveOrgParamInRequest {
		orgName = r.OrgWithSub.Username
	}
	twiggwc.Page( /*hideNavBar=*/ false, r.Flags, twiggwc.NewRepo(orgName)).Render(w)
}

func (hl handler) handlePost(w http.ResponseWriter,
	r wrappers.UserWithSubMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	owner := r.UserWithSub
	if r.HaveOrgParamInRequest {
		owner = r.OrgWithSub
	}

	canCreate, err := hl.canCreate.CanCreateRepo(owner, dbWrite)
	if err != nil {
		log.Printf("failed to check if userId=%d can create repo: %s", owner.Id, err)
		http.Error(w, "failed to check if can create repo", http.StatusInternalServerError)
		return
	}
	if !canCreate {
		http.Error(w, "user cant create repo", http.StatusBadRequest)
		return
	}

	newRepoName := r.FormValue(routes.NewRepoNameParameterName)
	newRepoDescriptiom := r.FormValue(routes.NewRepoDescriptionParameterName)

	const maxRepoNameLen = 64
	if newRepoName == "" || len(newRepoName) > maxRepoNameLen {
		http.Error(w, "invalid repo name: must be 1-64 chars", http.StatusBadRequest)
		return
	}

	if !repoNameRE.MatchString(newRepoName) {
		http.Error(w, "invalid repo name: use only letters, numbers, _ and -", http.StatusBadRequest)
		return
	}

	if len(newRepoDescriptiom) > repo.MaxDescriptionLength {
		http.Error(w, "invalid repo name: must be 1-100 chars", http.StatusBadRequest)
		return
	}

	newRepo, isAlreadyExistsErr, err := hl.rSrv.CreateNew(dbWrite, owner.Id, newRepoName, newRepoDescriptiom)
	if isAlreadyExistsErr {
		http.Error(w, "invalid. Repository already exist", http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Printf("failed to create repo: %s", err)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	if r.HaveOrgParamInRequest {
		if err := hl.grantOrgMembersAndOwnersWritePermission(dbWrite, r.OrgWithSub.Id, newRepo.Id); err != nil {
			log.Printf("failed to grant org members write perm for repoId=%d: %s", newRepo.Id, err)
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
	}

	shouldCommit = true
	return
}

func (hl handler) grantOrgMembersAndOwnersWritePermission(
	dbWrite context.Context, orgId int64, repoId uint64,
) error {
	orgAssetId := permissions.OrganizationAssetId(orgId)
	repoAssetId := permissions.RepoAssetId(repoId)

	permissionsToGrant := []permissions.Permission{
		permissions.Permission_OrganizationOwner,
		permissions.Permission_OrganizationMember,
	}

	for _, orgPerm := range permissionsToGrant {
		it, err := hl.permSrv.GetUsersWithPermission(dbWrite, orgAssetId, orgPerm)
		if err != nil {
			return err
		}
		for it.Next() {
			userId, err := it.Get()
			if err != nil {
				return err
			}
			_, err = hl.permSrv.GrantPermissionIfNotExists(
				dbWrite,
				userId,
				permissions.Permission_WriteRepo,
				repoAssetId,
			)
			if err != nil {
				return err
			}
		}
		if err := it.Err(); err != nil {
			return err
		}
	}
	return nil
}