package jwks

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingJWKSServer serves a mutable JWKS and counts every HTTP fetch so the
// tests can assert exactly how many source refreshes a lookup caused.
type countingJWKSServer struct {
	srv *httptest.Server

	mu      sync.Mutex
	keys    []JWK
	status  int
	fetches int

	block   chan struct{}
	started chan struct{}
}

func newCountingJWKSServer(t *testing.T, keys ...JWK) *countingJWKSServer {
	t.Helper()
	s := &countingJWKSServer{
		keys:    keys,
		status:  http.StatusOK,
		started: make(chan struct{}, 16),
	}
	s.srv = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *countingJWKSServer) handle(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	s.fetches++
	status := s.status
	block := s.block
	keys := append([]JWK(nil), s.keys...)
	s.mu.Unlock()

	if block != nil {
		select {
		case s.started <- struct{}{}:
		default:
		}
		<-block
	}

	if status != http.StatusOK {
		w.WriteHeader(status)
		return
	}
	body, err := json.Marshal(JWKS{Keys: keys})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

func (s *countingJWKSServer) uri() string {
	return s.srv.URL + "/jwks"
}

func (s *countingJWKSServer) setKeys(keys ...JWK) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys = keys
}

func (s *countingJWKSServer) setStatus(status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

func (s *countingJWKSServer) fetchCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fetches
}

// blockUntil makes every subsequent request block until release is closed.
func (s *countingJWKSServer) blockUntil(release chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.block = release
}

func testRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func newRemoteJWKSTestCache(t *testing.T, server *countingJWKSServer) *RemoteJWKSCache {
	t.Helper()
	cache := NewRemoteJWKSCache(time.Hour, time.Hour, server.srv.Client())
	t.Cleanup(cache.Close)
	return cache
}

// metricValue reads a CounterVec child through client_model instead of
// prometheus/testutil: testutil pulls an extra module (kylelemons/godebug),
// which this batch must not add.
func metricValue(t *testing.T, vec *prometheus.CounterVec, label string) float64 {
	t.Helper()
	metric := &dto.Metric{}
	require.NoError(t, vec.WithLabelValues(label).Write(metric))
	return metric.GetCounter().GetValue()
}

// signRS256 signs a payload so the test can prove the resolved key really
// verifies, not just that the lookup returned a non-nil pointer.
func signRS256(t *testing.T, key *rsa.PrivateKey, payload string) ([]byte, []byte) {
	t.Helper()
	digest := sha256.Sum256([]byte(payload))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return digest[:], signature
}

func TestUnknownKidTriggersSingleSourceRefreshAndResolvesNewKey(t *testing.T) {
	keyA := testRSAKey(t)
	keyB := testRSAKey(t)
	server := newCountingJWKSServer(t, jwkFromPub(&keyA.PublicKey, "kid-a"))
	cache := newRemoteJWKSTestCache(t, server)
	ctx := context.Background()

	_, err := cache.GetPublicKey(ctx, server.uri(), "kid-a")
	require.NoError(t, err)

	// Rotation just happened: the issuer now publishes the new kid.
	server.setKeys(jwkFromPub(&keyA.PublicKey, "kid-a"), jwkFromPub(&keyB.PublicKey, "kid-b"))
	fetchesBefore := server.fetchCount()
	successBefore := metricValue(t, jwksRefreshTotal, "success")
	resolvedBefore := metricValue(t, jwksUnknownKidTotal, "resolved")

	pubB, err := cache.GetPublicKey(ctx, server.uri(), "kid-b")
	require.NoError(t, err)
	require.NotNil(t, pubB)

	assert.Equal(t, fetchesBefore+1, server.fetchCount(), "unknown kid must trigger exactly one source refresh")
	assert.Equal(t, 1.0, metricValue(t, jwksRefreshTotal, "success")-successBefore)
	assert.Equal(t, 1.0, metricValue(t, jwksUnknownKidTotal, "resolved")-resolvedBefore)
	assert.Zero(t, pubB.N.Cmp(keyB.N), "resolved a key that is not the refreshed kid")

	digest, signature := signRS256(t, keyB, "rotation-overlap")
	require.NoError(t, rsa.VerifyPKCS1v15(pubB, crypto.SHA256, digest, signature),
		"token signed by the refreshed key must verify")
}

func TestUnknownKidAfterRefreshRejects(t *testing.T) {
	keyA := testRSAKey(t)
	server := newCountingJWKSServer(t, jwkFromPub(&keyA.PublicKey, "kid-a"))
	cache := newRemoteJWKSTestCache(t, server)
	ctx := context.Background()

	_, err := cache.GetPublicKey(ctx, server.uri(), "kid-a")
	require.NoError(t, err)

	fetchesBefore := server.fetchCount()
	rejectedBefore := metricValue(t, jwksUnknownKidTotal, "rejected")

	pub, err := cache.GetPublicKey(ctx, server.uri(), "kid-ghost")
	require.Nil(t, pub, "an unknown kid must never resolve through a fallback key")
	require.Error(t, err)

	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr, "unknown kid must return a platform error")
	assert.Equal(t, apperrors.CodeUnauthorized, appErr.Code)
	assert.Contains(t, err.Error(), "unknown signing key")

	assert.Equal(t, fetchesBefore+1, server.fetchCount(), "the rejection must follow exactly one refresh")
	assert.Equal(t, 1.0, metricValue(t, jwksUnknownKidTotal, "rejected")-rejectedBefore)
}

