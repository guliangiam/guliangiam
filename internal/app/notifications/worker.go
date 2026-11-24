package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	providersdomain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	domain "github.com/guliangiam/guliangiam/internal/domain/notifications"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
	"github.com/guliangiam/guliangiam/internal/platform/redact"
	tentxtx "github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

const (
	workerDefaultPollingInterval = 30 * time.Second
	workerDefaultBatchSize       = 100
	workerDefaultMaxRetries      = 3
	workerDefaultTimeout         = 10 * time.Second
	workerDefaultBaseRetryDelay  = 60 * time.Second
	workerMaxRetryDelay          = 1 * time.Hour
)

// Worker polls for pending notifications and delivers them via the notification factory.
type Worker struct {
	UOW             appuow.UnitOfWork
	Factory         domain.NotificationFactory
	PollingInterval time.Duration
	BatchSize       int
	MaxRetries      int
	Timeout         time.Duration
	BaseRetryDelay  time.Duration

	// systemTx 由组合根注入：在系统作用域事务内执行 Worker 的 RLS 表读写。
	// nil 表示直连（单测路径）；生产装配必须注入（DEF-35）。
	systemTx func(context.Context, func(context.Context, appuow.UnitOfWork) error) error

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	logger  *slog.Logger
}

// SetSystemTxRunner 注入系统作用域事务执行器（生产装配使用；单测可不注入）。
func (w *Worker) SetSystemTxRunner(runner func(context.Context, func(context.Context, appuow.UnitOfWork) error) error) {
	if runner != nil {
		w.systemTx = runner
	}
}

func (w *Worker) withSystemTx(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	if w.systemTx == nil {
		return fn(ctx, w.UOW)
	}
	return w.systemTx(ctx, fn)
}

func NewWorker(uow appuow.UnitOfWork, factory domain.NotificationFactory, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{
		UOW:             uow,
		Factory:         factory,
		PollingInterval: workerDefaultPollingInterval,
		BatchSize:       workerDefaultBatchSize,
		MaxRetries:      workerDefaultMaxRetries,
		Timeout:         workerDefaultTimeout,
		BaseRetryDelay:  workerDefaultBaseRetryDelay,
		stopCh:          make(chan struct{}),
		logger:          logger.With("component", "notification-worker"),
	}
}

// Start begins the background polling loop.传入外部 ctx 用于优雅关闭；
// Stop() 仍可独立停止。两种关闭路径均幂等。
func (w *Worker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	parent := ctx
	if parent == nil {
		parent = context.Background()
	}
	w.ctx, w.cancel = context.WithCancel(parent)
	w.stopCh = make(chan struct{})
	w.mu.Unlock()

	recover.SafeGo("notification-worker", w.run)
	w.logger.Info("notification delivery worker started")
}

// Stop gracefully shuts down the worker.
func (w *Worker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	if w.cancel != nil {
		w.cancel()
	}
	close(w.stopCh)
	w.running = false
	w.logger.Info("notification delivery worker stopped")
}

func (w *Worker) run() {
	ticker := time.NewTicker(w.PollingInterval)
	defer ticker.Stop()

	// Process immediately on start
	w.processPendingNotifications()

	for {
		select {
		case <-w.stopCh:
			return
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.processPendingNotifications()
		}
	}
}

func (w *Worker) processPendingNotifications() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// notifications 表受 FORCE RLS 保护，Worker 无请求租户上下文；必须在系统
	// 作用域事务内读取，否则永远 0 行、通知永不投递（DEF-35）。
	var pending []*domain.Notification
	err := w.withSystemTx(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		items, lerr := tx.Notifications().ListPendingRetries(ctx, w.BatchSize)
		if lerr != nil {
			return lerr
		}
		pending = items
		return nil
	})
	if err != nil {
		w.logger.ErrorContext(ctx, "failed to list pending notifications", "error", err)
		return
	}

	for _, n := range pending {
		w.deliverNotification(ctx, n)
	}
}

