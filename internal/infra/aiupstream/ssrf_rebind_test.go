package aiupstream

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// DNS rebinding 回归：拨号必须使用已校验的 numeric IP，不二次解析。
func TestSSRFDialContext_DialsValidatedNumericIPWithoutReresolve(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	var lookups int32
	orig := lookupIPAddr
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		if atomic.AddInt32(&lookups, 1) > 1 {
			t.Error("host was re-resolved after validation (DNS rebinding window)")
		}
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	defer func() { lookupIPAddr = orig }()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn, err := ssrfDialContext(context.Background(), "tcp", net.JoinHostPort("rebind.test", port))
	if err != nil {
		t.Fatalf("dial via validated IP failed: %v", err)
	}
	conn.Close()
	if got := atomic.LoadInt32(&lookups); got != 1 {
		t.Errorf("lookups = %d, want exactly 1", got)
	}
}

func TestSSRFDialContext_RejectsAllDisallowed(t *testing.T) {
	orig := lookupIPAddr
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}, nil
	}
	defer func() { lookupIPAddr = orig }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := ssrfDialContext(ctx, "tcp", "metadata.test:80")
	if err == nil {
		conn.Close()
		t.Fatal("dial to link-local must be rejected")
	}
}
