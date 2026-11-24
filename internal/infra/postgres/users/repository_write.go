package users

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

func (r Repository) Create(ctx context.Context, user *domain.User) error {
	// User + profile + credentials must be created atomically to ensure aggregate consistency.
	return r.withinTransaction(ctx, func(ctx context.Context, tx shared.DBTX) error {
		recoveryCodeHashes := user.RecoveryCodeHashes

		addresses, err := shared.MarshalJSON(user.Addresses)
		if err != nil {
			return err
		}

		managedAccounts, err := shared.MarshalJSON(user.ManagedAccounts)
		if err != nil {
			return err
		}

		faceIds, err := shared.MarshalJSON(user.FaceIds)
		if err != nil {
			return err
		}

		cart, err := shared.MarshalJSON(user.Cart)
		if err != nil {
			return err
		}

		mfaItems, err := shared.MarshalJSON(user.MfaItems)
		if err != nil {
			return err
		}

		mfaAccounts, err := shared.MarshalJSON(user.MfaAccounts)
		if err != nil {
			return err
		}

		applicationScopes, err := shared.MarshalJSON(user.ApplicationScopes)
		if err != nil {
			return err
		}

		if _, err = tx.Exec(ctx, queryInsertUserCore,
			user.ID, user.OrganizationID, user.Username, nullableString(user.ExternalID), user.UserType, user.Status, user.IsAdmin, user.IsForbidden, user.IsVerified, user.IsOnline, user.CreatedIp, nullableString(user.SignupApplicationID), user.RegisterType, nullableString(user.RegisterSource), user.Tag, user.Score, user.IsDeleted, user.IpWhitelist,
			user.CreatedAt, user.UpdatedAt, user.DeletedAt, user.Revision); err != nil {
			return err
		}

		if err := r.upsertUserSubtables(ctx, tx, user, recoveryCodeHashes, addresses, managedAccounts, faceIds, cart, mfaItems, mfaAccounts, applicationScopes); err != nil {
			return err
		}

		// Create profile within the same transaction
		if err := r.createProfileTx(ctx, tx, &user.Profile); err != nil {
			return err
		}

		// Create credentials within the same transaction
		if err := r.createCredentialsTx(ctx, tx, &user.Credentials); err != nil {
			return err
		}

		return nil
	})
}

func (r Repository) Update(ctx context.Context, user *domain.User) error {
	return r.updateCore(ctx, user, queryUpdateUserCore, false, "")
}

// UpdateForOrganization persists the aggregate only when the row still belongs
// to organizationID. Zero affected rows means the user is foreign or gone and is
// reported as not found, never as a revision conflict.
func (r Repository) UpdateForOrganization(ctx context.Context, user *domain.User, organizationID string) error {
	return r.updateCore(ctx, user, queryUpdateUserCoreForOrganization, true, organizationID)
}

func (r Repository) updateCore(ctx context.Context, user *domain.User, updateQuery string, scoped bool, organizationID string) error {
	recoveryCodeHashes, addresses, managedAccounts, faceIds, cart, mfaItems, mfaAccounts, applicationScopes, err := marshalUserAggregateJSON(user)
	if err != nil {
		return err
	}

	args := userUpdateCoreArgs(user)

	// 乐观锁守卫与子表写入必须同一事务：主表 revision 不匹配时子表不得被改写。
	return r.withinTransaction(ctx, func(ctx context.Context, tx shared.DBTX) error {
		args = append(args, user.Revision-1, user.ID)
		if scoped {
			args = append(args, organizationID)
			tag, err := tx.Exec(ctx, updateQuery, args...)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				// 0 rows is ambiguous: cross-tenant, missing, or a stale revision.
				// Probe inside the same tenant-scoped transaction so a foreign id
				// still collapses to NotFound while a same-tenant row reports the
				// real optimistic-lock Conflict. The probe cannot widen visibility:
				// it carries the same organization predicate as the UPDATE.
				exists, err := userExistsForOrganization(ctx, tx, user.ID, organizationID)
				if err != nil {
					return err
				}
				if exists {
					return apperrors.NewConflict("user revision conflict")
				}
				return apperrors.NewNotFound("user not found")
			}
		} else {
			var newRevision int64
			if err := tx.QueryRow(ctx, updateQuery, args...).Scan(&newRevision); err != nil {
				if err == pgx.ErrNoRows {
					return apperrors.NewConflict("user revision conflict")
				}
				return err
			}
			user.Revision = newRevision
		}

		if err := r.upsertUserSubtables(ctx, tx, user, recoveryCodeHashes, addresses, managedAccounts, faceIds, cart, mfaItems, mfaAccounts, applicationScopes); err != nil {
			return err
		}
		return nil
	})
}