func (w *Worker) deliverNotification(ctx context.Context, n *domain.Notification) {
	ctx = tentxtx.System(ctx)
	// Look up the notification provider to determine the provider type
	var provider *providersdomain.NotificationProvider
	err := w.withSystemTx(tentxtx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		p, perr := tx.NotificationProviders().GetByID(ctx, n.ProviderID)
		if perr != nil {
			return perr
		}
		provider = p
		return nil
	})
	if err != nil {
		w.logger.ErrorContext(ctx, "failed to fetch notification provider",
			"notification_id", n.ID,
			"provider_id", n.ProviderID,
			"error", err,
		)
		// Mark as failed — cannot deliver without a provider
		if markErr := w.markFailed(ctx, n.ID, fmt.Sprintf("provider not found: %s", n.ProviderID)); markErr != nil {
			w.logger.ErrorContext(ctx, "failed to mark notification as failed", "notification_id", n.ID, "error", markErr)
		}
		return
	}

	// BE-B-3 defense in depth: rows written before the create-side ownership
	// check (or by any other path) must never deliver through another
	// organization's provider credentials.
	if provider.OrganizationID != n.OrganizationID {
		w.logger.ErrorContext(ctx, "notification provider organization mismatch; refusing delivery",
			"notification_id", n.ID,
			"provider_id", provider.ID,
		)
		if markErr := w.markFailed(ctx, n.ID, "provider organization mismatch"); markErr != nil {
			w.logger.ErrorContext(ctx, "failed to mark notification as failed", "notification_id", n.ID, "error", markErr)
		}
		return
	}

	if !provider.IsEnabled {
		// A disabled provider can never deliver this notification. Leaving the
		// row pending would park it at the head of the ASC retry queue forever
		// and starve every later notification (BE-B-9). Terminate it instead.
		w.logger.InfoContext(ctx, "notification provider disabled; failing notification",
			"notification_id", n.ID,
			"provider_id", provider.ID,
		)
		if markErr := w.markFailed(ctx, n.ID, "notification provider is disabled"); markErr != nil {
			w.logger.ErrorContext(ctx, "failed to mark notification as failed", "notification_id", n.ID, "error", markErr)
		}
		return
	}

	// Determine provider type: use provider.ProviderType, fallback to inferring from notification type
	providerType := provider.ProviderType
	if providerType == "" {
		w.logger.WarnContext(ctx, "notification provider has no type; failing notification",
			"notification_id", n.ID,
			"provider_id", provider.ID,
		)
		if markErr := w.markFailed(ctx, n.ID, "notification provider has no type"); markErr != nil {
			w.logger.ErrorContext(ctx, "failed to mark notification as failed", "notification_id", n.ID, "error", markErr)
		}
		return
	}

	// BE-B-9: one delivery attempt per polling cycle. The previous in-process
	// retry loop slept BaseRetryDelay (60s) inside a 60s batch context, so the
	// first retry never ran and retry_count was never incremented: a permanently
	// failing notification stayed pending at retry_count=0 and blocked the head
	// of the ASC queue forever. Retries are now driven by the persisted
	// retry_count (ListPendingRetries filters retry_count < 3, ASC), and every
	// failure advances the row so the queue always drains.
	sendN := &domain.SendNotification{
		Type:           providerType,
		OrganizationID: n.OrganizationID,
		Recipient:      n.Recipient,
		Subject:        n.Subject,
		Content:        n.Content,
		Metadata:       n.Variables,
	}

	if provider.WebhookURL != "" {
		if sendN.Metadata == nil {
			sendN.Metadata = make(map[string]interface{})
		}
		sendN.Metadata["webhook_url"] = provider.WebhookURL
	}
	if provider.AccessToken != "" {
		if sendN.Metadata == nil {
			sendN.Metadata = make(map[string]interface{})
		}
		sendN.Metadata["access_token"] = provider.AccessToken
	}
	for k, v := range provider.Config {
		sendN.Metadata[k] = v
	}

	timeout := w.Timeout
	if timeout <= 0 {
		timeout = workerDefaultTimeout
	}
	sendCtx, sendCancel := context.WithTimeout(ctx, timeout)
	err = w.Factory.SendNotification(sendCtx, providerType, sendN)
	sendCancel()

	if err == nil {
		// Success — mark as sent
		if markErr := w.markSent(ctx, n.ID); markErr != nil {
			w.logger.ErrorContext(ctx, "failed to mark notification as sent", "notification_id", n.ID, "error", markErr)
		}
		w.logger.InfoContext(ctx, "notification delivered successfully",
			"notification_id", n.ID,
			"provider_type", providerType,
			"recipient", redact.MaskRecipient(n.Recipient),
		)
		return
	}

	w.logger.WarnContext(ctx, "notification delivery failed",
		"notification_id", n.ID,
		"provider_type", providerType,
		"retry_count", n.RetryCount,
		"error", redact.WrapError(err),
	)

	// Persist the failed attempt unconditionally so the next poll sees progress.
	if incErr := w.incrementRetryCount(ctx, n.ID); incErr != nil {
		w.logger.ErrorContext(ctx, "failed to increment retry count", "notification_id", n.ID, "error", incErr)
	}

	// MaxRetries is the total attempt budget (the SQL queue admits rows with
	// retry_count < 3, matching the default). Once the budget is spent the row
	// must reach the terminal failed state — otherwise it would linger pending
	// beyond the SQL filter and never be retried nor finalized.
	if n.RetryCount+1 >= w.MaxRetries {
		// The provider error may echo an upstream response body, so the
		// persisted reason is sanitized too: it is read back through the
		// notifications API.
		failMsg := fmt.Sprintf("delivery failed after %d attempts: %s", n.RetryCount+1, redact.SanitizeText(err.Error()))
		if markErr := w.markFailed(ctx, n.ID, failMsg); markErr != nil {
			w.logger.ErrorContext(ctx, "failed to mark notification as failed", "notification_id", n.ID, "error", markErr)
		}
		w.logger.WarnContext(ctx, "notification delivery exhausted retries",
			"notification_id", n.ID,
			"provider_type", providerType,
			"retries", w.MaxRetries,
		)
	}
}

func (w *Worker) calculateRetryDelay(attempt int) time.Duration {
	delay := w.BaseRetryDelay * time.Duration(1<<(attempt-1))
	if delay > workerMaxRetryDelay {
		delay = workerMaxRetryDelay
	}
	return delay
}

// markSent / markFailed 走系统作用域事务：Worker 无租户 GUC，直接写 RLS 表会失败。
func (w *Worker) markSent(ctx context.Context, id string) error {
	return w.withSystemTx(tentxtx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		return tx.Notifications().MarkSent(ctx, id)
	})
}

// incrementRetryCount 走系统作用域事务持久化失败尝试次数（BE-B-9）。
func (w *Worker) incrementRetryCount(ctx context.Context, id string) error {
	return w.withSystemTx(tentxtx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		return tx.Notifications().IncrementRetryCount(ctx, id)
	})
}

func (w *Worker) markFailed(ctx context.Context, id, reason string) error {
	return w.withSystemTx(tentxtx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		return tx.Notifications().MarkFailed(ctx, id, reason)
	})
}
