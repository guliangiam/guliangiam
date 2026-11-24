package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3RequestCapture struct {
	method           string
	path             string
	contentType      string
	contentLength    int64
	transferEncoding []string
	body             []byte
	readErr          error
}

// newFakeS3Storage points an S3 client at a local HTTP server and captures the
// single PUT request it receives. No real AWS credentials or connectivity are involved.
func newFakeS3Storage(t *testing.T) (*S3Storage, <-chan s3RequestCapture) {
	t.Helper()

	captured := make(chan s3RequestCapture, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		captured <- s3RequestCapture{
			method:           r.Method,
			path:             r.URL.Path,
			contentType:      r.Header.Get("Content-Type"),
			contentLength:    r.ContentLength,
			transferEncoding: r.TransferEncoding,
			body:             body,
			readErr:          err,
		}
		w.Header().Set("ETag", `"fake-etag"`)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := s3.NewFromConfig(aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("fake-access-key", "fake-secret-key", ""),
	}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(server.URL)
		o.UsePathStyle = true
	})

	return &S3Storage{
		client:    client,
		bucket:    "test-bucket",
		presigner: s3.NewPresignClient(client),
	}, captured
}

func TestS3PutWritesBody(t *testing.T) {
	payload := []byte("object payload")
	provider, captured := newFakeS3Storage(t)

	if err := provider.Put(context.Background(), "docs/hello.txt", payload, "text/plain"); err != nil {
		t.Fatalf("Put() error = %v, want nil", err)
	}

	req := <-captured
	if req.readErr != nil {
		t.Fatalf("fake S3 read request body: %v", req.readErr)
	}
	if req.method != http.MethodPut {
		t.Fatalf("method = %q, want PUT", req.method)
	}
	if req.path != "/test-bucket/docs/hello.txt" {
		t.Fatalf("path = %q, want /test-bucket/docs/hello.txt", req.path)
	}
	if req.contentType != "text/plain" {
		t.Fatalf("Content-Type = %q, want text/plain", req.contentType)
	}
	if req.contentLength != int64(len(payload)) {
		t.Fatalf("Content-Length = %d, want %d", req.contentLength, len(payload))
	}
	if len(req.transferEncoding) != 0 {
		t.Fatalf("Transfer-Encoding = %v, want fixed-length request", req.transferEncoding)
	}
	if !bytes.Equal(req.body, payload) {
		t.Fatalf("body = %q, want %q", req.body, payload)
	}
}

func TestS3PutEmptyBody(t *testing.T) {
	provider, captured := newFakeS3Storage(t)

	if err := provider.Put(context.Background(), "docs/empty.txt", nil, "text/plain"); err != nil {
		t.Fatalf("Put() error = %v, want nil", err)
	}

	req := <-captured
	if req.readErr != nil {
		t.Fatalf("fake S3 read request body: %v", req.readErr)
	}
	if req.contentType != "text/plain" {
		t.Fatalf("Content-Type = %q, want text/plain", req.contentType)
	}
	if req.contentLength != 0 {
		t.Fatalf("Content-Length = %d, want 0", req.contentLength)
	}
	if len(req.body) != 0 {
		t.Fatalf("body = %q, want empty", req.body)
	}
}

func TestS3PutStreamWritesBody(t *testing.T) {
	payload := []byte("streamed payload")
	provider, captured := newFakeS3Storage(t)

	if err := provider.PutStream(context.Background(), "docs/stream.txt", bytes.NewReader(payload), "text/plain"); err != nil {
		t.Fatalf("PutStream() error = %v, want nil", err)
	}

	req := <-captured
	if req.readErr != nil {
		t.Fatalf("fake S3 read request body: %v", req.readErr)
	}
	if req.contentType != "text/plain" {
		t.Fatalf("Content-Type = %q, want text/plain", req.contentType)
	}
	if !bytes.Equal(req.body, payload) {
		t.Fatalf("body = %q, want %q", req.body, payload)
	}
}

// seekableErrorReader exposes the partial payload and then returns err. It is
// seekable so the SDK checksum middleware can exercise the read path.
type seekableErrorReader struct {
	data []byte
	err  error
	pos  int
}

func (r *seekableErrorReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *seekableErrorReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		r.pos = int(offset)
	case io.SeekCurrent:
		r.pos += int(offset)
	case io.SeekEnd:
		r.pos = len(r.data) + int(offset)
	}
	return int64(r.pos), nil
}

func TestS3PutStreamPropagatesReaderError(t *testing.T) {
	boom := errors.New("reader boom")
	provider, _ := newFakeS3Storage(t)

	err := provider.PutStream(context.Background(), "docs/error.txt", &seekableErrorReader{
		data: []byte("partial"),
		err:  boom,
	}, "text/plain")
	if err == nil {
		t.Fatal("PutStream() error = nil, want reader error")
	}
	if !errors.Is(err, boom) {
		t.Fatalf("PutStream() error = %v, want wrapping %v", err, boom)
	}
}
