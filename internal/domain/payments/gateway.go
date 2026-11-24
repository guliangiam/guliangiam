package payments

import "strings"

// Outcome is a payment channel's verdict expressed in this system's own
// vocabulary. Channel status strings (`TRADE_SUCCESS`, `SUCCESS`, `succeeded`,
// …) are facts about a third party; only the domain may decide which lifecycle
// state they imply. Translating a channel status straight into a persisted state
// literal — which the transport layer used to do in four separate places — put
// the state vocabulary in three layers at once and let the same channel answer
// produce different states depending on which surface handled it.
type Outcome int

const (
	// OutcomeUnknown means the channel reported something this system does not
	// recognise. A caller must leave the state machine untouched: advancing a
	// lifecycle on an unparsed third-party string invents a fact.
	OutcomeUnknown Outcome = iota
	// OutcomePending means the trade exists but the buyer has not paid.
	OutcomePending
	// OutcomeSucceeded means the channel confirmed the money moved.
	OutcomeSucceeded
	// OutcomeFailed means the attempt was rejected or expired at the channel.
	OutcomeFailed
	// OutcomeCancelled means the trade was closed without collecting payment.
	OutcomeCancelled
	// OutcomeRefunded means the channel reversed a previously collected payment.
	OutcomeRefunded
)

// tradeStatusTable maps `providerType` → channel status → Outcome, matched
// case-insensitively. It is the only place in the system where third-party
// status strings are understood.
var tradeStatusTable = map[string]map[string]Outcome{
	"alipay": {
		"TRADE_SUCCESS":  OutcomeSucceeded,
		"TRADE_FINISHED": OutcomeSucceeded,
		"TRADE_CLOSED":   OutcomeCancelled,
		"WAIT_BUYER_PAY": OutcomePending,
	},
	"wechat": {
		channelStatusSuccess: OutcomeSucceeded,
		"CLOSED":             OutcomeCancelled,
		"NOTPAY":             OutcomePending,
		"USERPAYING":         OutcomePending,
		"PAYERROR":           OutcomeFailed,
		"REFUND":             OutcomeRefunded,
		"REFUNDING":          OutcomeRefunded,
	},
	"stripe": {
		"PAID":                    OutcomeSucceeded,
		channelStatusSucceeded:    OutcomeSucceeded,
		channelStatusPending:      OutcomePending,
		"PROCESSING":              OutcomePending,
		"CANCELED":                OutcomeCancelled,
		"REQUIRES_PAYMENT_METHOD": OutcomeFailed,
		"REFUNDED":                OutcomeRefunded,
	},
	"paypal": {
		"COMPLETED":          OutcomeSucceeded,
		"APPROVED":           OutcomeSucceeded,
		channelStatusPending: OutcomePending,
		"CREATED":            OutcomePending,
		"VOIDED":             OutcomeCancelled,
		"REFUNDED":           OutcomeRefunded,
		"PARTIALLY_REFUNDED": OutcomeRefunded,
		channelStatusFailed:  OutcomeFailed,
	},
	"adyen": {
		"AUTHORISED":           OutcomeSucceeded,
		"CAPTURED":             OutcomeSucceeded,
		channelStatusPending:   OutcomePending,
		"REFUSED":              OutcomeFailed,
		channelStatusCancelled: OutcomeCancelled,
		"REFUND":               OutcomeRefunded,
	},
	"airwallex": {
		channelStatusSuccess:   OutcomeSucceeded,
		channelStatusPending:   OutcomePending,
		channelStatusFailed:    OutcomeFailed,
		channelStatusCancelled: OutcomeCancelled,
	},
	"balance": {
		channelStatusSucceeded: OutcomeSucceeded,
		channelStatusPending:   OutcomePending,
		channelStatusFailed:    OutcomeFailed,
		channelStatusCancelled: OutcomeCancelled,
	},
	"gc": {
		channelStatusSucceeded: OutcomeSucceeded,
		channelStatusPending:   OutcomePending,
		channelStatusFailed:    OutcomeFailed,
	},
}

// commonTradeStatuses answers for channels whose name the caller did not supply
// (for instance a generic query adapter) or does not know. It only contains
// values that are unambiguous across every channel this system talks to.
var commonTradeStatuses = map[string]Outcome{
	"TRADE_SUCCESS":        OutcomeSucceeded,
	"TRADE_FINISHED":       OutcomeSucceeded,
	channelStatusSuccess:   OutcomeSucceeded,
	"PAID":                 OutcomeSucceeded,
	channelStatusSucceeded: OutcomeSucceeded,
	"TRADE_CLOSED":         OutcomeCancelled,
	"CLOSED":               OutcomeCancelled,
	"CANCELED":             OutcomeCancelled,
	"WAIT_BUYER_PAY":       OutcomePending,
	"NOTPAY":               OutcomePending,
	channelStatusPending:   OutcomePending,
	"PROCESSING":           OutcomePending,
}

// TranslateTradeStatus resolves a channel status into an Outcome. The result is
// never a state literal, so an unrecognised value cannot leak into persistence.
func TranslateTradeStatus(providerType, tradeStatus string) Outcome {
	status := strings.ToUpper(strings.TrimSpace(tradeStatus))
	if status == "" {
		return OutcomeUnknown
	}
	if table, ok := tradeStatusTable[strings.ToLower(strings.TrimSpace(providerType))]; ok {
		if outcome, ok := table[status]; ok {
			return outcome
		}
	}
	if outcome, ok := commonTradeStatuses[status]; ok {
		return outcome
	}
	return OutcomeUnknown
}
