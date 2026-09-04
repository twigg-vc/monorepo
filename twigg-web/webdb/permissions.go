package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"monorepo/base/iterator"
	"monorepo/twigg-web/permissions"
	"strings"
)

func (db webDb) HasPermission(ctx context.Context, userId int64, p permissions.Permission, assetId string) (bool, error) {
	var uId int64
	err := db.s.QueryRow(ctx, `
		SELECT
			userId
		FROM permissions
		WHERE
			userId = ? AND permission = ? AND assetId = ?
	`, userId, p, assetId).Scan(
		&uId,
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("failed to query permission: %s", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return true, nil
}

func (db webDb) GrantPermissionIfNotExists(ctx context.Context, userId int64, p permissions.Permission, assetId string) (bool, error) {
	hasPermission, err := db.HasPermission(ctx, userId, p, assetId)
	if err != nil {
		return false, err
	}
	if hasPermission {
		return true, nil
	}
	switch p {
	case permissions.Permission_OrganizationOwner:
		count, err := db.CountUsersWithPermission(ctx, assetId, p)
		if err != nil {
			return false, err
		}
		if count >= int64(permissions.MaxNumberOfOwnersInOrg) {
			return false, fmt.Errorf("organization already has the maximum number of owners (%d)", permissions.MaxNumberOfOwnersInOrg)
		}
	case permissions.Permission_OrganizationMember:
		count, err := db.CountUsersWithPermission(ctx, assetId, p)
		if err != nil {
			return false, err
		}
		if count >= int64(permissions.MaxNumberOfMembersInOrg) {
			return false, fmt.Errorf("organization already has the maximum number of members (%d)", permissions.MaxNumberOfMembersInOrg)
		}
	}
	_, err = db.s.Exec(ctx, `
		INSERT INTO permissions (
			userId, permission, assetId
		) VALUES (?, ?, ?)
	`, userId, p, assetId)
	if err != nil {
		return false, fmt.Errorf("failed to grant permission: %s", err)
	}
	return false, nil
}

func (db webDb) RevokePermissionIfExists(ctx context.Context, userId int64, p permissions.Permission, assetId string) error {
	hasPermission, err := db.HasPermission(ctx, userId, p, assetId)
	if err != nil {
		return err
	}
	if !hasPermission {
		return nil
	}

	_, err = db.s.Exec(ctx, `
		DELETE FROM
			permissions
		WHERE
			userId = ? AND permission = ? AND assetId = ?
	`, userId, p, assetId)
	if err != nil {
		return fmt.Errorf("failed to revoke permission: %s", err)
	}
	return nil
}

func (db webDb) RevokeAllPermissionsToAsset(ctx context.Context, assetId string) error {
	_, err := db.s.Exec(ctx, `
		DELETE FROM
			permissions
		WHERE
			assetId = ?
	`, assetId)
	if err != nil {
		return fmt.Errorf("failed to revoke all permissions to asset %s: %s", assetId, err)
	}
	return err
}

func (db webDb) GetUserPermissions(ctx context.Context, userId int64) (iterator.I[permissions.Permission], error) {
	rows, err := db.s.Query(ctx, `
		SELECT
			permission
		FROM permissions
		WHERE
			userId = ?
	`, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to get user permission: %s", err)
	}
	return permissionIterWrapper{rows}, nil
}

type permissionIterWrapper struct {
	rows *sql.Rows
}

func (it permissionIterWrapper) Get() (permissions.Permission, error) {
	var permission permissions.Permission
	err := it.rows.Scan(&permission)
	if err != nil {
		return 0, fmt.Errorf("failed to get permission from iter: %s", err)
	}
	return permission, nil
}
func (it permissionIterWrapper) Next() bool {
	return it.rows.Next()
}
func (it permissionIterWrapper) Err() error {
	return it.rows.Err()
}

func (db webDb) GetUsersWithPermission(ctx context.Context, assetId string, p permissions.Permission) (iterator.I[int64], error) {
	rows, err := db.s.Query(ctx, `
		SELECT
			userId
		FROM permissions
		WHERE
			permission = ? AND assetId = ?
	`, p, assetId)
	if err != nil {
		return nil, fmt.Errorf("failed to get users with permission: %s", err)
	}
	return usersWithPermissionIter{rows}, nil
}

func (db webDb) GetUserAssetIdsWithPermission(ctx context.Context,
	userId int64, p ...permissions.Permission) (iterator.I[string], error) {
	if len(p) > 20 {
		// This might make the query expensive.
		// We can definitely adjust it if needed.
		return nil, fmt.Errorf("too many permissions")
	}
	if len(p) == 0 {
		panic("called GetUserAssetIdsWithPermission with zero permissions")
	}

	// The query itself is variable and depends on the number of arguments.
	// We must construct a list of placeholders like `?, ?, ...`
	// of the same size of p.
	// The args for the query are then the userId followed by the permissions
	queryPlaceholdersList := make([]string, len(p))
	queryArgs := make([]any, len(p)+1)
	queryArgs[0] = userId
	for i, perm := range p {
		queryPlaceholdersList[i] = "?"
		queryArgs[i+1] = perm
	}
	queryPlaceholders := strings.Join(queryPlaceholdersList, ", ")
	rows, err := db.s.Query(ctx, fmt.Sprintf(`
		SELECT
			assetId
		FROM permissions
		WHERE
			userId = ? AND permission IN (%s)
		GROUP BY assetId
		`, queryPlaceholders,
	), queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get users with permission: %s", err)
	}
	return assetIdIter{rows}, nil
}

func (db webDb) CountUserAssetsWithPermission(
	ctx context.Context,
	userId int64,
	permission permissions.Permission,
) (int64, error) {
	var count int64
	err := db.s.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM permissions
		WHERE
			userId = ? AND permission = ?
	`,
		userId,
		permission,
	).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("failed to count assets with permission: %s", err)
	}
	return count, nil
}

type usersWithPermissionIter struct {
	rows *sql.Rows
}

func (it usersWithPermissionIter) Get() (int64, error) {
	var userId int64
	err := it.rows.Scan(&userId)
	if err != nil {
		return 0, fmt.Errorf("failed to get userId from iter: %s", err)
	}
	return userId, err
}
func (it usersWithPermissionIter) Next() bool {
	return it.rows.Next()
}
func (it usersWithPermissionIter) Err() error {
	return it.rows.Err()
}

type assetIdIter struct {
	rows *sql.Rows
}

func (it assetIdIter) Get() (string, error) {
	var assetId string
	err := it.rows.Scan(&assetId)
	if err != nil {
		return "", fmt.Errorf("failed to get assetId from iter: %s", err)
	}
	return assetId, err
}
func (it assetIdIter) Next() bool {
	return it.rows.Next()
}
func (it assetIdIter) Err() error {
	return it.rows.Err()
}

func (db webDb) CountUsersWithPermission(ctx context.Context, assetId string, p permissions.Permission) (int64, error) {
	var count int64
	err := db.s.QueryRow(ctx, `
		SELECT COUNT(*) FROM permissions WHERE assetId = ? AND permission = ?
	`, assetId, p).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users with permission: %s", err)
	}
	return count, nil
}
