package users

import (
	"context"
	"testing"
	"time"

	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Tests for Service.LinkIdentity
// ---------------------------------------------------------------------------

func TestService_LinkIdentity_MissingUserID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	err := svc.LinkIdentity(ctx(), LinkIdentityCommand{
		Provider:   "google",
		ExternalID: "ext-123",
	})
	assert.Error(t, err)
}

func TestService_LinkIdentity_MissingProvider(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	err := svc.LinkIdentity(ctx(), LinkIdentityCommand{
		UserID:     "user-1",
		ExternalID: "ext-123",
	})
	assert.Error(t, err)
}

func TestService_LinkIdentity_MissingExternalID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	err := svc.LinkIdentity(ctx(), LinkIdentityCommand{
		UserID:   "user-1",
		Provider: "google",
	})
	assert.Error(t, err)
}

func TestService_LinkIdentity_UserNotFound(t *testing.T) {
	svc := Service{
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return nil, apperrors.NewNotFound("user not found")
				},
			},
		},
	}
	err := svc.LinkIdentity(ctx(), LinkIdentityCommand{
		UserID:     "nonexistent",
		Provider:   "google",
		ExternalID: "ext-123",
	})
	assert.Error(t, err)
}

func TestService_LinkIdentity_Success(t *testing.T) {
	var capturedIdentity *userdomain.UserIdentity
	svc := Service{
		Clock: svcFakeClock{fixedTime: timeNow()},
		UUID:  svcFakeUUID{fixedID: "identity-1"},
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return &userdomain.User{ID: "user-1"}, nil
				},
				createIdentityFn: func(_ context.Context, identity *userdomain.UserIdentity) error {
					capturedIdentity = identity
					return nil
				},
			},
		},
	}
	err := svc.LinkIdentity(ctx(), LinkIdentityCommand{
		UserID:       "user-1",
		Provider:     "google",
		ProviderType: "oauth",
		ExternalID:   "ext-123",
		DisplayName:  "Google Account",
	})
	assert.NoError(t, err)
	assert.NotNil(t, capturedIdentity)
	assert.Equal(t, "identity-1", capturedIdentity.ID)
	assert.Equal(t, "user-1", capturedIdentity.UserID)
	assert.Equal(t, "google", capturedIdentity.ProviderID)
	assert.Equal(t, "ext-123", capturedIdentity.Subject)
}

func TestService_LinkIdentity_UUIDError(t *testing.T) {
	svc := Service{
		Clock: svcFakeClock{fixedTime: timeNow()},
		UUID:  svcFakeUUIDErr{},
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return &userdomain.User{ID: "user-1"}, nil
				},
			},
		},
	}
	err := svc.LinkIdentity(ctx(), LinkIdentityCommand{
		UserID:     "user-1",
		Provider:   "google",
		ExternalID: "ext-123",
	})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Tests for Service.UnlinkIdentity
// ---------------------------------------------------------------------------

func TestService_UnlinkIdentity_MissingUserID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	err := svc.UnlinkIdentity(ctx(), UnlinkIdentityCommand{
		OrganizationID: "org-1",
		IdentityID:     "identity-1",
	})
	assert.Error(t, err)
}

func TestService_UnlinkIdentity_MissingIdentityID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	err := svc.UnlinkIdentity(ctx(), UnlinkIdentityCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
	})
	assert.Error(t, err)
}

func TestService_UnlinkIdentity_UserNotFound(t *testing.T) {
	svc := Service{
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return nil, apperrors.NewNotFound("user not found")
				},
			},
		},
	}
	err := svc.UnlinkIdentity(ctx(), UnlinkIdentityCommand{
		OrganizationID: "org-1",
		UserID:         "nonexistent",
		IdentityID:     "identity-1",
	})
	assert.Error(t, err)
}

func TestService_UnlinkIdentity_Success(t *testing.T) {
	svc := Service{
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return &userdomain.User{ID: "user-1"}, nil
				},
				deleteIdentityFn: func(_ context.Context, userID string, identityID string) error {
					return nil
				},
			},
		},
	}
	err := svc.UnlinkIdentity(ctx(), UnlinkIdentityCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		IdentityID:     "identity-1",
	})
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Tests for Service.GetUserIdentities
// ---------------------------------------------------------------------------

func TestService_GetUserIdentities_MissingUserID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	_, err := svc.GetUserIdentities(ctx(), "")
	assert.Error(t, err)
}

func TestService_GetUserIdentities_UserNotFound(t *testing.T) {
	svc := Service{
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return nil, apperrors.NewNotFound("user not found")
				},
			},
		},
	}
	_, err := svc.GetUserIdentities(ctx(), "nonexistent")
	assert.Error(t, err)
}

func TestService_GetUserIdentities_Success(t *testing.T) {
	svc := Service{
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return &userdomain.User{
						ID: "user-1",
						Identities: []userdomain.UserIdentity{
							{
								ID:              "id-1",
								ProviderID:      "google",
								IdentityType:    "oauth",
								Subject:         "ext-123",
								ProviderAccount: "Google Account",
								LinkedAt:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
							},
						},
					}, nil
				},
			},
		},
	}
	result, err := svc.GetUserIdentities(ctx(), "user-1")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "id-1", result[0].ID)
	assert.Equal(t, "google", result[0].Provider)
	assert.Equal(t, "oauth", result[0].ProviderType)
	assert.Equal(t, "ext-123", result[0].ExternalID)
}