// userExistsForOrganization is the tenant-scoped existence probe used only
// after a scoped UPDATE affected zero rows. It distinguishes a stale revision
// (row present in this tenant → Conflict) from a foreign/missing id (→ NotFound)
// without ever revealing rows owned by another organization.
func userExistsForOrganization(ctx context.Context, tx shared.DBTX, id, organizationID string) (bool, error) {
	var one int
	err := tx.QueryRow(ctx, queryUserExistsForOrganization, id, organizationID).Scan(&one)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func marshalUserAggregateJSON(user *domain.User) (recoveryCodeHashes []string, addresses, managedAccounts, faceIds, cart, mfaItems, mfaAccounts, applicationScopes []byte, err error) {
	recoveryCodeHashes = user.RecoveryCodeHashes
	if addresses, err = shared.MarshalJSON(user.Addresses); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if managedAccounts, err = shared.MarshalJSON(user.ManagedAccounts); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if faceIds, err = shared.MarshalJSON(user.FaceIds); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if cart, err = shared.MarshalJSON(user.Cart); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if mfaItems, err = shared.MarshalJSON(user.MfaItems); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if mfaAccounts, err = shared.MarshalJSON(user.MfaAccounts); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if applicationScopes, err = shared.MarshalJSON(user.ApplicationScopes); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	return recoveryCodeHashes, addresses, managedAccounts, faceIds, cart, mfaItems, mfaAccounts, applicationScopes, nil
}

func marshalUserCollectionsJSON(collections domain.UserCollections) (addresses, managedAccounts, faceIds, cart, mfaItems, mfaAccounts, applicationScopes []byte, err error) {
	if addresses, err = shared.MarshalJSON(collections.Addresses); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	if managedAccounts, err = shared.MarshalJSON(collections.ManagedAccounts); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	if faceIds, err = shared.MarshalJSON(collections.FaceIds); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	if cart, err = shared.MarshalJSON(collections.Cart); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	if mfaItems, err = shared.MarshalJSON(collections.MfaItems); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	if mfaAccounts, err = shared.MarshalJSON(collections.MfaAccounts); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	if applicationScopes, err = shared.MarshalJSON(collections.ApplicationScopes); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	return addresses, managedAccounts, faceIds, cart, mfaItems, mfaAccounts, applicationScopes, nil
}

// SetContactVerifiedForOrganization flips contact verification flags without
// touching any other profile field. The tenant predicate lives in SQL so a
// cross-tenant id is indistinguishable from a missing profile.
func (r Repository) SetContactVerifiedForOrganization(ctx context.Context, userID, organizationID string, emailVerified, phoneVerified bool) error {
	tag, err := r.db.Exec(ctx, queryUpdateUserContactVerified, userID, organizationID, emailVerified, phoneVerified)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("user profile not found")
	}
	return nil
}

