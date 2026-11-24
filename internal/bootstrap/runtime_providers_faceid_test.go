package bootstrap

import (
	"context"
	"errors"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// provider=local_uniface 只提供比对，不提供活体；薄适配必须把 unimplemented
// 原样回给 app 层，不能回一个“通过”的活体结果。
func TestVerifyFaceIdLiveness_LocalUniFaceFailsClosed(t *testing.T) {
	isLive, score, _, err := verifyFaceIdLiveness(
		context.Background(), "local_uniface", "", "", "http://127.0.0.1:8100", "image", "device",
	)
	if err == nil {
		t.Fatal("expected liveness to fail closed for local_uniface")
	}
	if isLive || score != 0 {
		t.Fatalf("liveness = %v score = %v, want false/0", isLive, score)
	}

	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error type = %T, want *apperrors.Error", err)
	}
	if appErr.Code != apperrors.CodeUnimplemented {
		t.Fatalf("code = %q, want %q", appErr.Code, apperrors.CodeUnimplemented)
	}
}
