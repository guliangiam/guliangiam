package batch

import (
	"context"
	"sync"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const maxOperationsPerRequest = 100

type BatchOperation struct {
	Method string         `json:"method"`
	Path   string         `json:"path"`
	Body   map[string]any `json:"body"`
}

type BatchResult struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type BatchRequest struct {
	Operations []BatchOperation `json:"operations"`
}

type BatchResponse struct {
	Results []BatchResult `json:"results"`
}

type ExecutorFunc func(ctx context.Context, op BatchOperation) BatchResult

type Handler struct {
	executors map[string]ExecutorFunc
}

func NewHandler(executors map[string]ExecutorFunc) Handler {
	return Handler{executors: executors}
}

func (h Handler) HandleBatch(ctx context.Context, req BatchRequest) (BatchResponse, error) {
	if len(req.Operations) == 0 {
		return BatchResponse{Results: []BatchResult{}}, nil
	}

	if len(req.Operations) > maxOperationsPerRequest {
		return BatchResponse{}, apperrors.NewInvalidArgument("batch request exceeds maximum of 100 operations")
	}

	results := make([]BatchResult, len(req.Operations))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i, op := range req.Operations {
		wg.Add(1)
		go func(idx int, operation BatchOperation) {
			defer wg.Done()

			executor, ok := h.executors[operation.Path]
			if !ok {
				mu.Lock()
				results[idx] = BatchResult{
					Method: operation.Method,
					Path:   operation.Path,
					Status: "error",
					Error:  "unsupported batch path",
				}
				mu.Unlock()
				return
			}

			result := executor(ctx, operation)
			mu.Lock()
			results[idx] = result
			mu.Unlock()
		}(i, op)
	}

	wg.Wait()
	return BatchResponse{Results: results}, nil
}
