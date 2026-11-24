package webhooks

import (
	"testing"

	webhookdomain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
	infrawebhook "github.com/guliangiam/guliangiam/internal/infra/webhook"
)

func TestHTTPDeliverySenderImplementsDeliverySender(t *testing.T) {
	var _ webhookdomain.DeliverySender = (*infrawebhook.HTTPDeliverySender)(nil)
}
