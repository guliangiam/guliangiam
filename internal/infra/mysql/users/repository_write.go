package users

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (r Repository) Create(ctx context.Context, user *domain.User) error {
	return r.withinTransaction(ctx, func(ctx context.Context, tx mysqlshared.DBTX) error {
		recoveryCodes, err := mysqlshared.MarshalJSON(user.RecoveryCodeHashes)
		if err != nil {
			return err
		}
		addresses, err := mysqlshared.MarshalJSON(user.Addresses)
		if err != nil {
			return err
		}
		managedAccounts, err := mysqlshared.MarshalJSON(user.ManagedAccounts)
		if err != nil {
			return err
		}
		faceIds, err := mysqlshared.MarshalJSON(user.FaceIds)
		if err != nil {
			return err
		}
		cart, err := mysqlshared.MarshalJSON(user.Cart)
		if err != nil {
			return err
		}
		mfaItems, err := mysqlshared.MarshalJSON(user.MfaItems)
		if err != nil {
			return err
		}
		mfaAccounts, err := mysqlshared.MarshalJSON(user.MfaAccounts)
		if err != nil {
			return err
		}
		applicationScopes, err := mysqlshared.MarshalJSON(user.ApplicationScopes)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, queryInsertUserCore,
			user.ID, user.OrganizationID, user.Username, nullableString(user.ExternalID),
			user.UserType, user.Status, user.IsAdmin, user.IsForbidden, user.IsVerified, user.IsOnline,
			nullableString(user.CreatedIp), nullableString(user.SignupApplicationID),
			nullableString(user.RegisterType), nullableString(user.RegisterSource),
			user.Tag, user.Score,
			user.IsDeleted, user.IpWhitelist,
			user.CreatedAt, user.UpdatedAt, nullTimeArg(user.DeletedAt), user.Revision,
		); err != nil {
			return err
		}

		if err := r.upsertUserSubtables(ctx, tx, user, recoveryCodes, addresses, managedAccounts,
			faceIds, cart, mfaItems, mfaAccounts, applicationScopes); err != nil {
			return err
		}

		if err := r.createProfileTx(ctx, tx, &user.Profile); err != nil {
			return err
		}
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
	recoveryCodes, addresses, managedAccounts, faceIds, cart, mfaItems, mfaAccounts, applicationScopes, err := marshalUserAggregateJSON(user)
	if err != nil {
		return err
	}

	args := userUpdateCoreArgs(user)
	args = append(args, user.ID, user.Revision-1)

	// 乐观锁守卫与子表写入必须同一事务：主表 revision 不匹配时子表不得被改写。
	// 领域层在调用 Update 前已执行 Revision++，因此 WHERE 使用 Revision-1，
	// 与 PostgreSQL 的 REVISION-1 + RETURNING revision 语义一致。
	return r.withinTransaction(ctx, func(ctx context.Context, tx mysqlshared.DBTX) error {
		if scoped {
			affected, err := rowsAffected(tx.ExecContext(ctx, updateQuery, append(args, organizationID)...))
			if err != nil {
				return err
			}
			if affected == 0 {
				// MySQL has no RLS: the UPDATE already carried the explicit
				// organization predicate. Probe with the same predicate to tell a
				// same-tenant stale revision (409) from a foreign/missing id (404).
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
			affected, err := rowsAffected(tx.ExecContext(ctx, updateQuery, args...))
			if err != nil {
				return err
			}
			if affected == 0 {
				return apperrors.NewConflict("user revision conflict")
			}
		}

		if err := r.upsertUserSubtables(ctx, tx, user, recoveryCodes, addresses, managedAccounts,
			faceIds, cart, mfaItems, mfaAccounts, applicationScopes); err != nil {
			return err
		}
		return nil
	})
}

// userExistsForOrganization is the tenant-scoped existence probe used only
// after a scoped UPDATE affected zero rows. The explicit organization_id
// predicate keeps MySQL (which has no RLS) from leaking a foreign row.
func userExistsForOrganization(ctx context.Context, tx mysqlshared.DBTX, id, organizationID string) (bool, error) {
	var one int
	err := tx.QueryRowContext(ctx, queryUserExistsForOrganization, id, organizationID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func marshalUserAggregateJSON(user *domain.User) (recoveryCodes, addresses, managedAccounts, faceIds, cart, mfaItems, mfaAccounts, applicationScopes []byte, err error) {
	if recoveryCodes, err = mysqlshared.MarshalJSON(user.RecoveryCodeHashes); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if addresses, err = mysqlshared.MarshalJSON(user.Addresses); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if managedAccounts, err = mysqlshared.MarshalJSON(user.ManagedAccounts); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if faceIds, err = mysqlshared.MarshalJSON(user.FaceIds); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if cart, err = mysqlshared.MarshalJSON(user.Cart); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if mfaItems, err = mysqlshared.MarshalJSON(user.MfaItems); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if mfaAccounts, err = mysqlshared.MarshalJSON(user.MfaAccounts); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if applicationScopes, err = mysqlshared.MarshalJSON(user.ApplicationScopes); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	return recoveryCodes, addresses, managedAccounts, faceIds, cart, mfaItems, mfaAccounts, applicationScopes, nil
}

func userUpdateCoreArgs(user *domain.User) []any {
	return []any{
		nullableString(user.ExternalID),
		user.UserType, user.Status, user.IsAdmin, user.IsForbidden, user.IsVerified, user.IsOnline,
		nullableString(user.CreatedIp), nullableString(user.SignupApplicationID),
		nullableString(user.RegisterType), nullableString(user.RegisterSource),
		user.Tag, user.Score,
		user.IsDeleted, user.IpWhitelist,
		user.UpdatedAt, nullTimeArg(user.DeletedAt),
	}
}

// upsertUserSubtables 写入 ADR-022 God Table 拆分出的 5 张子表。每条语句都是
// 以 user_id 为键的 upsert，使创建与更新共用一条路径；事务由调用方持有，保证聚合原子。
func (r Repository) upsertUserSubtables(ctx context.Context, tx mysqlshared.DBTX, user *domain.User,
	recoveryCodes, addresses, managedAccounts, faceIds, cart, mfaItems, mfaAccounts, applicationScopes []byte,
) error {
	if _, err := tx.ExecContext(ctx, queryUpsertUserSigninTracking,
		user.ID, user.SigninWrongTimes, nullTimeArg(user.LastSigninWrongTime),
		nullTimeArg(user.LastSigninAt), nullableString(user.LastSigninIP)); err != nil {
		return err
	}
	totpSecret, err := sealTotpSecret(user.ID, user.TotpSecret)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, queryUpsertUserMfaConfig,
		user.ID, nullableString(user.PreferredMfaType), recoveryCodes, nullableString(totpSecret),
		nullableString(user.MfaRadiusUsername), nullableString(user.MfaRadiusProvider),
		user.MfaPushReceiver, user.MfaPushProvider, user.MfaPushEnabled,
		nullTimeArg(user.MfaRememberDeadline)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, queryUpsertUserFinancials,
		user.ID, user.Balance, user.BalanceCredit, user.BalanceCurrency); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, queryUpsertUserCollections,
		user.ID, addresses, managedAccounts, faceIds, cart, mfaItems, mfaAccounts, applicationScopes); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, queryUpsertUserCustomAttributes,
		user.ID, user.Custom1, user.Custom2, user.Custom3, user.Custom4, user.Custom5,
		user.Custom6, user.Custom7, user.Custom8, user.Custom9, user.Custom10)
	return err
}

func (r Repository) UpdateProfile(ctx context.Context, profile *domain.UserProfile) error {
	return r.updateProfileCtx(ctx, r.db, profile)
}

func (r Repository) createProfileTx(ctx context.Context, tx mysqlshared.DBTX, profile *domain.UserProfile) error {
	return r.updateProfileCtx(ctx, tx, profile)
}

func (r Repository) updateProfileCtx(ctx context.Context, tx mysqlshared.DBTX, profile *domain.UserProfile) error {
	properties, err := mysqlshared.MarshalJSON(profile.Properties)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, queryUpsertUserProfile,
		profile.UserID, nullableString(profile.DisplayName), nullableString(profile.FirstName),
		nullableString(profile.LastName), nullableString(profile.AvatarURL),
		nullableString(profile.Email), profile.EmailVerified,
		nullableString(profile.Phone), profile.PhoneVerified,
		nullableString(profile.CountryCode), nullableString(profile.Region), nullableString(profile.Location),
		nullableString(profile.Birthday), nullableString(profile.Gender), nullableString(profile.Bio),
		nullableString(profile.RealName), nullableString(profile.IDCardType), nullableString(profile.IDCard),
		nullableString(profile.PassportNo), nullableString(profile.DriverLicenseNo), nullableString(profile.IDCardExpiry),
		nullableString(profile.IssueCountry), nullableString(profile.IssuingAuthority), nullableString(profile.PassportPhotoURL),
		nullableString(profile.DriverLicenseFrontURL), nullableString(profile.DriverLicenseBackURL),
		properties,
	)
	return err
}

