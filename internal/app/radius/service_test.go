package radius

import (
	"context"
	"errors"
	"testing"
)

type fakeServer struct {
	addr     string
	secret   string
	auth     func(ctx context.Context, username, password string) (bool, error)
	started  bool
	startErr error
}

func (s *fakeServer) SetAuthHandler(fn func(ctx context.Context, username, password string) (bool, error)) {
	s.auth = fn
}

func (s *fakeServer) Start(context.Context) error {
	s.started = true
	return s.startErr
}

func (s *fakeServer) Stop() error { return nil }

func TestNewServerPassesAddressAndSecret(t *testing.T) {
	var created *fakeServer
	svc := NewService(func(addr, secret string) Server {
		created = &fakeServer{addr: addr, secret: secret}
		return created
	})
	got := svc.NewServer(":1812", "shared")
	if got != created || created.addr != ":1812" || created.secret != "shared" {
		t.Fatalf("server = %+v", created)
	}
}

func TestStartServerBindsAuthHandler(t *testing.T) {
	srv := &fakeServer{}
	svc := NewService(func(addr, secret string) Server { return srv })
	var called bool
	got, err := svc.StartServer(context.Background(), ":1812", "shared", func(context.Context, string, string) (bool, error) {
		called = true
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != srv || !srv.started || srv.auth == nil {
		t.Fatalf("server = %+v", srv)
	}
	if _, err := srv.auth(context.Background(), "u", "p"); err != nil || !called {
		t.Fatalf("auth handler not wired: called=%v err=%v", called, err)
	}
}

func TestStartServerPropagatesStartError(t *testing.T) {
	srv := &fakeServer{startErr: errors.New("port busy")}
	svc := NewService(func(addr, secret string) Server { return srv })
	if _, err := svc.StartServer(context.Background(), ":1812", "shared", nil); err == nil {
		t.Fatal("expected start error")
	}
}
