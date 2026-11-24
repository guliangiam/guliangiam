package verifications

import (
	"sync"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/recover"
)

const (
	// MaxVerifyAttempts is the maximum number of failed verification attempts
	// before the code is invalidated.
	MaxVerifyAttempts = 3

	// MaxIPVerifyAttempts is the maximum number of verification attempts
	// allowed per IP address per hour.
	MaxIPVerifyAttempts = 10

	// ipRateLimitWindow is the time window for IP-level rate limiting.
	ipRateLimitWindow = time.Hour

	// cleanupInterval is how often expired records are purged.
	cleanupInterval = 5 * time.Minute
)

// attemptRecord tracks failed verification attempts for a specific recipient+type.
type attemptRecord struct {
	count     int
	expiresAt time.Time
}

// ipRecord tracks verification attempts from a specific IP address.
type ipRecord struct {
	count     int
	expiresAt time.Time
}

// RateLimiter provides in-memory rate limiting for verification code operations.
type RateLimiter struct {
	mu       sync.RWMutex
	attempts map[string]*attemptRecord // key: "type:recipient"
	ips      map[string]*ipRecord      // key: IP address
	stopCh   chan struct{}
	clock    func() time.Time
}

// NewRateLimiter creates a new verification rate limiter and starts background cleanup.
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		attempts: make(map[string]*attemptRecord),
		ips:      make(map[string]*ipRecord),
		stopCh:   make(chan struct{}),
		clock:    time.Now,
	}
	recover.SafeGo("verification-ratelimit-cleanup", rl.cleanupLoop)
	return rl
}

// CheckAttempt checks whether a verification attempt is allowed for the given recipient.
// Returns false if the recipient has exceeded the maximum number of failed attempts.
func (r *RateLimiter) CheckAttempt(verificationType, recipient string) bool {
	key := verificationType + ":" + recipient
	r.mu.RLock()
	defer r.mu.RUnlock()

	rec, ok := r.attempts[key]
	if !ok {
		return true
	}
	if r.clock().After(rec.expiresAt) {
		return true
	}
	return rec.count < MaxVerifyAttempts
}

// RecordFailedAttempt records a failed verification attempt for the given recipient.
// If the maximum number of failed attempts is reached, subsequent attempts will be blocked.
func (r *RateLimiter) RecordFailedAttempt(verificationType, recipient string) {
	key := verificationType + ":" + recipient
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.clock()
	rec, ok := r.attempts[key]
	if !ok || now.After(rec.expiresAt) {
		r.attempts[key] = &attemptRecord{
			count:     1,
			expiresAt: now.Add(verificationCodeTimeout),
		}
		return
	}
	rec.count++
}

// ResetAttempts clears the failed attempt counter for a recipient after a successful verification.
func (r *RateLimiter) ResetAttempts(verificationType, recipient string) {
	key := verificationType + ":" + recipient
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.attempts, key)
}

// CheckIP checks whether a verification attempt is allowed from the given IP address.
// Returns false if the IP has exceeded the hourly attempt limit.
func (r *RateLimiter) CheckIP(ip string) bool {
	if ip == "" {
		return true
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	rec, ok := r.ips[ip]
	if !ok {
		return true
	}
	if r.clock().After(rec.expiresAt) {
		return true
	}
	return rec.count < MaxIPVerifyAttempts
}

// RecordIPAttempt records a verification attempt from the given IP address.
func (r *RateLimiter) RecordIPAttempt(ip string) {
	if ip == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.clock()
	rec, ok := r.ips[ip]
	if !ok || now.After(rec.expiresAt) {
		r.ips[ip] = &ipRecord{
			count:     1,
			expiresAt: now.Add(ipRateLimitWindow),
		}
		return
	}
	rec.count++
}

// Stop terminates the background cleanup goroutine.
func (r *RateLimiter) Stop() {
	close(r.stopCh)
}

// cleanupLoop periodically removes expired records.
func (r *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.cleanup()
		}
	}
}

// cleanup removes all expired attempt and IP records.
func (r *RateLimiter) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.clock()

	for k, v := range r.attempts {
		if now.After(v.expiresAt) {
			delete(r.attempts, k)
		}
	}

	for k, v := range r.ips {
		if now.After(v.expiresAt) {
			delete(r.ips, k)
		}
	}
}