func (r Repository) UpdateCredentials(ctx context.Context, credentials *domain.UserCredentials) error {
	return r.updateCredentialsCtx(ctx, r.db, credentials)
}

func (r Repository) createCredentialsTx(ctx context.Context, tx mysqlshared.DBTX, credentials *domain.UserCredentials) error {
	return r.updateCredentialsCtx(ctx, tx, credentials)
}

func (r Repository) updateCredentialsCtx(ctx context.Context, tx mysqlshared.DBTX, credentials *domain.UserCredentials) error {
	_, err := tx.ExecContext(ctx, queryUpsertUserCredentials,
		credentials.UserID, credentials.PasswordHash, credentials.PasswordAlgo,
		nullTimeArg(credentials.PasswordUpdatedAt),
		credentials.AccessKey, credentials.AccessSecretHash,
		nullTimeArg(credentials.AccessSecretRotatedAt),
		credentials.NeedUpdatePassword,
		credentials.AccessSecretHashPrev,
		nullTimeArg(credentials.AccessSecretPrevExpiresAt),
	)
	return err
}

// UpdatePasswordCredential 只写口令列，保留 access_key/access_secret_hash。
// upsert 语义与 PG 镜像一致：无凭据行用户首次重置口令必须落库。
func (r Repository) UpdatePasswordCredential(ctx context.Context, credentials *domain.UserCredentials) error {
	_, err := r.db.ExecContext(ctx, queryUpdatePasswordCredential,
		credentials.PasswordHash, credentials.PasswordAlgo,
		nullTimeArg(credentials.PasswordUpdatedAt),
		credentials.NeedUpdatePassword, credentials.UserID,
	)
	return err
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	affected, err := rowsAffected(r.db.ExecContext(ctx, queryDeleteUser, id, organizationID))
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("user not found")
	}
	return nil
}

