package radius

import (
	"context"
	"testing"
)

func TestNewServer(t *testing.T) {
	s := NewServer("0.0.0.0:1812", "secret123")
	if s == nil {
		t.Fatal("expected non-nil server")
	}
	if s.addr != "0.0.0.0:1812" {
		t.Errorf("expected addr=0.0.0.0:1812, got %s", s.addr)
	}
	if s.secret != "secret123" {
		t.Errorf("expected secret=secret123, got %s", s.secret)
	}
}

func TestServer_SetAuthHandler(t *testing.T) {
	s := NewServer("0.0.0.0:1812", "secret")
	s.SetAuthHandler(func(_ context.Context, _, _ string) (bool, error) {
		return true, nil
	})
	if s.authFunc == nil {
		t.Error("expected authFunc to be set")
	}
}

func TestServer_SetAcctHandler(t *testing.T) {
	s := NewServer("0.0.0.0:1813", "secret")
	s.SetAcctHandler(func(_ context.Context, _, _, _ string, _, _ int64) {})
	if s.acctFunc == nil {
		t.Error("expected acctFunc to be set")
	}
}

func TestServer_Stop_NoServer(t *testing.T) {
	s := NewServer("0.0.0.0:1812", "secret")
	err := s.Stop()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