func TestService_GetUserIdentities_EmptyIdentities(t *testing.T) {
	svc := Service{
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return &userdomain.User{ID: "user-1"}, nil
				},
			},
		},
	}
	result, err := svc.GetUserIdentities(ctx(), "user-1")
	assert.NoError(t, err)
	assert.Empty(t, result)
}

// ---------------------------------------------------------------------------
// Tests for Service.ListUserTransactions
// ---------------------------------------------------------------------------

func TestService_ListUserTransactions_MissingUserID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	_, _, err := svc.ListUserTransactions(ctx(), "org-1", "", 1, 10)
	assert.Error(t, err)
}

func TestService_ListUserTransactions_MissingOrganizationID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	_, _, err := svc.ListUserTransactions(ctx(), "", "user-1", 1, 10)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Tests for Service.ListUserConsents
// ---------------------------------------------------------------------------

func TestService_ListUserConsents_MissingUserID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	_, err := svc.ListUserConsents(ctx(), "org-1", "")
	assert.Error(t, err)
}

func TestService_ListUserConsents_MissingOrganizationID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	_, err := svc.ListUserConsents(ctx(), "", "user-1")
	assert.Error(t, err)
}

func TestService_ListUserConsents_UserNotFound(t *testing.T) {
	svc := Service{
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return nil, apperrors.NewNotFound("user not found")
				},
			},
		},
	}
	_, err := svc.ListUserConsents(ctx(), "org-1", "nonexistent")
	assert.Error(t, err)
}

func TestService_ListUserConsents_Success(t *testing.T) {
	svc := Service{
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return &userdomain.User{
						ID: "user-1",
						ApplicationScopes: []userdomain.ConsentRecord{
							{
								Application: "app-1",
								Scopes:      []string{"read", "write"},
								GrantedAt:   "2026-01-01T00:00:00Z",
							},
						},
					}, nil
				},
			},
		},
	}
	result, err := svc.ListUserConsents(ctx(), "org-1", "user-1")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "app-1", result[0].ApplicationID)
	assert.Equal(t, []string{"read", "write"}, result[0].GrantedScopes)
}

// ---------------------------------------------------------------------------
// Tests for Service.RevokeUserConsent
// ---------------------------------------------------------------------------

func TestService_RevokeUserConsent_MissingUserID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	err := svc.RevokeUserConsent(ctx(), RevokeConsentCommand{
		OrganizationID: "org-1",
		ConsentID:      "app-1",
	})
	assert.Error(t, err)
}

func TestService_RevokeUserConsent_MissingOrganizationID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	err := svc.RevokeUserConsent(ctx(), RevokeConsentCommand{
		UserID:    "user-1",
		ConsentID: "app-1",
	})
	assert.Error(t, err)
}

func TestService_RevokeUserConsent_MissingConsentID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	err := svc.RevokeUserConsent(ctx(), RevokeConsentCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
	})
	assert.Error(t, err)
}

func TestService_RevokeUserConsent_UserNotFound(t *testing.T) {
	svc := Service{
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return nil, apperrors.NewNotFound("user not found")
				},
			},
		},
	}
	err := svc.RevokeUserConsent(ctx(), RevokeConsentCommand{
		OrganizationID: "org-1",
		UserID:         "nonexistent",
		ConsentID:      "app-1",
	})
	assert.Error(t, err)
}

func TestService_RevokeUserConsent_ConsentNotFound(t *testing.T) {
	svc := Service{
		Clock: svcFakeClock{fixedTime: timeNow()},
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return &userdomain.User{
						ID:                "user-1",
						ApplicationScopes: []userdomain.ConsentRecord{},
					}, nil
				},
			},
		},
	}
	err := svc.RevokeUserConsent(ctx(), RevokeConsentCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		ConsentID:      "nonexistent-app",
	})
	assert.Error(t, err)
}

func TestService_RevokeUserConsent_Success(t *testing.T) {
	var updatedUser *userdomain.User
	svc := Service{
		Clock: svcFakeClock{fixedTime: timeNow()},
		UOW: svcTestUnitOfWork{
			users: &svcMockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return &userdomain.User{
						ID: "user-1",
						ApplicationScopes: []userdomain.ConsentRecord{
							{Application: "app-1", Scopes: []string{"read"}},
							{Application: "app-2", Scopes: []string{"write"}},
						},
					}, nil
				},
				updateFn: func(_ context.Context, u *userdomain.User) error {
					updatedUser = u
					return nil
				},
			},
		},
	}
	err := svc.RevokeUserConsent(ctx(), RevokeConsentCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		ConsentID:      "app-1",
	})
	assert.NoError(t, err)
	assert.NotNil(t, updatedUser)
	assert.Len(t, updatedUser.ApplicationScopes, 1)
	assert.Equal(t, "app-2", updatedUser.ApplicationScopes[0].Application)
}