func (r Repository) SetContactVerifiedForOrganization(ctx context.Context, userID, organizationID string, emailVerified, phoneVerified bool) error {
	result, err := r.db.ExecContext(ctx, queryUpdateUserContactVerified, userID, organizationID, emailVerified, phoneVerified)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("user profile not found")
	}
	return nil
}

func (r Repository) UpdateCollectionsForOrganization(ctx context.Context, userID, organizationID string, collections domain.UserCollections) error {
	return r.withinTransaction(ctx, func(ctx context.Context, tx mysqlshared.DBTX) error {
		var exists int
		if err := tx.QueryRowContext(ctx, queryUserExistsForOrganization, userID, organizationID).Scan(&exists); err != nil {
			if err == sql.ErrNoRows {
				return apperrors.NewNotFound("user not found")
			}
			return err
		}
		addresses, err := mysqlshared.MarshalJSON(collections.Addresses)
		if err != nil {
			return err
		}
		managedAccounts, err := mysqlshared.MarshalJSON(collections.ManagedAccounts)
		if err != nil {
			return err
		}
		faceIds, err := mysqlshared.MarshalJSON(collections.FaceIds)
		if err != nil {
			return err
		}
		cart, err := mysqlshared.MarshalJSON(collections.Cart)
		if err != nil {
			return err
		}
		mfaItems, err := mysqlshared.MarshalJSON(collections.MfaItems)
		if err != nil {
			return err
		}
		mfaAccounts, err := mysqlshared.MarshalJSON(collections.MfaAccounts)
		if err != nil {
			return err
		}
		applicationScopes, err := mysqlshared.MarshalJSON(collections.ApplicationScopes)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, queryUpsertUserCollections, userID, addresses, managedAccounts, faceIds, cart, mfaItems, mfaAccounts, applicationScopes)
		return err
	})
}

