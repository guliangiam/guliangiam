package aiupstream

import (
	"context"
	"errors"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scriptedReadCloser returns its payload once, then the configured read error.
// It simulates an upstream connection that breaks mid-stream.
type scriptedReadCloser struct {
	data []byte
	err  error
}

func (r *scriptedReadCloser) Read(p []byte) (int, error) {
	if len(r.data) > 0 {
		n := copy(p, r.data)
		r.data = r.data[n:]
		return n, nil
	}
	return 0, r.err
}

func (r *scriptedReadCloser) Close() error { return nil }

func drainStream(ch <-chan *domain.CanonicalStreamEvent) []*domain.CanonicalStreamEvent {
	events := make([]*domain.CanonicalStreamEvent, 0, 8)
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}

func requireStreamError(t *testing.T, events []*domain.CanonicalStreamEvent, wantMessage string) {
	t.Helper()
	require.NotEmpty(t, events)
	last := events[len(events)-1]
	require.NotNil(t, last, "terminal stream event must not be nil")
	assert.Equal(t, domain.StreamEventError, last.Type)
	require.NotNil(t, last.Error, "error event must carry the structured payload")
	assert.Equal(t, "unavailable", last.Error.Code)
	assert.Equal(t, wantMessage, last.Error.Message)
	assert.NotContains(t, last.Error.Message, "secret", "raw upstream text must not reach the client")
}

func TestOpenAICompatConsumeSSE_ReadErrorEmitsTerminalEvent(t *testing.T) {
	body := &scriptedReadCloser{
		data: []byte(`data: {"choices":[{"delta":{"content":"hi"}}]}` + "\n\n"),
		err:  errors.New("secret read failure"),
	}
	ch := make(chan *domain.CanonicalStreamEvent, 4)
	(&OpenAICompatAdapter{}).consumeSSE(context.Background(), body, ch)

	events := drainStream(ch)
	require.Len(t, events, 2, "content event followed by the terminal error event")
	assert.Equal(t, "content_delta", events[0].Type)
	requireStreamError(t, events, streamReadErrorMessage)
}

func TestOpenAICompatConsumeSSE_UpstreamErrorFrameIsSanitized(t *testing.T) {
	body := &scriptedReadCloser{
		data: []byte(`data: {"error":{"message":"secret upstream body","type":"server_error"}}` + "\n\n"),
		err:  errors.New("eof"),
	}
	ch := make(chan *domain.CanonicalStreamEvent, 4)
	(&OpenAICompatAdapter{}).consumeSSE(context.Background(), body, ch)

	events := drainStream(ch)
	requireStreamError(t, events, streamReportedErrorMessage)
}

func TestAnthropicConsumeSSE_ErrorEventIsSanitized(t *testing.T) {
	body := &scriptedReadCloser{
		data: []byte("event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"secret upstream body\"}}\n\n"),
		err:  errors.New("eof"),
	}
	ch := make(chan *domain.CanonicalStreamEvent, 4)
	(&AnthropicMessagesAdapter{}).consumeAnthropicSSE(context.Background(), body, ch)

	events := drainStream(ch)
	requireStreamError(t, events, streamReportedErrorMessage)
}
