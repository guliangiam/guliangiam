package actions

import (
	"context"
	"strings"
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BE-D-21：CPULimitMs 必须真实约束调用时长；输入写入失败不得被静默吞掉。

func TestWASMExecutor_CPULimitMs_CapsInvocation(t *testing.T) {
	exec := NewWASMExecutor(nil)

	start := time.Now()
	_, execErr, runErr := exec.Execute(context.Background(), fxWazeroInfiniteLoop, nil, ExecOptions{
		Timeout:    30 * time.Second,
		CPULimitMs: 150,
	})
	elapsed := time.Since(start)

	assert.Nil(t, execErr)
	require.Error(t, runErr, "CPU budget must interrupt the tight loop")
	assert.Less(t, elapsed, 1*time.Second,
		"150ms CPU budget must win over the 30s wall-clock timeout")
	errMsg := runErr.Error()
	assert.True(t,
		strings.Contains(errMsg, "context deadline exceeded") || strings.Contains(errMsg, "module closed"),
		"error must mention ctx deadline or module close, got: %s", errMsg)
}

func TestWASMExecutor_InputExceedingMemory_Rejected(t *testing.T) {
	exec := NewWASMExecutor(NoopProxy{})

	input := make([]byte, 1024*1024)
	_, execErr, runErr := exec.Execute(context.Background(), fxEchoBack, input, ExecOptions{Timeout: 5 * time.Second})

	assert.Nil(t, execErr)
	require.Error(t, runErr, "oversized input must fail instead of silently running with empty memory")
	assert.Equal(t, apperrors.CodeUnprocessableEntity, apperrors.FromError(runErr).Code)
}
