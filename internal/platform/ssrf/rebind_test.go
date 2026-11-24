package ssrf

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// DNS rebinding 回归：DialContext 必须按已校验的 numeric IP 拨号，
// 不允许“校验后再解析”。注入的 lookupIPAddr 若被调用第二次即失败。
func TestSafeTransport_DialsValidatedNumericIPWithoutReresolve(t *testing.T) {
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

	transport := SafeTransportWithAllowlist(2*time.Second, []string{"127.0.0.0/8"})
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn, err := transport.DialContext(context.Background(), "tcp", net.JoinHostPort("rebind.test", port))
	if err != nil {
		t.Fatalf("dial via validated IP failed: %v", err)
	}
	conn.Close()
	if got := atomic.LoadInt32(&lookups); got != 1 {
		t.Errorf("lookups = %d, want exactly 1", got)
	}
}

func TestSafeTransport_MixedResolutionBlockedWhenAnyIPUnsafe(t *testing.T) {
	orig := lookupIPAddr
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{
			{IP: net.ParseIP("8.8.8.8")},
			{IP: net.ParseIP("169.254.169.254")}, // 云元数据
		}, nil
	}
	defer func() { lookupIPAddr = orig }()

	transport := SafeTransport(2 * time.Second)
	conn, err := transport.DialContext(context.Background(), "tcp", "mixed.test:80")
	if err == nil {
		conn.Close()
		t.Fatal("dial must fail when any resolved IP is unsafe")
	}
}
