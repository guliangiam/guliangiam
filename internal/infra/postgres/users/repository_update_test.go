package users

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

func TestUpdate_QueryRowError(t *testing.T) {
	ctx := context.Background()
	user := &domain.User{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, user)
	assert.Error(t, err)
}

func TestUpdate_RevisionConflict(t *testing.T) {
	ctx := context.Background()
	user := &domain.User{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, user)
	assert.Error(t, err)
}

func TestUpdate_Success(t *testing.T) {
	ctx := context.Background()
	user := &domain.User{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, user)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), user.Revision)
}

func TestUpdateProfile_ExecError(t *testing.T) {
	ctx := context.Background()
	userProfile := &domain.UserProfile{}
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateProfile(ctx, userProfile)
	assert.Error(t, err)
}

func TestUpdateProfile_Success(t *testing.T) {
	ctx := context.Background()
	userProfile := &domain.UserProfile{}
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateProfile(ctx, userProfile)
	assert.NoError(t, err)
}

// 证件列必须随 PUT /profile 一起进入 UPSERT：$19-$26 对应
// passport_no/driver_license_no/id_card_expiry/issue_country/issuing_authority/
// passport_photo_url/driver_license_front_url/driver_license_back_url，空白写 NULL；
// $28-$30 是 V19 `_sealed` v2 信封孪生列（AAD 绑定 user_profiles|列|user_id）。
func TestUpdateProfile_CertificateColumnsPassedToUpsert(t *testing.T) {
	ctx := context.Background()
	var captured []any
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "user_profiles") {
				captured = append([]any(nil), args...)
			}
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateProfile(ctx, &domain.UserProfile{
		UserID:                "user-1",
		PassportNo:            "E12345678",
		DriverLicenseNo:       "D1234567",
		IDCardExpiry:          "2030-12-31",
		IssueCountry:          "CN",
		IssuingAuthority:      "MPS Beijing",
		PassportPhotoURL:      "https://cdn.example.com/passport.png",
		DriverLicenseFrontURL: "https://cdn.example.com/dl-front.png",
	})
	assert.NoError(t, err)
	assert.Len(t, captured, 30)
	assert.Equal(t, "E12345678", captured[18])
	assert.Equal(t, "D1234567", captured[19])
	assert.Equal(t, "2030-12-31", captured[20])
	assert.Equal(t, "CN", captured[21])
	assert.Equal(t, "MPS Beijing", captured[22])
	assert.Equal(t, "https://cdn.example.com/passport.png", captured[23])
	assert.Equal(t, "https://cdn.example.com/dl-front.png", captured[24])
	assert.Nil(t, captured[25], "blank driver_license_back_url must be SQL NULL")
	assert.Nil(t, captured[27], "empty id_card must keep the sealed twin NULL")

	passportSealed, ok := captured[28].(*string)
	if assert.True(t, ok && passportSealed != nil, "passport_no_sealed must be written") {
		opened, err := userEncryptedMapper.DecryptEnvelopeColumn(
			crypto.AADContext{Table: userProfilesTable, Column: "passport_no_sealed", PKValue: "user-1"}, *passportSealed)
		assert.NoError(t, err)
		assert.Equal(t, "E12345678", opened)
	}
	driverSealed, ok := captured[29].(*string)
	if assert.True(t, ok && driverSealed != nil, "driver_license_no_sealed must be written") {
		opened, err := userEncryptedMapper.DecryptEnvelopeColumn(
			crypto.AADContext{Table: userProfilesTable, Column: "driver_license_no_sealed", PKValue: "user-1"}, *driverSealed)
		assert.NoError(t, err)
		assert.Equal(t, "D1234567", opened)
	}
}

func TestUpdateCredentials_ExecError(t *testing.T) {
	ctx := context.Background()
	userCredentials := &domain.UserCredentials{}
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateCredentials(ctx, userCredentials)
	assert.Error(t, err)
}

func TestUpdateCredentials_Success(t *testing.T) {
	ctx := context.Background()
	userCredentials := &domain.UserCredentials{}
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateCredentials(ctx, userCredentials)
	assert.NoError(t, err)
}