func (r Repository) ReplaceIdentities(ctx context.Context, userID string, identities []domain.UserIdentity) error {
	return r.withinTransaction(ctx, func(ctx context.Context, tx mysqlshared.DBTX) error {
		if _, err := tx.ExecContext(ctx, queryDeleteUserIdentities, userID); err != nil {
			return err
		}
		for i := range identities {
			identity := &identities[i]
			profileSnapshot, err := mysqlshared.MarshalJSON(identity.ProfileSnapshot)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, queryInsertUserIdentity,
				identity.ID, userID, identity.ProviderID, identity.IdentityType,
				identity.Subject, identity.ProviderAccount, profileSnapshot, identity.LinkedAt,
			); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r Repository) CreateIdentity(ctx context.Context, identity *domain.UserIdentity) error {
	profileSnapshot, err := mysqlshared.MarshalJSON(identity.ProfileSnapshot)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertUserIdentity,
		identity.ID, identity.UserID, identity.ProviderID, identity.IdentityType,
		identity.Subject, identity.ProviderAccount, profileSnapshot, identity.LinkedAt,
	)
	return err
}

func (r Repository) DeleteIdentity(ctx context.Context, userID, identityID string) error {
	_, err := r.db.ExecContext(ctx, queryDeleteSingleIdentity, userID, identityID)
	return err
}

func (r Repository) ReplaceMfaMethods(ctx context.Context, userID string, methods []domain.UserMfaMethod) error {
	return r.withinTransaction(ctx, func(ctx context.Context, tx mysqlshared.DBTX) error {
		if _, err := tx.ExecContext(ctx, queryDeleteUserMfaMethods, userID); err != nil {
			return err
		}
		for _, method := range methods {
			if _, err := tx.ExecContext(ctx, queryInsertUserMfaMethod,
				method.ID, userID, method.MethodType, method.Enabled, method.Verified,
				method.DisplayName, nullTimeArg(method.LastUsedAt),
			); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r Repository) ReplaceRoleBindings(ctx context.Context, userID string, bindings []domain.UserRoleBinding) error {
	return r.withinTransaction(ctx, func(ctx context.Context, tx mysqlshared.DBTX) error {
		if _, err := tx.ExecContext(ctx, queryDeleteUserRoleBindings, userID); err != nil {
			return err
		}
		for _, binding := range bindings {
			if _, err := tx.ExecContext(ctx, queryInsertUserRoleBinding, userID, binding.RoleID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r Repository) ReplaceGroupMemberships(ctx context.Context, userID string, memberships []domain.UserGroupMembership) error {
	return r.withinTransaction(ctx, func(ctx context.Context, tx mysqlshared.DBTX) error {
		if _, err := tx.ExecContext(ctx, queryDeleteUserGroupMemberships, userID); err != nil {
			return err
		}
		for _, membership := range memberships {
			if _, err := tx.ExecContext(ctx, queryInsertUserGroupMembership, userID, membership.GroupID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r Repository) CreatePasswordHistory(ctx context.Context, entry *domain.PasswordHistoryEntry) error {
	_, err := r.db.ExecContext(ctx, queryInsertPasswordHistory,
		entry.ID, entry.UserID, entry.PasswordHash, entry.PasswordAlgo, entry.CreatedAt,
	)
	return err
}

func (r Repository) DeletePasswordHistoryBeyond(ctx context.Context, userID string, keepCount int) error {
	_, err := r.db.ExecContext(ctx, queryDeletePasswordHistoryBeyond, userID, userID, keepCount)
	return err
}

// ── List helpers for rebac ──────────────────────────────────────────────
