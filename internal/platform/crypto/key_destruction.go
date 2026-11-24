package crypto

import (
	"fmt"
	"sync"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DestructionRequest represents a pending request to destroy (remove) a KEK
// version from the available set. Destroying a KEK version means tokens
// encrypted with that version can no longer be decrypted, so this operation
// requires dual approval (two distinct approvers).
type DestructionRequest struct {
	ID         string
	Version    int
	Reason     string
	Requester  string
	Approvers  map[string]time.Time
	CreatedAt  time.Time
	ApprovedAt *time.Time
	Destroyed  bool
}

// KeyDestructionApprover implements the dual-approval flow for KEK version
// destruction. Two distinct approvers must approve a request before the
// version can be destroyed.
type KeyDestructionApprover struct {
	mu       sync.Mutex
	requests map[string]*DestructionRequest
	seq      int
}

// NewKeyDestructionApprover creates a new approver.
func NewKeyDestructionApprover() *KeyDestructionApprover {
	return &KeyDestructionApprover{
		requests: make(map[string]*DestructionRequest),
	}
}

// Submit creates a new destruction request for the given KEK version.
// The requester cannot approve their own request.
func (a *KeyDestructionApprover) Submit(version int, reason, requester string) (*DestructionRequest, error) {
	if requester == "" {
		return nil, apperrors.NewInvalidArgument("requester is required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	a.seq++
	id := fmt.Sprintf("destroy-%d", a.seq)
	req := &DestructionRequest{
		ID:        id,
		Version:   version,
		Reason:    reason,
		Requester: requester,
		Approvers: make(map[string]time.Time),
		CreatedAt: time.Now(),
	}
	a.requests[id] = req
	return req, nil
}

// Approve adds an approver to the request. The requester cannot approve their
// own request, and an approver can only approve once. Returns true if the
// request now has two distinct approvals and is ready for destruction.
func (a *KeyDestructionApprover) Approve(requestID, approver string) (bool, error) {
	if approver == "" {
		return false, apperrors.NewInvalidArgument("approver is required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	req, ok := a.requests[requestID]
	if !ok {
		return false, apperrors.NewNotFoundf("request %s not found", requestID)
	}
	if req.Destroyed {
		return false, apperrors.NewConflictf("request %s already destroyed", requestID)
	}
	if approver == req.Requester {
		return false, apperrors.NewForbidden("requester cannot approve their own destruction request")
	}
	if _, already := req.Approvers[approver]; already {
		return false, apperrors.NewConflictf("approver %s has already approved", approver)
	}

	req.Approvers[approver] = time.Now()

	if len(req.Approvers) >= 2 {
		now := time.Now()
		req.ApprovedAt = &now
		return true, nil
	}
	return false, nil
}

// Ready returns true if the request has two distinct approvals.
func (a *KeyDestructionApprover) Ready(requestID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	req, ok := a.requests[requestID]
	if !ok {
		return false
	}
	return req.ApprovedAt != nil && !req.Destroyed
}

// Destroy marks a fully-approved request as destroyed. The caller is
// responsible for actually removing the KEK version from the encryptor's
// key set. Returns the version to destroy, or an error if not ready.
func (a *KeyDestructionApprover) Destroy(requestID string) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	req, ok := a.requests[requestID]
	if !ok {
		return 0, apperrors.NewNotFoundf("request %s not found", requestID)
	}
	if req.ApprovedAt == nil {
		return 0, apperrors.NewConflictf("request %s has not received dual approval", requestID)
	}
	if req.Destroyed {
		return 0, apperrors.NewConflictf("request %s already destroyed", requestID)
	}

	req.Destroyed = true
	return req.Version, nil
}

// Get returns a copy of the request, or nil if not found.
func (a *KeyDestructionApprover) Get(requestID string) *DestructionRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	req, ok := a.requests[requestID]
	if !ok {
		return nil
	}
	cp := *req
	cp.Approvers = make(map[string]time.Time, len(req.Approvers))
	for k, v := range req.Approvers {
		cp.Approvers[k] = v
	}
	return &cp
}

// List returns copies of all requests.
func (a *KeyDestructionApprover) List() []*DestructionRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]*DestructionRequest, 0, len(a.requests))
	for _, req := range a.requests {
		cp := *req
		cp.Approvers = make(map[string]time.Time, len(req.Approvers))
		for k, v := range req.Approvers {
			cp.Approvers[k] = v
		}
		out = append(out, &cp)
	}
	return out
}