func TestUnknownKidCooldownSuppressesRepeatedRefresh(t *testing.T) {
	keyA := testRSAKey(t)
	server := newCountingJWKSServer(t, jwkFromPub(&keyA.PublicKey, "kid-a"))
	cache := newRemoteJWKSTestCache(t, server)
	cache.unknownKidCooldown = time.Minute
	ctx := context.Background()

	_, err := cache.GetPublicKey(ctx, server.uri(), "kid-a")
	require.NoError(t, err)

	fetchesBefore := server.fetchCount()
	rejectedBefore := metricValue(t, jwksUnknownKidTotal, "rejected")
	cooldownBefore := metricValue(t, jwksUnknownKidTotal, "cooldown")

	for i := 0; i < 5; i++ {
		pub, lookupErr := cache.GetPublicKey(ctx, server.uri(), "kid-ghost")
		require.Nil(t, pub)
		require.Error(t, lookupErr)
	}
	assert.Equal(t, fetchesBefore+1, server.fetchCount(),
		"repeated probes for one kid inside the cooldown must not refresh again")
	assert.Equal(t, 1.0, metricValue(t, jwksUnknownKidTotal, "rejected")-rejectedBefore)
	assert.Equal(t, 4.0, metricValue(t, jwksUnknownKidTotal, "cooldown")-cooldownBefore)

	// After the cooldown expires the same kid is allowed one more refresh.
	cache.mu.Lock()
	cache.unknownKidAttempts[unknownKidAttemptKey(server.uri(), "kid-ghost")] = time.Now().Add(-2 * time.Minute)
	cache.mu.Unlock()

	fetchesBefore = server.fetchCount()
	_, err = cache.GetPublicKey(ctx, server.uri(), "kid-ghost")
	require.Error(t, err)
	assert.Equal(t, fetchesBefore+1, server.fetchCount())
}

func TestUnknownKidFloodIsThrottledPerIssuer(t *testing.T) {
	keyA := testRSAKey(t)
	server := newCountingJWKSServer(t, jwkFromPub(&keyA.PublicKey, "kid-a"))
	cache := newRemoteJWKSTestCache(t, server)
	cache.unknownKidCooldown = time.Minute
	ctx := context.Background()

	_, err := cache.GetPublicKey(ctx, server.uri(), "kid-a")
	require.NoError(t, err)

	fetchesBefore := server.fetchCount()
	throttledBefore := metricValue(t, jwksUnknownKidTotal, "throttled")
	const extraKids = 4

	for i := 0; i < maxUnknownKidRefreshesPerWindow+extraKids; i++ {
		pub, lookupErr := cache.GetPublicKey(ctx, server.uri(), fmt.Sprintf("kid-ghost-%d", i))
		require.Nil(t, pub)
		require.Error(t, lookupErr)
	}
	assert.Equal(t, fetchesBefore+maxUnknownKidRefreshesPerWindow, server.fetchCount(),
		"rotating the kid must not bypass the per-issuer refresh budget")
	assert.Equal(t, float64(extraKids), metricValue(t, jwksUnknownKidTotal, "throttled")-throttledBefore)
}

func TestConcurrentUnknownKidLookupsShareOneRefresh(t *testing.T) {
	keyA := testRSAKey(t)
	keyB := testRSAKey(t)
	server := newCountingJWKSServer(t, jwkFromPub(&keyA.PublicKey, "kid-a"))
	cache := newRemoteJWKSTestCache(t, server)
	// Disable the cooldown so only single-flight can keep the fetch count at one.
	cache.unknownKidCooldown = 0
	ctx := context.Background()

	_, err := cache.GetPublicKey(ctx, server.uri(), "kid-a")
	require.NoError(t, err)

	server.setKeys(jwkFromPub(&keyA.PublicKey, "kid-a"), jwkFromPub(&keyB.PublicKey, "kid-b"))
	release := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	server.blockUntil(release)
	fetchesBefore := server.fetchCount()

	const workers = 16
	entered := make(chan struct{}, workers)
	errs := make(chan error, workers)
	pubs := make(chan *rsa.PublicKey, workers)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			entered <- struct{}{}
			pub, lookupErr := cache.GetPublicKey(ctx, server.uri(), "kid-b")
			pubs <- pub
			errs <- lookupErr
		}()
	}
	for i := 0; i < workers; i++ {
		<-entered
	}

	select {
	case <-server.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the refresh request never reached the JWKS server")
	}
	time.Sleep(100 * time.Millisecond) // let the remaining lookups join the flight
	assert.Equal(t, fetchesBefore+1, server.fetchCount(),
		"concurrent unknown-kid lookups must share one source refresh")

	close(release)
	released = true
	wg.Wait()
	close(errs)
	close(pubs)

	for lookupErr := range errs {
		require.NoError(t, lookupErr)
	}
	for pub := range pubs {
		require.NotNil(t, pub)
		assert.Zero(t, pub.N.Cmp(keyB.N))
	}
	assert.Equal(t, fetchesBefore+1, server.fetchCount())
}