// UpdateCollectionsForOrganization 只替换 JSONB 集合子表；用户行与租户归属不变，
// 因此不使用 revision 乐观锁（集合写入不携带可比较的业务版本）。租户谓词在 SQL
// 内完成：跨租户或已删除用户返回 0 行，统一折叠为 NotFound。
func (r Repository) UpdateCollectionsForOrganization(ctx context.Context, userID, organizationID string, collections domain.UserCollections) error {
	addresses, managedAccounts, faceIds, cart, mfaItems, mfaAccounts, applicationScopes, err := marshalUserCollectionsJSON(collections)
	if err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx, queryUpsertUserCollectionsForOrganization, userID, addresses, managedAccounts, faceIds, cart, mfaItems, mfaAccounts, applicationScopes, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("user not found")
	}
	return nil
}

func userUpdateCoreArgs(user *domain.User) []any {
	return []any{
		nullableString(user.ExternalID), user.UserType, user.Status, user.IsAdmin, user.IsForbidden, user.IsVerified, user.IsOnline, user.CreatedIp, nullableString(user.SignupApplicationID), user.RegisterType, nullableString(user.RegisterSource), user.Tag, user.Score,
		user.IsDeleted, user.IpWhitelist, user.UpdatedAt, user.DeletedAt,
	}
}

// upsertUserSubtables 写入 ADR-022 God Table 拆分出的 5 张子表。每条语句都是
// 以 user_id 为键的 upsert，使创建与更新共用一条路径；事务由调用方持有，保证聚合原子。
func (r Repository) upsertUserSubtables(ctx context.Context, tx shared.DBTX, user *domain.User, recoveryCodeHashes []string, addresses, managedAccounts, faceIds, cart, mfaItems, mfaAccounts, applicationScopes []byte) error {
	// recovery_code_hashes 是 text[] NOT NULL（默认 '{}'）。新建用户没有恢复码时
	// user.RecoveryCodeHashes 是 nil slice，pgx 会编码成 SQL NULL，显式 NULL 覆盖
	// 列默认值并触发 23502（not-null violation），创建用户整体回滚。空 slice 编码
	// 为 '{}'，与列语义一致。
	if recoveryCodeHashes == nil {
		recoveryCodeHashes = []string{}
	}
	if _, err := tx.Exec(ctx, queryUpsertUserSigninTracking, user.ID, user.SigninWrongTimes, user.LastSigninWrongTime, user.LastSigninAt, nullableString(user.LastSigninIP)); err != nil {
		return err
	}
	totpSecret, err := sealTotpSecret(user.ID, user.TotpSecret)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, queryUpsertUserMfaConfig, user.ID, nullableString(user.PreferredMfaType), recoveryCodeHashes, nullableString(totpSecret), user.MfaRadiusUsername, user.MfaRadiusProvider, user.MfaPushReceiver, user.MfaPushProvider, user.MfaPushEnabled, user.MfaRememberDeadline); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, queryUpsertUserFinancials, user.ID, user.Balance, user.BalanceCredit, user.BalanceCurrency); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, queryUpsertUserCollections, user.ID, addresses, managedAccounts, faceIds, cart, mfaItems, mfaAccounts, applicationScopes); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, queryUpsertUserCustomAttributes, user.ID, user.Custom1, user.Custom2, user.Custom3, user.Custom4, user.Custom5, user.Custom6, user.Custom7, user.Custom8, user.Custom9, user.Custom10); err != nil {
		return err
	}
	return nil
}

func (r Repository) UpdateProfile(ctx context.Context, profile *domain.UserProfile) error {
	properties, err := shared.MarshalJSON(profile.Properties)
	if err != nil {
		return err
	}
	idCardSealed, passportNoSealed, driverLicenseNoSealed, err := sealProfileDocuments(profile.UserID, profile)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, queryUpsertUserProfile, profile.UserID, profile.DisplayName, profile.FirstName, profile.LastName, profile.AvatarURL, nullableString(profile.Email), profile.EmailVerified, nullableString(profile.Phone), profile.PhoneVerified, profile.CountryCode, profile.Region, profile.Location, nullableString(profile.Birthday), profile.Gender, profile.Bio, profile.RealName, profile.IDCardType, profile.IDCard, nullableString(profile.PassportNo), nullableString(profile.DriverLicenseNo), nullableString(profile.IDCardExpiry), nullableString(profile.IssueCountry), nullableString(profile.IssuingAuthority), nullableString(profile.PassportPhotoURL), nullableString(profile.DriverLicenseFrontURL), nullableString(profile.DriverLicenseBackURL), properties, idCardSealed, passportNoSealed, driverLicenseNoSealed)
	return err
}

