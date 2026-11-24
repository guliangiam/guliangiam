package actions

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestNoopProxy_AlwaysDenied(t *testing.T) {
	p := NoopProxy{}
	if p.Allowed("example.com") {
		t.Error("Allowed must always be false on NoopProxy")
	}
	if _, err := p.Get(context.Background(), "https://example.com/x"); !errors.Is(err, ErrProxyDenied) {
		t.Errorf("Get err = %v, want ErrProxyDenied", err)
	}
	if _, err := p.Post(context.Background(), "https://example.com/x", nil); !errors.Is(err, ErrProxyDenied) {
		t.Errorf("Post err = %v, want ErrProxyDenied", err)
	}
}

func TestWhitelistProxy_Allowed(t *testing.T) {
	p := NewWhitelistProxy([]string{"example.com", "api.io"}, nil)

	cases := []struct {
		host string
		want bool
	}{
		{"example.com", true},
		{"api.example.com", true},
		{"deep.nested.api.example.com", true},
		{"EXAMPLE.COM", true},  // case insensitive
		{".example.com", true}, // NewWhitelistProxy strips leading dot
		{"notexample.com", false},
		{"other.com", false},
		{"evil-example.com", false}, // prefix not allowed
		{"", false},
		{"example.com:8080", false}, // Allowed expects pure hostname
	}
	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			if got := p.Allowed(tc.host); got != tc.want {
				t.Errorf("Allowed(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}

func TestWhitelistProxy_GetAllowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	// extract host from httptest URL (scheme://host:port)
	u, _ := url.Parse(srv.URL)

	// whitelist the hostname (strip port) — easier: just whitelist the
	// full host:port for this test
	p := NewWhitelistProxy([]string{u.Hostname()}, nil)
	if !p.Allowed(u.Hostname()) {
		t.Fatalf("test hostname %q not allowed by proxy setup", u.Hostname())
	}

	data, err := p.Get(context.Background(), srv.URL+"/hello")
	if err != nil {
		t.Fatalf("Get err = %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Errorf("body = %q", data)
	}
}

func TestWhitelistProxy_RejectsOversizedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, maxProxyResponseBytes+1))
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	p := NewWhitelistProxy([]string{u.Hostname()}, nil)

	if _, err := p.Get(context.Background(), srv.URL); err == nil {
		t.Fatal("expected oversized upstream response to be rejected")
	}
}

func TestWhitelistProxy_GetDenied(t *testing.T) {
	p := NewWhitelistProxy([]string{"trusted.com"}, nil)
	_, err := p.Get(context.Background(), "https://evil.com/x")
	if !errors.Is(err, ErrProxyDenied) {
		t.Errorf("err = %v, want ErrProxyDenied", err)
	}
}

func TestWhitelistProxy_PostAllowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		_, _ = w.Write([]byte("posted"))
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	p := NewWhitelistProxy([]string{u.Hostname()}, nil)

	data, err := p.Post(context.Background(), srv.URL, []byte(`{"a":1}`))
	if err != nil {
		t.Fatalf("Post err = %v", err)
	}
	if string(data) != "posted" {
		t.Errorf("body = %q", data)
	}
}

func TestWhitelistProxy_EmptyWhitelistDeniesEverything(t *testing.T) {
	p := NewWhitelistProxy(nil, nil)
	if p.Allowed("anywhere.com") {
		t.Error("nil whitelist should deny all")
	}
	if _, err := p.Get(context.Background(), "https://anywhere.com/x"); !errors.Is(err, ErrProxyDenied) {
		t.Errorf("err = %v, want ErrProxyDenied", err)
	}
}

func TestWhitelistProxy_InvalidURL(t *testing.T) {
	p := NewWhitelistProxy([]string{"example.com"}, nil)
	if _, err := p.Get(context.Background(), "://badurl"); err == nil {
		t.Error("expected error for invalid url")
	}
}
