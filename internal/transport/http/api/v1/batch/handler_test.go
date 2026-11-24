package batch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleBatch_EmptyOperations(t *testing.T) {
	h := NewHandler(nil)
	resp, err := h.HandleBatch(context.Background(), BatchRequest{Operations: []BatchOperation{}})
	assert.NoError(t, err)
	assert.Empty(t, resp.Results)
}

func TestHandleBatch_ExceedsMaxOperations(t *testing.T) {
	h := NewHandler(nil)
	ops := make([]BatchOperation, 101)
	for i := range ops {
		ops[i] = BatchOperation{Method: "POST", Path: "/test"}
	}
	_, err := h.HandleBatch(context.Background(), BatchRequest{Operations: ops})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum")
}

func TestHandleBatch_UnsupportedPath(t *testing.T) {
	h := NewHandler(map[string]ExecutorFunc{})
	resp, err := h.HandleBatch(context.Background(), BatchRequest{
		Operations: []BatchOperation{
			{Method: "POST", Path: "/unsupported", Body: map[string]any{}},
		},
	})
	assert.NoError(t, err)
	assert.Len(t, resp.Results, 1)
	assert.Equal(t, "error", resp.Results[0].Status)
	assert.Equal(t, "unsupported batch path", resp.Results[0].Error)
}

func TestHandleBatch_WithExecutor(t *testing.T) {
	h := NewHandler(map[string]ExecutorFunc{
		"/test": func(ctx context.Context, op BatchOperation) BatchResult {
			return BatchResult{
				Method: op.Method,
				Path:   op.Path,
				Status: "success",
			}
		},
	})
	resp, err := h.HandleBatch(context.Background(), BatchRequest{
		Operations: []BatchOperation{
			{Method: "POST", Path: "/test", Body: map[string]any{"key": "value"}},
		},
	})
	assert.NoError(t, err)
	assert.Len(t, resp.Results, 1)
	assert.Equal(t, "success", resp.Results[0].Status)
	assert.Equal(t, "POST", resp.Results[0].Method)
	assert.Equal(t, "/test", resp.Results[0].Path)
}

func TestHandleBatch_MixedResults(t *testing.T) {
	h := NewHandler(map[string]ExecutorFunc{
		"/supported": func(ctx context.Context, op BatchOperation) BatchResult {
			return BatchResult{Method: op.Method, Path: op.Path, Status: "ok"}
		},
	})
	resp, err := h.HandleBatch(context.Background(), BatchRequest{
		Operations: []BatchOperation{
			{Method: "POST", Path: "/supported"},
			{Method: "DELETE", Path: "/unsupported"},
			{Method: "GET", Path: "/supported"},
		},
	})
	assert.NoError(t, err)
	assert.Len(t, resp.Results, 3)
	assert.Equal(t, "ok", resp.Results[0].Status)
	assert.Equal(t, "error", resp.Results[1].Status)
	assert.Equal(t, "ok", resp.Results[2].Status)
}

func TestHandleBatch_ExactMaxOperations(t *testing.T) {
	h := NewHandler(map[string]ExecutorFunc{
		"/test": func(ctx context.Context, op BatchOperation) BatchResult {
			return BatchResult{Method: op.Method, Path: op.Path, Status: "ok"}
		},
	})
	ops := make([]BatchOperation, 100)
	for i := range ops {
		ops[i] = BatchOperation{Method: "POST", Path: "/test"}
	}
	resp, err := h.HandleBatch(context.Background(), BatchRequest{Operations: ops})
	assert.NoError(t, err)
	assert.Len(t, resp.Results, 100)
}