func TestUnknownKidRefreshFailureRejectsAndDoesNotCacheSuccess(t *testing.T) {
	keyA := testRSAKey(t)
	keyB := testRSAKey(t)
	server := newCountingJWKSServer(t, jwkFromPub(&keyA.PublicKey, "kid-a"))
	cache := newRemoteJWKSTestCache(t, server)
	ctx := context.Background()

	_, err := cache.GetPublicKey(ctx, server.uri(), "kid-a")
	require.NoError(t, err)

	server.setStatus(http.StatusInternalServerError)
	fetchesBefore := server.fetchCount()
	failureBefore := metricValue(t, jwksRefreshTotal, "failure")
	refreshFailedBefore := metricValue(t, jwksUnknownKidTotal, "refresh_failed")

	pub, err := cache.GetPublicKey(ctx, server.uri(), "kid-b")
	require.Nil(t, pub, "a failed refresh must not yield a verification key")
	require.Error(t, err)

	assert.Equal(t, fetchesBefore+1, server.fetchCount())
	assert.Equal(t, 1.0, metricValue(t, jwksRefreshTotal, "failure")-failureBefore)
	assert.Equal(t, 1.0, metricValue(t, jwksUnknownKidTotal, "refresh_failed")-refreshFailedBefore)

	cache.mu.RLock()
	cached := cache.cache[server.uri()]
	var cachedKid bool
	if cached != nil {
		_, cachedKid = cached.keys["kid-b"]
	}
	cache.mu.RUnlock()
	assert.False(t, cachedKid, "a failed refresh must not insert the unknown kid")

	// The failure must not be turned into an accept, and the cooldown absorbs
	// the repeated probe instead of re-fetching.
	fetchesBefore = server.fetchCount()
	pub, err = cache.GetPublicKey(ctx, server.uri(), "kid-b")
	require.Nil(t, pub)
	require.Error(t, err)
	assert.Equal(t, fetchesBefore, server.fetchCount(),
		"a repeated probe after failure must not refresh again inside the cooldown")

	// Once the source recovers the failure is not sticky: a later probe may
	// refresh and resolve the key.
	server.setStatus(http.StatusOK)
	server.setKeys(jwkFromPub(&keyA.PublicKey, "kid-a"), jwkFromPub(&keyB.PublicKey, "kid-b"))
	cache.mu.Lock()
	cache.unknownKidAttempts[unknownKidAttemptKey(server.uri(), "kid-b")] = time.Now().Add(-2 * time.Minute)
	cache.mu.Unlock()

	pub, err = cache.GetPublicKey(ctx, server.uri(), "kid-b")
	require.NoError(t, err)
	require.NotNil(t, pub)
	assert.Zero(t, pub.N.Cmp(keyB.N))
}

func TestRetiredKeyRemovedFromJWKSStillVerifiesFromCache(t *testing.T) {
	keyA := testRSAKey(t)
	keyB := testRSAKey(t)
	server := newCountingJWKSServer(t,
		jwkFromPub(&keyA.PublicKey, "kid-a"),
		jwkFromPub(&keyB.PublicKey, "kid-b"),
	)
	cache := newRemoteJWKSTestCache(t, server)
	ctx := context.Background()

	pubB, err := cache.GetPublicKey(ctx, server.uri(), "kid-b")
	require.NoError(t, err)
	digest, signature := signRS256(t, keyB, "retired-overlap")
	require.NoError(t, rsa.VerifyPKCS1v15(pubB, crypto.SHA256, digest, signature))

	// The issuer retires the key: it disappears from JWKS while tokens minted
	// in the overlap window are still valid.
	server.setKeys(jwkFromPub(&keyA.PublicKey, "kid-a"))
	fetchesBefore := server.fetchCount()

	pubB, err = cache.GetPublicKey(ctx, server.uri(), "kid-b")
	require.NoError(t, err, "a retired key must still verify from the local cache")
	assert.Zero(t, pubB.N.Cmp(keyB.N))
	assert.Equal(t, fetchesBefore, server.fetchCount(), "a cache hit must not refresh")

	// Once the cache expires the key is no longer published: fail closed.
	cache.mu.Lock()
	cache.cache[server.uri()].fetchedAt = time.Now().Add(-2 * time.Hour)
	cache.mu.Unlock()

	pub, err := cache.GetPublicKey(ctx, server.uri(), "kid-b")
	require.Nil(t, pub,
		"a key removed from JWKS must not verify past the local cache window")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown signing key")
}
