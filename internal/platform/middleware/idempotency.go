package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/guliangiam/guliangiam/internal/domain/idempotency"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	httperr "github.com/guliangiam/guliangiam/internal/platform/http"
)

// IdempotencyKeyHeader is the standard header clients send with write requests
// to achieve end-to-end idempotency (ADR-018).
const IdempotencyKeyHeader = "Idempotency-Key"

// IdempotencyReplayedHeader is the response header set when a request was
// served from cache rather than executed fresh.
const IdempotencyReplayedHeader = "Idempotency-Replayed"

// idempotencyBodyLimit bounds how much of the request body the middleware
// buffers for hashing. Bodies larger than this are passed through untouched
// (BE-E-05): truncating them would corrupt uploads and make the hash cover
// only a prefix. The bootstrap MaxBytesReader wall still caps the real body.
const idempotencyBodyLimit = 1 << 20 // 1 MiB

// idempotencyCompleteTimeout bounds the post-response Complete write, which
// runs on a context detached from the request (BE-E-06) so a client
// disconnect cannot strand the record in in_progress.
const idempotencyCompleteTimeout = 5 * time.Second

// IdempotencyMiddleware returns a middleware that enforces HTTP write idempotency
// per ADR-018 (Unified Idempotency).
//
// Flow:
//  1. No Idempotency-Key header → pass through (middleware is opt-in by the client)
//  2. Read + hash request body → close body for downstream handlers
//  3. TryAcquire in_progress record — if existing:
//     a. same hash + succeeded → return cached response + Idempotency-Replayed: true
//     b. same hash + in_progress → return 425 Too Early (don't double-execute)
//     c. different hash → return 409 Conflict (key collision = client bug)
//  4. Record acquired → wrap ResponseWriter, capture response, Complete on success
//
// identity resolves the trusted (organization slug, user ID) pair from the
// session context; both feed the scope so a key can never replay another
// user's stored response (BE-E-01).
func IdempotencyMiddleware(repo idempotency.Repository, identity func(ctx context.Context) (orgSlug, userID string)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get(IdempotencyKeyHeader)
			if key == "" {
				// No key → no idempotency, just pass through
				next.ServeHTTP(w, r)
				return
			}

			// Only write verbs are idempotent. GET/HEAD/DELETE are already idempotent
			// per HTTP spec; allowing them would just waste DB cycles.
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch:
				// ok
			default:
				next.ServeHTTP(w, r)
				return
			}

			// 读请求体 + 计算哈希。上限+1 探测：超过 1MiB 的请求体不截断、不替换，
			// 原样交还下游（前缀 + 剩余流拼接）并跳过幂等记录（BE-E-05）。
			body, err := io.ReadAll(io.LimitReader(r.Body, idempotencyBodyLimit+1))
			if err != nil {
				httperr.WriteError(w, r, apperrors.NewInvalidArgument("idempotency: read request body"))
				return
			}
			if len(body) > idempotencyBodyLimit {
				slog.WarnContext(r.Context(), "idempotency: request body exceeds hashing limit; passing through without idempotency",
					"request_path", r.URL.Path,
					"request_method", r.Method,
					"limit_bytes", idempotencyBodyLimit,
				)
				r.Body = struct {
					io.Reader
					io.Closer
				}{
					Reader: io.MultiReader(bytes.NewReader(body), r.Body),
					Closer: r.Body,
				}
				next.ServeHTTP(w, r)
				return
			}
			r.Body.Close()
			// 放回 body 给下游 handler 读
			r.Body = io.NopCloser(bytes.NewReader(body))

			hash := hashBody(body)
			orgSlug, userID := identity(r.Context())
			scope := idempotency.ScopeFor(orgSlug, userID, r.URL.Path)

			rec := &idempotency.Record{
				Scope:         scope,
				Key:           key,
				RequestMethod: r.Method,
				RequestPath:   r.URL.Path,
				RequestHash:   hash,
			}

			existing, acquired, err := repo.TryAcquire(r.Context(), rec)
			if err != nil {
				// 数据库故障 → fail-open，但必须留下可观测日志：
				// 静默放行曾掩盖 RLS/顺序缺陷（DEF-31），也违反 ADR-018 的可运维性要求。
				slog.WarnContext(r.Context(), "idempotency: acquire failed; failing open",
					"error", err,
					"scope", scope,
					"request_path", r.URL.Path,
					"request_method", r.Method,
				)
				next.ServeHTTP(w, r)
				return
			}

			if !acquired && existing != nil {
				// 已有记录 — 检查状态 + 哈希
				if existing.RequestHash != hash {
					// 同键不同体 → 409 Conflict
					httperr.WriteError(w, r, apperrors.NewConflict(
						"idempotency: key reused with different request body"))
					return
				}
				switch existing.Status {
				case idempotency.StatusSucceeded:
					// 成功的缓存响应
					w.Header().Set(IdempotencyReplayedHeader, "true")
					// 还原捕获的 Content-Type（BE-E-12）；旧行 NULL/空则保持
					// 既有默认行为（不设置显式头，由 net/http 按 body 嗅探）。
					if existing.ContentType != nil && *existing.ContentType != "" {
						w.Header().Set("Content-Type", *existing.ContentType)
					}
					w.WriteHeader(intSafe(existing.ResponseStatus, 200))
					if existing.ResponseBody != nil {
						if _, err := w.Write([]byte(*existing.ResponseBody)); err != nil {
							slog.Warn("idempotency: replay response write failed", "error", err)
						}
					}
					return
				case idempotency.StatusInProgress:
					// 另一个请求正在处理 — 返回 503 Service Unavailable + Retry-After
					w.Header().Set("Retry-After", "2")
					httperr.WriteError(w, r, apperrors.NewUnavailable(
						"idempotency: request in progress, retry later"))
					return
				case idempotency.StatusFailed:
					// 失败的请求 — 不缓存，允许重试。重新执行并捕获终态（BE-E-12）：
					// PostgreSQL 仓储的 TryAcquire 已把同哈希 failed 行翻回
					// in_progress（acquired=true），走到这里说明方言镜像未实现
					// 抢占；仍复用既有行 ID 尽力 Complete，失败只记 WARN。
					rec.ID = existing.ID
				}
			}

			// 成功获取（或 failed 重试）— 包装 ResponseWriter 来捕获响应
			cw := &captureWriter{ResponseWriter: w}
			next.ServeHTTP(cw, r)

			// 记录响应
			status := idempotency.StatusSucceeded
			if cw.status >= 500 {
				status = idempotency.StatusFailed
			}
			respBody := cw.body.String()
			// 还原捕获的显式 Content-Type（BE-E-12）：在首次写出时取值，
			// 只记录 handler 显式设置的头；未设置则为空 → 存 NULL，
			// 回放保持既有默认行为（由 net/http 按 body 嗅探）。
			respContentType := cw.contentType
			// Complete 必须脱离请求取消（BE-E-06）：客户端断连会取消 r.Context()，
			// 若沿用该 ctx，落库失败会让行滞留 in_progress，同 key 后续请求最长
			// 503 到 24h 过期。WithoutCancel 保留 trace 等值，仅剥离取消信号。
			completeCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), idempotencyCompleteTimeout)
			defer cancel()
			if err := repo.Complete(completeCtx, rec.ID, status, cw.status, respBody, respContentType); err != nil {
				slog.WarnContext(completeCtx, "idempotency: complete failed",
					"error", err,
					"scope", scope,
					"request_path", r.URL.Path,
					"status", cw.status,
				)
			}
		})
	}
}

func hashBody(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

func intSafe(p *int, def int) int {
	if p != nil {
		return *p
	}
	return def
}

// captureWriter wraps http.ResponseWriter to capture the status code + body
// and the explicit Content-Type at first write (BE-E-12). The header must be
// sampled before the underlying writer flushes: httptest.ResponseRecorder
// backfills a sniffed Content-Type into its header map on Write, which would
// otherwise be mistaken for a handler-set value.
type captureWriter struct {
	http.ResponseWriter
	status      int
	body        bytes.Buffer
	contentType string
}

func (c *captureWriter) WriteHeader(code int) {
	c.status = code
	c.contentType = c.Header().Get("Content-Type")
	c.ResponseWriter.WriteHeader(code)
}

func (c *captureWriter) Write(b []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
		c.contentType = c.Header().Get("Content-Type")
	}
	c.body.Write(b)
	return c.ResponseWriter.Write(b)
}
