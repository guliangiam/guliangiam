package applications

import (
	"context"
	"strings"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
)

// app_form_config 的 upsert 是整行替换（EXCLUDED），空值语义由 app 层指针合并
// 决定：nil = 不修改（保留加载值），非 nil 空串 = 显式清空。这组用例钉死该契约，
// 防止未来把「未提交」误写成「清空」或反之。
func formConfigPatchFixture() (*mockApplicationRepo, Service) {
	now := time.Now()
	repo := &mockApplicationRepo{apps: []*domain.Application{{
		ID:             "app-1",
		OrganizationID: "org-1",
		Slug:           "test-app",
		DisplayName:    "Test App",
		Revision:       1,
		CreatedAt:      now,
		UpdatedAt:      now,
		CustomCss:      ".existing { color: red; }",
		FormCss:        ".login-form { max-width: 420px; }",
		HeaderHtml:     "<header>existing</header>",
	}}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: &fakeUUIDGenerator{fixedID: "test"}, UOW: testUnitOfWork{appRepo: repo}}
	return repo, svc
}

func TestService_PatchApplication_FormConfigNilPreserves(t *testing.T) {
	repo, svc := formConfigPatchFixture()
	displayName := "Renamed"
	err := svc.PatchApplication(context.Background(), PatchApplicationCommand{
		ID:             "app-1",
		OrganizationID: "org-1",
		Revision:       1,
		DisplayName:    &displayName,
	})
	if err != nil {
		t.Fatalf("PatchApplication() error = %v", err)
	}
	updated := repo.apps[0]
	if updated.CustomCss != ".existing { color: red; }" {
		t.Errorf("CustomCss = %q, want preserved existing value", updated.CustomCss)
	}
	if updated.FormCss != ".login-form { max-width: 420px; }" {
		t.Errorf("FormCss = %q, want preserved existing value", updated.FormCss)
	}
	if updated.HeaderHtml != "<header>existing</header>" {
		t.Errorf("HeaderHtml = %q, want preserved existing value", updated.HeaderHtml)
	}
}

func TestService_PatchApplication_FormConfigEmptyStringClears(t *testing.T) {
	repo, svc := formConfigPatchFixture()
	empty := ""
	err := svc.PatchApplication(context.Background(), PatchApplicationCommand{
		ID:             "app-1",
		OrganizationID: "org-1",
		Revision:       1,
		CustomCss:      &empty,
		FormCss:        &empty,
		HeaderHtml:     &empty,
	})
	if err != nil {
		t.Fatalf("PatchApplication() error = %v", err)
	}
	updated := repo.apps[0]
	if updated.CustomCss != "" || updated.FormCss != "" || updated.HeaderHtml != "" {
		t.Errorf("explicit empty strings must clear: css=%q formCss=%q header=%q",
			updated.CustomCss, updated.FormCss, updated.HeaderHtml)
	}
}

// V17 容量迁移后 CSS/HTML 列为 TEXT，app 层不得残留 255 字符上限。
func TestService_PatchApplication_FormConfigLongText(t *testing.T) {
	repo, svc := formConfigPatchFixture()
	longCss := strings.Repeat(".login-content .gl-panel { border-radius: 12px; }\n", 40)
	longHtml := "<footer>" + strings.Repeat("brand ", 200) + "</footer>"
	err := svc.PatchApplication(context.Background(), PatchApplicationCommand{
		ID:             "app-1",
		OrganizationID: "org-1",
		Revision:       1,
		CustomCss:      &longCss,
		FooterHtml:     &longHtml,
	})
	if err != nil {
		t.Fatalf("PatchApplication() error = %v", err)
	}
	updated := repo.apps[0]
	if updated.CustomCss != longCss {
		t.Errorf("CustomCss length = %d, want %d (TEXT column must not cap at 255)", len(updated.CustomCss), len(longCss))
	}
	if updated.FooterHtml != longHtml {
		t.Errorf("FooterHtml length = %d, want %d", len(updated.FooterHtml), len(longHtml))
	}
}

// Revision 0 是 legacy 客户端边界：跳过乐观锁守卫但仍递增落库 revision。
func TestService_PatchApplication_RevisionZeroSkipsGuard(t *testing.T) {
	now := time.Now()
	repo := &mockApplicationRepo{apps: []*domain.Application{{
		ID: "app-1", OrganizationID: "org-1", Slug: "test-app", DisplayName: "Test App",
		Revision: 5, CreatedAt: now, UpdatedAt: now,
	}}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: &fakeUUIDGenerator{fixedID: "test"}, UOW: testUnitOfWork{appRepo: repo}}
	displayName := "Renamed"
	err := svc.PatchApplication(context.Background(), PatchApplicationCommand{
		ID:             "app-1",
		OrganizationID: "org-1",
		DisplayName:    &displayName,
		Revision:       0,
	})
	if err != nil {
		t.Fatalf("PatchApplication() with revision 0 error = %v", err)
	}
	if repo.apps[0].Revision != 6 {
		t.Errorf("Revision = %d, want 6", repo.apps[0].Revision)
	}
}