// createProfileTx creates a user profile within an existing transaction.
func (r Repository) createProfileTx(ctx context.Context, tx shared.DBTX, profile *domain.UserProfile) error {
	properties, err := shared.MarshalJSON(profile.Properties)
	if err != nil {
		return err
	}
	idCardSealed, passportNoSealed, driverLicenseNoSealed, err := sealProfileDocuments(profile.UserID, profile)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, queryUpsertUserProfile, profile.UserID, profile.DisplayName, profile.FirstName, profile.LastName, profile.AvatarURL, nullableString(profile.Email), profile.EmailVerified, nullableString(profile.Phone), profile.PhoneVerified, profile.CountryCode, profile.Region, profile.Location, nullableString(profile.Birthday), profile.Gender, profile.Bio, profile.RealName, profile.IDCardType, profile.IDCard, nullableString(profile.PassportNo), nullableString(profile.DriverLicenseNo), nullableString(profile.IDCardExpiry), nullableString(profile.IssueCountry), nullableString(profile.IssuingAuthority), nullableString(profile.PassportPhotoURL), nullableString(profile.DriverLicenseFrontURL), nullableString(profile.DriverLicenseBackURL), properties, idCardSealed, passportNoSealed, driverLicenseNoSealed)
	return err
}

func (r Repository) UpdateCredentials(ctx context.Context, credentials *domain.UserCredentials) error {
	_, err := r.db.Exec(ctx, queryUpsertUserCredentials, credentials.UserID, credentials.PasswordHash, credentials.PasswordAlgo, credentials.PasswordUpdatedAt, credentials.AccessKey, credentials.AccessSecretHash, credentials.AccessSecretRotatedAt, credentials.NeedUpdatePassword, credentials.AccessSecretHashPrev, credentials.AccessSecretPrevExpiresAt)
	return err
}

// UpdatePasswordCredential 只写口令列，保留 access_key/access_secret_hash。
// upsert：无凭据行的用户（外部身份/无口令创建）首次重置口令必须落库，
// 纯 UPDATE 会 0 行静默成功（运行期缺陷：200 但口令未持久化）。
func (r Repository) UpdatePasswordCredential(ctx context.Context, credentials *domain.UserCredentials) error {
	_, err := r.db.Exec(ctx, queryUpdatePasswordCredential, credentials.UserID, credentials.PasswordHash, credentials.PasswordAlgo, credentials.PasswordUpdatedAt, credentials.NeedUpdatePassword)
	return err
}

// createCredentialsTx creates user credentials within an existing transaction.
func (r Repository) createCredentialsTx(ctx context.Context, tx shared.DBTX, credentials *domain.UserCredentials) error {
	// NewUser 不会初始化 credentials，只有 CreateUser 带了密码才会在事务内创建。
	// 没有密码时跳过 — 零值的 UserID 会把空字符串写进 uuid NOT NULL 列。
	if credentials == nil || credentials.UserID == "" {
		return nil
	}
	_, err := tx.Exec(ctx, queryUpsertUserCredentials, credentials.UserID, credentials.PasswordHash, credentials.PasswordAlgo, credentials.PasswordUpdatedAt, credentials.AccessKey, credentials.AccessSecretHash, credentials.AccessSecretRotatedAt, credentials.NeedUpdatePassword, credentials.AccessSecretHashPrev, credentials.AccessSecretPrevExpiresAt)
	return err
}

