package verifications

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// sendRecipientPeekLimit bounds how much of the send body the rate-limit key
// function inspects. Send payloads are tiny; anything larger falls back to
// the IP-keyed bucket instead of being buffered.
const sendRecipientPeekLimit = 64 << 10

// sendRecipientRateLimitKey derives the recipient dimension for the send
// endpoint's stacked rate limiter (BE-A-04). It peeks the JSON body without
// consuming it — the handler still decodes the exact same bytes afterwards.
// An unparseable or oversized body returns "" so the middleware falls back to
// the trusted client IP subject.
func sendRecipientRateLimitKey(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	buf, err := io.ReadAll(io.LimitReader(r.Body, sendRecipientPeekLimit+1))
	// Restore the body regardless of read outcome: whatever was read is
	// replayed first, the unread remainder stays attached.
	r.Body = &replayedBody{
		Reader: io.MultiReader(bytes.NewReader(buf), r.Body),
		Closer: r.Body,
	}
	if err != nil || len(buf) > sendRecipientPeekLimit {
		return ""
	}
	var payload struct {
		Type      string `json:"type"`
		Recipient string `json:"recipient"`
	}
	if json.Unmarshal(buf, &payload) != nil {
		return ""
	}
	recipient := strings.ToLower(strings.TrimSpace(payload.Recipient))
	if recipient == "" {
		return ""
	}
	return "verification-send:" + payload.Type + ":" + recipient
}

type replayedBody struct {
	io.Reader
	io.Closer
}
