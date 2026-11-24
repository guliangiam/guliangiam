package applications

import (
	"context"
	"errors"
	"testing"
	"time"

	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	applicationsmock "github.com/guliangiam/guliangiam/internal/domain/applications/mock"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	"go.uber.org/mock/gomock"
)

// INC-BE-02 回归：token 属于第 2 页的应用时也必须校验通过。
// 单页 PageSize:100 的旧实现会让第 2 页起的应用合法 token 恒 401。
func TestValidateSCIMBearerToken_SecondPageApplicationMatches(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)
	repo := applicationsmock.NewMockRepository(ctrl)
	uow.EXPECT().Applications().Return(repo).AnyTimes()

	const token = "scim-token-page-2"
	hash, err := platformcrypto.HashClientSecret(token)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	repo.EXPECT().ListUsableClientSecrets(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()

	page1 := make([]*domain.Application, 100)
	for i := range page1 {
		page1[i] = &domain.Application{ID: "app-" + string(rune('a'+i%26)), OrganizationID: "org-1"}
	}
	gomock.InOrder(
		repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, q domain.ListQuery) ([]*domain.Application, int64, error) {
				if q.Page != 1 {
					t.Fatalf("first List page = %d, want 1", q.Page)
				}
				if q.OrganizationID != "org-1" {
					t.Fatalf("org = %q", q.OrganizationID)
				}
				return page1, 101, nil
			}),
		repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, q domain.ListQuery) ([]*domain.Application, int64, error) {
				if q.Page != 2 {
					t.Fatalf("second List page = %d, want 2", q.Page)
				}
				return []*domain.Application{{ID: "app-last", OrganizationID: "org-1", ClientSecretHash: hash}}, 101, nil
			}),
	)

	svc := Service{UOW: uow}
	ok, err := svc.ValidateSCIMBearerToken(context.Background(), "org-1", token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("page-2 application token must validate")
	}
}

// 第 2 页应用的新版可用密钥行（active/grace）同样必须命中。
func TestValidateSCIMBearerToken_SecondPageUsableSecretMatches(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)
	repo := applicationsmock.NewMockRepository(ctrl)
	uow.EXPECT().Applications().Return(repo).AnyTimes()

	const token = "scim-token-usable"
	hash, err := platformcrypto.HashClientSecret(token)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	page1 := make([]*domain.Application, 100)
	for i := range page1 {
		page1[i] = &domain.Application{ID: "app-x", OrganizationID: "org-1"}
	}
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domain.ListQuery) ([]*domain.Application, int64, error) {
			if q.Page == 1 {
				return page1, 101, nil
			}
			return []*domain.Application{{ID: "app-last", OrganizationID: "org-1"}}, 101, nil
		}).Times(2)
	repo.EXPECT().ListUsableClientSecrets(gomock.Any(), "org-1", gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, appID string, _ time.Time) ([]*domain.ClientSecret, error) {
			if appID == "app-last" {
				return []*domain.ClientSecret{{SecretHash: hash}}, nil
			}
			return nil, nil
		}).AnyTimes()

	svc := Service{UOW: uow}
	ok, err := svc.ValidateSCIMBearerToken(context.Background(), "org-1", token)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v, want true/nil", ok, err)
	}
}

func TestValidateSCIMBearerToken_EmptyInputs(t *testing.T) {
	svc := Service{}
	for _, tc := range []struct{ org, token string }{
		{"", "tok"}, {"org-1", ""}, {"  ", "tok"}, {"org-1", "  "},
	} {
		ok, err := svc.ValidateSCIMBearerToken(context.Background(), tc.org, tc.token)
		if ok || err != nil {
			t.Fatalf("org=%q token=%q: ok=%v err=%v, want false/nil", tc.org, tc.token, ok, err)
		}
	}
}

func TestValidateSCIMBearerToken_NoMatchAfterFullPagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)
	repo := applicationsmock.NewMockRepository(ctrl)
	uow.EXPECT().Applications().Return(repo).AnyTimes()

	repo.EXPECT().ListUsableClientSecrets(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domain.ListQuery) ([]*domain.Application, int64, error) {
			if q.Page == 1 {
				items := make([]*domain.Application, 100)
				for i := range items {
					items[i] = &domain.Application{ID: "app-x", OrganizationID: "org-1"}
				}
				return items, 101, nil
			}
			return []*domain.Application{{ID: "app-last", OrganizationID: "org-1"}}, 101, nil
		}).Times(2)

	svc := Service{UOW: uow}
	ok, err := svc.ValidateSCIMBearerToken(context.Background(), "org-1", "unknown-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("unknown token must not validate")
	}
}

func TestValidateSCIMBearerToken_ListErrorPropagates(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)
	repo := applicationsmock.NewMockRepository(ctrl)
	uow.EXPECT().Applications().Return(repo).AnyTimes()

	wantErr := errors.New("db down")
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), wantErr)

	svc := Service{UOW: uow}
	ok, err := svc.ValidateSCIMBearerToken(context.Background(), "org-1", "tok")
	if ok || !errors.Is(err, wantErr) {
		t.Fatalf("ok=%v err=%v, want false/%v", ok, err, wantErr)
	}
}

func TestValidateSCIMBearerToken_SecretsErrorPropagates(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)
	repo := applicationsmock.NewMockRepository(ctrl)
	uow.EXPECT().Applications().Return(repo).AnyTimes()

	wantErr := errors.New("secrets query failed")
	repo.EXPECT().List(gomock.Any(), gomock.Any()).
		Return([]*domain.Application{{ID: "app-1", OrganizationID: "org-1"}}, int64(1), nil)
	repo.EXPECT().ListUsableClientSecrets(gomock.Any(), "org-1", "app-1", gomock.Any()).
		Return(nil, wantErr)

	svc := Service{UOW: uow}
	ok, err := svc.ValidateSCIMBearerToken(context.Background(), "org-1", "tok")
	if ok || !errors.Is(err, wantErr) {
		t.Fatalf("ok=%v err=%v, want false/%v", ok, err, wantErr)
	}
}