func (r Repository) ReplaceIdentities(ctx context.Context, userID string, identities []domain.UserIdentity) error {
	// DELETE + INSERT must be atomic to prevent data loss under concurrent access.
	return r.withinTransaction(ctx, func(ctx context.Context, tx shared.DBTX) error {
		if _, err := tx.Exec(ctx, queryDeleteUserIdentities, userID); err != nil {
			return err
		}

		for i := range identities {
			identity := &identities[i]
			profileSnapshot, err := shared.MarshalJSON(identity.ProfileSnapshot)
			if err != nil {
				return err
			}

			if _, err := tx.Exec(ctx, queryInsertUserIdentity, identity.ID, userID, identity.ProviderID, identity.IdentityType, identity.Subject, identity.ProviderAccount, profileSnapshot, identity.LinkedAt); err != nil {
				return err
			}
		}

		return nil
	})
}

func (r Repository) CreateIdentity(ctx context.Context, identity *domain.UserIdentity) error {
	profileSnapshot, err := shared.MarshalJSON(identity.ProfileSnapshot)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, queryInsertUserIdentity, identity.ID, identity.UserID, identity.ProviderID, identity.IdentityType, identity.Subject, identity.ProviderAccount, profileSnapshot, identity.LinkedAt)
	return err
}

func (r Repository) DeleteIdentity(ctx context.Context, userID, identityID string) error {
	_, err := r.db.Exec(ctx, queryDeleteSingleIdentity, userID, identityID)
	return err
}

func (r Repository) ReplaceMfaMethods(ctx context.Context, userID string, methods []domain.UserMfaMethod) error {
	// DELETE + INSERT must be atomic to prevent data loss under concurrent access.
	return r.withinTransaction(ctx, func(ctx context.Context, tx shared.DBTX) error {
		if _, err := tx.Exec(ctx, queryDeleteUserMfaMethods, userID); err != nil {
			return err
		}

		for _, method := range methods {
			if _, err := tx.Exec(ctx, queryInsertUserMfaMethod, method.ID, userID, method.MethodType, method.Enabled, method.Verified, method.DisplayName, method.LastUsedAt); err != nil {
				return err
			}
		}

		return nil
	})
}

func (r Repository) ReplaceRoleBindings(ctx context.Context, userID string, bindings []domain.UserRoleBinding) error {
	// DELETE + INSERT must be atomic to prevent data loss under concurrent access.
	return r.withinTransaction(ctx, func(ctx context.Context, tx shared.DBTX) error {
		if _, err := tx.Exec(ctx, queryDeleteUserRoleBindings, userID); err != nil {
			return err
		}

		for _, binding := range bindings {
			if _, err := tx.Exec(ctx, queryInsertUserRoleBinding, userID, binding.RoleID); err != nil {
				return err
			}
		}

		return nil
	})
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, queryDeleteUser, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("user not found")
	}
	return nil
}

func (r Repository) ReplaceGroupMemberships(ctx context.Context, userID string, memberships []domain.UserGroupMembership) error {
	// DELETE + INSERT must be atomic to prevent data loss under concurrent access.
	return r.withinTransaction(ctx, func(ctx context.Context, tx shared.DBTX) error {
		if _, err := tx.Exec(ctx, queryDeleteUserGroupMemberships, userID); err != nil {
			return err
		}

		for _, membership := range memberships {
			if _, err := tx.Exec(ctx, queryInsertUserGroupMembership, userID, membership.GroupID); err != nil {
				return err
			}
		}

		return nil
	})
}

func (r Repository) CreatePasswordHistory(ctx context.Context, entry *domain.PasswordHistoryEntry) error {
	_, err := r.db.Exec(ctx, queryInsertPasswordHistory, entry.ID, entry.UserID, entry.PasswordHash, entry.PasswordAlgo, entry.CreatedAt)
	return err
}

func (r Repository) DeletePasswordHistoryBeyond(ctx context.Context, userID string, keepCount int) error {
	_, err := r.db.Exec(ctx, queryDeletePasswordHistoryBeyond, userID, keepCount)
	return err
}
