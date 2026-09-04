package reposettings

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/secrets"
	"monorepo/twigg-web/services/repo"
	"monorepo/twigg-web/webcomponents"
	"monorepo/twigg-web/wrappers"
	"monorepo/twigg/client"
	"monorepo/twigg/xchange"
	"net/http"
	"strconv"
)

func (h handler) handleGetRepoSettings(w http.ResponseWriter, r wrappers.UserRepoMuxRequest, dbRead context.Context) {
	Members := []webcomponents.RepositorySettingsMember{}
	const maxMembers = 1_000

	it, err := h.db.GetUsersWithPermission(
		dbRead, permissions.RepoAssetId(r.Repo.Id),
		permissions.Permission_WriteRepo)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// TODO implement "Owner" role
	Members = append(Members, webcomponents.RepositorySettingsMember{
		Username: r.RepoOwnerUsr.Username,
		Role:     "Owner",
	})
	for it.Next() {
		if len(Members) > maxMembers {
			http.Error(w, "Maximum number of collaborators", http.StatusInternalServerError)
			return
		}
		idOfUserPermissions, err := it.Get()
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		user, _, err := h.userS.Get(dbRead, idOfUserPermissions)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		Members = append(Members, webcomponents.RepositorySettingsMember{
			Username: user.Username,
			Role:     "Read/Write",
		})
	}
	err = it.Err()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	secrets := []secrets.SecretRef{}
	if r.Flags.RepoSecretsIsEnabled {
		secrets, err = h.secrets.GetRepoIdSecretsPage(dbRead, r.Repo.Id, 0)
		if err != nil {
			log.Printf("failed to GetRepoIdSecretsPage for Repo.Id=%v in handleGetRepoSettings. err=%q", r.Repo.Id, err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	webcomponents.Page( /*hideNavBar*/ false,
		r.Flags,
		webcomponents.RepoSettings(r.RepoOwnerUsr.Username,
			r.Repo.DisplayName, r.Repo.Description, r.Repo.IsGitMirrorEnabled,
			r.Repo.SanitizedGitMirrorUrl, Members, secrets, r.Repo.IsPublic,
		),
	).Render(w)
}

func (h handler) handlePostRemoveRepoPermission(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool) {

	userToRevokePermission, isNotFoundErr, err := h.userS.GetByUsername(
		dbWrite, r.FormValue(routes.UsernameParameterName))
	if isNotFoundErr {
		const notFoundMsg = "not found"
		w.Write([]byte(notFoundMsg))
		return
	}
	if err != nil {
		http.Error(w, "Internal err getting usr", http.StatusInternalServerError)
		return
	}

	if r.RepoOwnerUsr.IsOrganization {
		hasPerm, err := h.db.HasPermission(dbWrite, userToRevokePermission.Id, permissions.Permission_OrganizationOwner, permissions.OrganizationAssetId(r.RepoOwnerUsr.Id))
		if err != nil {
			http.Error(w, "Internal err checking permissions", http.StatusInternalServerError)
			return
		}
		if hasPerm {
			http.Error(w, "can not revoke permission from a org owner", http.StatusBadRequest)
			return
		}
	}

	err = h.db.RevokePermissionIfExists(dbWrite, userToRevokePermission.Id,
		permissions.Permission_WriteRepo, permissions.RepoAssetId(r.Repo.Id))

	if err != nil {
		http.Error(w, "Failed to revoke permission", http.StatusInternalServerError)
		return
	}

	const okMsg = "ok"
	_, err = w.Write([]byte(okMsg))
	shouldCommit = err == nil
	return
}

func (h handler) handlePostAddRepoPermission(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool) {

	userToGrantPermission, isNotFoundErr, err := h.userS.GetByUsername(
		dbWrite, r.FormValue(routes.UsernameParameterName))
	if isNotFoundErr {
		const notFoundMsg = "not found"
		w.Write([]byte(notFoundMsg))
		return
	}
	if err != nil {
		http.Error(w, "Internal err getting usr", http.StatusInternalServerError)
		return
	}

	const hasPermissionMsg = "Already had"
	if userToGrantPermission.Id == r.RepoOwnerUsr.Id {
		w.Write([]byte(hasPermissionMsg))
		return
	}

	if r.RepoOwnerUsr.IsOrganization {
		hasOwnerPerm, err := h.db.HasPermission(dbWrite, userToGrantPermission.Id, permissions.Permission_OrganizationOwner, permissions.OrganizationAssetId(r.RepoOwnerUsr.Id))
		if err != nil {
			http.Error(w, "Internal err checking permissions", http.StatusInternalServerError)
			return
		}
		hasMemberPerm, err := h.db.HasPermission(dbWrite, userToGrantPermission.Id, permissions.Permission_OrganizationMember, permissions.OrganizationAssetId(r.RepoOwnerUsr.Id))
		if err != nil {
			http.Error(w, "Internal err checking permissions", http.StatusInternalServerError)
			return
		}
		if !hasOwnerPerm && !hasMemberPerm {
			http.Error(w, "can not add user that is not part of the organization", http.StatusBadRequest)
			return
		}
	}

	hadPermission, err := h.db.GrantPermissionIfNotExists(dbWrite, userToGrantPermission.Id,
		permissions.Permission_WriteRepo, permissions.RepoAssetId(r.Repo.Id))

	if err != nil {
		http.Error(w, "Failed to grant permission", http.StatusInternalServerError)
		return
	}
	if hadPermission {
		w.Write([]byte(hasPermissionMsg))
		return
	}
	const okMsg = "ok"
	_, err = w.Write([]byte(okMsg))
	shouldCommit = err == nil
	return
}

func (h handler) handlePostArchive(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	if r.RepoOwnerUsr.IsOrganization {
		isOrgOwner, err := h.db.HasPermission(dbWrite, r.UserWithWritePermission.Id,
			permissions.Permission_OrganizationOwner, permissions.OrganizationAssetId(r.RepoOwnerUsr.Id))
		if err != nil {
			log.Printf("failed to check org owner permission. org:%v user:%v. err=%q", r.RepoOwnerUsr.Id, r.RepoOwnerUsr.Id, err)
			http.Error(w, "failed to check org owner permission", http.StatusInternalServerError)
			return
		}
		if !isOrgOwner {
			http.Error(w, "only org owner can archive org repo", http.StatusBadRequest)
			return
		}
	} else {
		if r.UserWithWritePermission.Id != r.Repo.OwnerId {
			http.Error(w, "only owner can archive repo", http.StatusBadRequest)
			return
		}
	}
	err := h.db.RevokeAllPermissionsToAsset(dbWrite, permissions.RepoAssetId(r.Repo.Id))
	if err != nil {
		log.Printf("failed to revoke perms to repoId=%d repoName=%s ownrId=%d: %s",
			r.Repo.Id, r.Repo.DisplayName, r.Repo.OwnerId, err)
		http.Error(w, "failed to archive", http.StatusInternalServerError)
		return
	}
	err = h.repoS.ArchiveRepo(dbWrite, r.Repo.OwnerId, r.Repo.Id)
	if err != nil {
		log.Printf("failed to archive repoId=%d repoName=%s ownrId=%d: %s",
			r.Repo.Id, r.Repo.DisplayName, r.Repo.OwnerId, err)
		http.Error(w, "failed to archive", http.StatusInternalServerError)
		return
	}
	const okMsg = "ok"
	_, err = w.Write([]byte(okMsg))
	if err != nil {
		log.Printf("failed to write ok for archival repoId=%d repoName=%s ownrId=%d: %s",
			r.Repo.Id, r.Repo.DisplayName, r.Repo.OwnerId, err)
	}
	shouldCommit = err == nil
	return
}

func (h handler) handlePostSetPublic(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest,
	dbWrite context.Context) (shouldCommit bool) {

	err := h.repoS.SetPublic(dbWrite, r.Repo.OwnerId, r.Repo.DisplayName)
	if err != nil {
		log.Printf("failed to set repo public repoId=%d repoName=%s ownerId=%d: %s", r.Repo.Id, r.Repo.DisplayName, r.Repo.OwnerId, err)
		http.Error(w, "failed to set repo public", http.StatusInternalServerError)
		return
	}

	const okMsg = "ok"
	_, err = w.Write([]byte(okMsg))
	if err != nil {
		log.Printf("failed to write ok for set repo public repoId=%d repoName=%s ownerId=%d: %s", r.Repo.Id, r.Repo.DisplayName, r.Repo.OwnerId, err)
	}
	shouldCommit = err == nil
	return
}

func (h handler) handlePostSetServerId(w http.ResponseWriter,
	r wrappers.CliKeyAuthMuxRequest,
	dbWrite context.Context) (shouldCommit bool) {

	xchange.SetTwiggHeaderInResponse(w)
	id, err := strconv.ParseUint(
		r.URL.Query().Get(client.SetServerIdQueryParam), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	s, isNotFoundErr, err := h.repoS.GetServer(dbWrite, r.RepoOwnerUsr.Id,
		r.Repo.DisplayName)
	if isNotFoundErr {
		http.Error(w, "repo not found", http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Printf("failed to get server repoId=%d: %s", r.Repo.Id, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	err = s.SetNextServerId(id, h.repoS.GetServerWrite(dbWrite))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	const okMsg = "ok"
	_, err = w.Write([]byte(okMsg))
	if err != nil {
		log.Printf("failed to write ok for set server id repoId=%d: %s", r.Repo.Id, err)
	}
	shouldCommit = err == nil
	return
}

func (h handler) handlePostSetPrivate(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest,
	dbWrite context.Context) (shouldCommit bool) {

	err := h.repoS.SetPrivate(dbWrite, r.Repo.OwnerId, r.Repo.DisplayName)
	if err != nil {
		log.Printf("failed to set repo private repoId=%d repoName=%s ownerId=%d: %s",
			r.Repo.Id, r.Repo.DisplayName, r.Repo.OwnerId, err)
		http.Error(w, "failed to set repo private", http.StatusInternalServerError)
		return
	}

	const okMsg = "ok"
	_, err = w.Write([]byte(okMsg))
	if err != nil {
		log.Printf("failed to write ok for set repo private repoId=%d repoName=%s ownerId=%d: %s",
			r.Repo.Id, r.Repo.DisplayName, r.Repo.OwnerId, err)
	}
	shouldCommit = err == nil
	return
}

func (h handler) handlePostSetDescription(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest,
	dbWrite context.Context) (shouldCommit bool) {

	description := r.Request.FormValue(routes.RepoDescriptionParamName)
	if len(description) > repo.MaxDescriptionLength {
		http.Error(w, "got too long description", http.StatusBadRequest)
		return
	}

	err := h.db.SetRepoDescription(dbWrite, r.Repo.OwnerId, r.Repo.DisplayName, description)
	if err != nil {
		log.Printf("failed to set repo description repoId=%d repoName=%s ownerId=%d: %s",
			r.Repo.Id, r.Repo.DisplayName, r.Repo.OwnerId, err)
		http.Error(w, "failed to set repo description", http.StatusInternalServerError)
		return
	}

	const okMsg = "ok"
	_, err = w.Write([]byte(okMsg))
	if err != nil {
		log.Printf("failed to write ok for set repo description repoId=%d repoName=%s ownerId=%d: %s",
			r.Repo.Id, r.Repo.DisplayName, r.Repo.OwnerId, err)
	}
	shouldCommit = err == nil
	return
}

func (h handler) handlePostGitMirrorEnabled(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest,
	dbWrite context.Context) (shouldCommit bool) {

	raw := r.Request.FormValue(routes.GitMirrorEnabledParamName)

	const gitMirrorIsEnabledParamValue string = "on"
	const gitMirrorIsDisabledParamValue string = "off"
	var enabled bool
	switch raw {
	case gitMirrorIsEnabledParamValue:
		enabled = true
	case gitMirrorIsDisabledParamValue:
		enabled = false
	default:
		http.Error(w, "invalid git enabled value", http.StatusBadRequest)
		return
	}

	err := h.repoS.SetGitMirrorEnabled(dbWrite,
		r.Repo.OwnerId,
		r.Repo.DisplayName,
		enabled,
	)
	if err != nil {
		log.Printf(
			"failed to set git mirror enabled repoId=%d repoName=%s ownerId=%d enabled=%v: %s",
			r.Repo.Id, r.Repo.DisplayName, r.Repo.OwnerId, enabled, err,
		)
		http.Error(w, "failed to set git mirror enabled", http.StatusInternalServerError)
		return
	}

	const okMsg = "ok"
	_, err = w.Write([]byte(okMsg))
	if err != nil {
		log.Printf(
			"failed to write response for set git mirror enabled repoId=%d repoName=%s ownerId=%d: %s",
			r.Repo.Id, r.Repo.DisplayName, r.Repo.OwnerId, err,
		)
		return
	}

	shouldCommit = true
	return
}

func (h handler) handlePostGitMirrorUrl(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool) {

	const gitUrlParameterName = "url"
	gitUrl := r.Request.FormValue(gitUrlParameterName)
	if len(gitUrl) > 1000 {
		http.Error(w, "got too long url", http.StatusBadRequest)
		return
	}
	if !repo.IsValidGitMirrorUrl(gitUrl) {
		http.Error(w, "got invalid url", http.StatusBadRequest)
		return
	}
	err := h.repoS.SetGitMirrorUrl(dbWrite, r.Repo.Id, r.Repo.OwnerId,
		r.Repo.DisplayName, gitUrl)
	if err != nil {
		log.Printf("failed to set git mirror of repo repoId=%d repoName=%s ownrId=%d to %s: %s",
			r.Repo.Id, r.Repo.DisplayName, r.Repo.OwnerId, gitUrl, err)
		http.Error(w, "failed to set git mirror url", http.StatusInternalServerError)
		return
	}
	const okMsg = "ok"
	_, err = w.Write([]byte(okMsg))
	if err != nil {
		log.Printf("failed to write ok set git mirror of repoId=%d repoName=%s ownrId=%d: %s",
			r.Repo.Id, r.Repo.DisplayName, r.Repo.OwnerId, err)
	}
	shouldCommit = err == nil
	return
}
func (h handler) handlePutSetRepoSecret(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest,
	dbWrite context.Context) (shouldCommit bool) {
	if !r.Flags.RepoSecretsIsEnabled {
		http.Error(w, "feature is disabled", http.StatusServiceUnavailable)
		return
	}
	secretName := r.Request.FormValue(routes.RepoSecretNameParamName)
	if secretName == "" {
		http.Error(w, "invalid secretName", http.StatusBadRequest)
		return
	}
	if secretName == repo.GitMirrorUrlSecretName {
		http.Error(w, "reserved secret name", http.StatusBadRequest)
		return
	}
	secretValue := r.Request.FormValue(routes.RepoSecretValueParamName)
	if secretValue == "" {
		http.Error(w, "invalid secretValue", http.StatusBadRequest)
		return
	}

	newSecret, err := h.secrets.SetRepoIdSecret(dbWrite, r.Repo.Id, secretName, secretValue)
	if err != nil {
		log.Printf("failed to set repo secret, repoId=%d secretName=%q. err=%q", r.Repo.Id, secretName, err)
		http.Error(w, "failed to set repo secret", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	newSecretJson, err := json.Marshal(newSecret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = w.Write(newSecretJson)
	if err != nil {
		log.Printf("failed to write response for set repo secret repoId=%d secretName=%q.err=%q", r.Repo.Id, secretName, err)
		http.Error(w, "internal error in set repo secret", http.StatusInternalServerError)
		return
	}

	shouldCommit = true
	return
}
func (h handler) handlePostSetRepoSecret(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest,
	dbWrite context.Context) (shouldCommit bool) {
	if !r.Flags.RepoSecretsIsEnabled {
		http.Error(w, "feature is disabled", http.StatusServiceUnavailable)
		return
	}
	secretName := r.Request.FormValue(routes.RepoSecretNameParamName)
	if secretName == "" {
		http.Error(w, "invalid secretName", http.StatusBadRequest)
		return
	}
	if secretName == repo.GitMirrorUrlSecretName {
		http.Error(w, "reserved secret name", http.StatusBadRequest)
		return
	}
	secretValue := r.Request.FormValue(routes.RepoSecretValueParamName)
	if secretValue == "" {
		http.Error(w, "invalid secretValue", http.StatusBadRequest)
		return
	}
	hasSecret, err := h.secrets.RepoIdHasSecret(dbWrite, r.Repo.Id, secretName)
	if err != nil {
		log.Printf("failed to check if repo has secret, repoId=%d secretName=%q. err=%q", r.Repo.Id, secretName, err)
		http.Error(w, "failed to set repo secret", http.StatusInternalServerError)
		return
	}
	if hasSecret {
		http.Error(w, "secret already exist", http.StatusBadRequest)
		return
	}

	newSecret, err := h.secrets.SetRepoIdSecret(dbWrite, r.Repo.Id, secretName, secretValue)
	if err != nil {
		log.Printf("failed to set repo secret, repoId=%d secretName=%q. err=%q", r.Repo.Id, secretName, err)
		http.Error(w, "failed to set repo secret", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	newSecretJson, err := json.Marshal(newSecret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = w.Write(newSecretJson)
	if err != nil {
		log.Printf("failed to write response for set repo secret repoId=%d secretName=%q.err=%q", r.Repo.Id, secretName, err)
		http.Error(w, "internal error in set repo secret", http.StatusInternalServerError)
		return
	}

	shouldCommit = true
	return
}

func (h handler) handleDeleteRepoSecret(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest,
	dbWrite context.Context) (shouldCommit bool) {
	if !r.Flags.RepoSecretsIsEnabled {
		http.Error(w, "feature is disabled", http.StatusServiceUnavailable)
		return
	}
	secretName := r.URL.Query().Get(routes.RepoSecretNameParamName)
	if secretName == "" {
		http.Error(w, "invalid secretName", http.StatusBadRequest)
		return
	}
	if secretName == repo.GitMirrorUrlSecretName {
		http.Error(w, "reserved secret name", http.StatusBadRequest)
		return
	}

	err := h.secrets.DeleteRepoIdSecretIfExists(dbWrite, r.Repo.Id, secretName)
	if err != nil {
		log.Printf("failed to delete repo secret, repoId=%d secretName=%q. err=%q", r.Repo.Id, secretName, err)
		http.Error(w, "failed to delete repo secret", http.StatusInternalServerError)
		return
	}

	shouldCommit = true
	return
}

func (h handler) getPayloadDisplayString(payload []byte) string {
	var args pushTopToGitMirrorPayloadArgs
	err := args.decode(payload)
	if err != nil {
		return "bad gitMirror payload: " + string(payload)
	}
	return fmt.Sprintf("push top of repoId=%d to gitMirror", args.RepoId)
}

func (h handler) handleQueuePushToGitMirror(payload []byte) error {
	var args pushTopToGitMirrorPayloadArgs

	err := args.decode(payload)
	if err != nil {
		return err
	}

	dbRead, closeDbRead, err := h.db.BeginRead()
	defer closeDbRead()
	if err != nil {
		err = fmt.Errorf("got err=%s getting dbRead in handleQueuePushToGitMirror", err)
		return err
	}

	s, err := h.repoS.GetServerByRepoId(dbRead, args.RepoId)
	if err != nil {
		err = fmt.Errorf("got err=%s getting Server in handleQueuePushToGitMirror", err)
		return err
	}

	const maxWorkdirSize = 500 * 1024 * 1024 // 500 MB
	err = h.mirrorSrv.PushTopCommit(h.repoS.GetServerRead(dbRead), s,
		args.GitRepoUrl, maxWorkdirSize)
	if err != nil {
		err = fmt.Errorf("got err=%s pushing to mirror in handleQueuePushToGitMirror", err)
		return err
	}
	return nil
}

type pushTopToGitMirrorPayloadArgs struct {
	RepoId     uint64
	GitRepoUrl string
}

func (a *pushTopToGitMirrorPayloadArgs) encode() ([]byte, error) {
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(a)
	if err != nil {
		return nil, fmt.Errorf("failed to encode pushToGitMirrorPayload err=%s", err)
	}
	return buf.Bytes(), nil
}
func (a *pushTopToGitMirrorPayloadArgs) decode(payload []byte) error {
	err := gob.NewDecoder(bytes.NewBuffer(payload)).Decode(a)
	if err != nil {
		return fmt.Errorf("failed to decode pushToGitMirrorPayload err=%s", err)
	}
	return nil
}

func pushToGitMirrorPayload(repoId uint64,
	gitRepoUrl string) (string, []byte, error) {
	args := pushTopToGitMirrorPayloadArgs{
		RepoId:     repoId,
		GitRepoUrl: gitRepoUrl,
	}
	payload, err := args.encode()
	return pushToGitMirrorPayloadType, payload, err
}

const pushToGitMirrorPayloadType = "push-to-git-mirror"