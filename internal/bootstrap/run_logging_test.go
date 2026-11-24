package bootstrap

import (
	"bytes"
	"log"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// 启动日志不能经由 slog 默认 handler：slog.SetDefault 会把标准 log 包的输出
// 重定向到新默认 handler，而默认 handler 自身又是经 log 写出的。包装它会让
// 第一条日志从 log 的非重入互斥锁上重新进入 sanitizer，进程在打印任何内容前
// 就永久阻塞（无监听、无报错）。这里用带超时的用例把该回归钉住。
func TestSanitizingLoggerDoesNotDeadlockThroughStdLog(t *testing.T) {
	var sink bytes.Buffer
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })

	slog.SetDefault(sanitizingLoggerTo(&sink))

	done := make(chan struct{})
	go func() {
		defer close(done)
		slog.Info("startup probe", "password", "s3cret")
		log.Println("standard library probe")
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("logging deadlocked: the sanitizing handler must not wrap slog's default handler")
	}

	out := sink.String()
	if !strings.Contains(out, "startup probe") {
		t.Fatalf("structured log line missing from sink: %q", out)
	}
	if strings.Contains(out, "s3cret") {
		t.Fatalf("sensitive value leaked into the sink: %q", out)
	}
	if !strings.Contains(out, "***") {
		t.Fatalf("sanitizer did not redact the sensitive attribute: %q", out)
	}
}
