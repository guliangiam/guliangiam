package auth

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/base64"
	"strings"
	"testing"
)

// deflate 可以把极小报文膨胀成超大数据；ParseSamlRequest 必须在上限处截断并
// 返回错误，而不是把整段解压进内存（解压炸弹）。
func TestParseSamlRequest_RejectsDecompressionBomb(t *testing.T) {
	var compressed bytes.Buffer
	w, err := flate.NewWriter(&compressed, flate.BestCompression)
	if err != nil {
		t.Fatalf("flate.NewWriter() error = %v", err)
	}
	// 1 MiB 的 'A' 压缩后只有约 1 KiB，且不含 "xmlns"，会走解压分支。
	if _, err := w.Write(bytes.Repeat([]byte("A"), 1<<20)); err != nil {
		t.Fatalf("write compressed payload: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close flate writer: %v", err)
	}

	encoded := base64.StdEncoding.EncodeToString(compressed.Bytes())

	svc := &Service{}
	_, err = svc.ParseSamlRequest(context.Background(), encoded)
	if err == nil {
		t.Fatal("expected decompression bomb to be rejected")
	}
	if !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("error = %v, want size-limit rejection", err)
	}
}
