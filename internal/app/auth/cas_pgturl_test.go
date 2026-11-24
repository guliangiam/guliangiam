package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// pgtURL 来自 CAS 客户端参数（外部可控）。投递前必须拒绝非 HTTPS 目标，
// 否则会把 PGTIOU 送到明文端点，并为 SSRF 打开入口。
func TestNotifyPgtURL_RejectsNonHTTPS(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	Service{}.notifyPgtURL(context.Background(), srv.URL+"/pgt", "PGTIOU-test")

	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Fatalf("plain HTTP pgtURL received %d request(s), want 0", got)
	}
}
