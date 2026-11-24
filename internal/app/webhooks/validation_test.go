package webhooks

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
)

// BE-B-17: URL is required and must be an absolute http(s) URL; headers must
// be valid JSON when present. 缺 URL 是 400（缺字段）；scheme/host 违规是 422
// （语义校验，手册 4.14.1）。
func TestValidateWebhookURL(t *testing.T) {
	cases := []struct {
		name      string
		url       string
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{"empty", "", true, assertInvalidArgument},
		{"blank", "   ", true, assertInvalidArgument},
		{"relative", "/hook", true, assertUnprocessableEntity},
		{"ftp scheme", "ftp://example.com/hook", true, assertUnprocessableEntity},
		{"no host", "https://", true, assertUnprocessableEntity},
		{"http ok", "http://example.com/hook", false, nil},
		{"https ok", "https://example.com:8443/hook?x=1", false, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateWebhookURL(tc.url)
			if tc.wantErr {
				tc.assertErr(t, err)
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateWebhookHeaders(t *testing.T) {
	if err := validateWebhookHeaders(""); err != nil {
		t.Fatalf("empty headers must be accepted: %v", err)
	}
	if err := validateWebhookHeaders(`{"X-Token":"abc"}`); err != nil {
		t.Fatalf("valid JSON headers rejected: %v", err)
	}
	assertUnprocessableEntity(t, validateWebhookHeaders(`{not json`))
}

func TestCreateWebhook_RejectsInvalidURLBeforeWrite(t *testing.T) {
	repo := &mockWebhookRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "wh-id"},
		UOW:   testUnitOfWork{webhookRepo: repo, eventRepo: &mockEventRepo{}},
	}
	_, err := svc.CreateWebhook(context.Background(), CreateWebhookCommand{
		OrganizationID: "org-1", Name: "test", URL: "ftp://example.com",
	})
	assertUnprocessableEntity(t, err)
	if len(repo.webhooks) != 0 {
		t.Fatalf("invalid URL must not persist a webhook, got %d", len(repo.webhooks))
	}
}

func TestCreateWebhook_RejectsInvalidHeaders(t *testing.T) {
	repo := &mockWebhookRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "wh-id"},
		UOW:   testUnitOfWork{webhookRepo: repo, eventRepo: &mockEventRepo{}},
	}
	_, err := svc.CreateWebhook(context.Background(), CreateWebhookCommand{
		OrganizationID: "org-1", Name: "test", URL: "https://example.com/hook", Headers: "{bad",
	})
	assertUnprocessableEntity(t, err)
	if len(repo.webhooks) != 0 {
		t.Fatalf("invalid headers must not persist a webhook, got %d", len(repo.webhooks))
	}
}

func TestPatchWebhook_RejectsInvalidURLAndHeadersBeforeTransaction(t *testing.T) {
	// Validation runs before WithinTransaction, so a zero-value Service (no
	// UOW) proves no transaction is opened for invalid input.
	svc := Service{}
	badURL := "ftp://example.com"
	err := svc.PatchWebhook(context.Background(), "org-1", PatchWebhookCommand{ID: "wh-1", URL: &badURL})
	assertUnprocessableEntity(t, err)
	badHeaders := "{bad"
	err = svc.PatchWebhook(context.Background(), "org-1", PatchWebhookCommand{ID: "wh-1", Headers: &badHeaders})
	assertUnprocessableEntity(t, err)
}

// BE-B-7: PATCH secret semantics — nil keeps, non-nil replaces, empty clears.
func TestPatchWebhook_SecretPointerSemantics(t *testing.T) {
	cases := []struct {
		name   string
		secret *string
		want   string
	}{
		{"nil keeps stored secret", nil, "whsec_stored"},
		{"non-nil replaces", strPtr("whsec_new"), "whsec_new"},
		{"empty clears", strPtr(""), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			repo := &mockWebhookRepo{webhooks: []*domain.Webhook{
				{ID: "wh-1", OrganizationID: "org-1", Name: "test", Secret: "whsec_stored", Revision: 1, CreatedAt: now, UpdatedAt: now},
			}}
			svc := Service{
				Clock: fakeClock{fixedTime: now},
				UOW:   testUnitOfWork{webhookRepo: repo, eventRepo: &mockEventRepo{}},
			}
			err := svc.PatchWebhook(context.Background(), "org-1", PatchWebhookCommand{ID: "wh-1", Secret: tc.secret, Revision: 1})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo.webhooks[0].Secret != tc.want {
				t.Fatalf("Secret = %q, want %q", repo.webhooks[0].Secret, tc.want)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

// BE-B-7: the detail projection exposes headers plus the masked secret
// (XxxSet + XxxMasked), never the plaintext.
func TestToDetail_HeadersAndMaskedSecret(t *testing.T) {
	w := &domain.Webhook{
		ID: "wh-1", OrganizationID: "org-1", Name: "test",
		Headers: `{"X-Token":"abc"}`,
		Secret:  "whsec_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	detail := toDetail(w)
	if detail.Headers != w.Headers {
		t.Fatalf("Headers = %q, want %q", detail.Headers, w.Headers)
	}
	if !detail.SecretSet {
		t.Fatal("SecretSet must be true when a secret is stored")
	}
	if detail.SecretMasked != "wh***ef" {
		t.Fatalf("SecretMasked = %q, want wh***ef", detail.SecretMasked)
	}
	if detail.SecretMasked == w.Secret {
		t.Fatal("masked secret must not equal the plaintext")
	}

	empty := toDetail(&domain.Webhook{ID: "wh-2"})
	if empty.SecretSet || empty.SecretMasked != "" {
		t.Fatalf("unset secret must project SecretSet=false, SecretMasked=\"\", got %+v", empty)
	}

	short := toDetail(&domain.Webhook{ID: "wh-3", Secret: "abc"})
	if !short.SecretSet || short.SecretMasked != "******" {
		t.Fatalf("short secret must be fully masked, got %+v", short)
	}
}
