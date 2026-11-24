package webhooks

import (
	"encoding/json"
	"time"
)

type ListWebhooksQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	IsEnabled               *bool
	Method                  string
}
type CreateWebhookCommand struct {
	OrganizationID, Name, DisplayName, URL, Method, Headers string
	Events                                                  []string
	IsEnabled                                               *bool
	RetryCount, TimeoutMs                                   int
	Organization, ContentType, CustomBody, CustomHeaders    string
	CertID, ErrorText                                       string
	IsError                                                 bool
	TokenFields, ObjectFields                               []string
	IsUserExtended, SingleOrgOnly                           bool
	MaxRetries, RetryInterval                               int
	UseExponentialBackoff                                   bool
}
type PatchWebhookCommand struct {
	ID                       string
	DisplayName, URL, Method *string
	Headers                  *string
	// Secret follows the repository PATCH convention: nil keeps the stored
	// signing secret, a non-nil value replaces it (empty string clears it).
	// Read endpoints never return the plaintext, so the form must omit the
	// field unless the operator typed a new secret.
	Secret                *string
	Events                []string
	IsEnabled             *bool
	RetryCount, TimeoutMs *int
	Organization          *string
	ContentType           *string
	CustomBody            *string
	CustomHeaders         *string
	CertID                *string
	IsError               *bool
	ErrorText             *string
	TokenFields           []string
	ObjectFields          []string
	IsUserExtended        *bool
	SingleOrgOnly         *bool
	MaxRetries            *int
	RetryInterval         *int
	UseExponentialBackoff *bool
	Revision              int64
}

type WebhookListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	URL            string    `json:"url"`
	Method         string    `json:"method"`
	IsEnabled      bool      `json:"isEnabled"`
	Organization   string    `json:"organization"`
	ContentType    string    `json:"contentType"`
	CertID         string    `json:"certId"`
	IsError        bool      `json:"isError"`
	SingleOrgOnly  bool      `json:"singleOrgOnly"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
type WebhookDetail struct {
	WebhookListItem
	Events                []string `json:"events"`
	Headers               string   `json:"headers"`
	RetryCount            int      `json:"retryCount"`
	TimeoutMs             int      `json:"timeoutMs"`
	CustomBody            string   `json:"customBody"`
	CustomHeaders         string   `json:"customHeaders"`
	ErrorText             string   `json:"errorText"`
	TokenFields           []string `json:"tokenFields"`
	ObjectFields          []string `json:"objectFields"`
	IsUserExtended        bool     `json:"isUserExtended"`
	MaxRetries            int      `json:"maxRetries"`
	RetryInterval         int      `json:"retryInterval"`
	UseExponentialBackoff bool     `json:"useExponentialBackoff"`
	// SecretSet / SecretMasked are the read-only projection of the signing
	// secret (XxxSet + XxxMasked convention). The plaintext is returned exactly
	// once, by the create response (secretPlain), never by a read.
	SecretSet    bool   `json:"secretSet"`
	SecretMasked string `json:"secretMasked,omitempty"`
	Revision     int64  `json:"revision"`
}

type WebhookEventListItem struct {
	ID           string    `json:"id"`
	WebhookID    string    `json:"webhookId"`
	EventType    string    `json:"eventType"`
	Status       string    `json:"status"`
	RetryCount   int       `json:"retryCount"`
	ResponseCode int       `json:"responseCode,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type WebhookEventDetail struct {
	WebhookEventListItem
	Payload      json.RawMessage `json:"payload,omitempty"`
	ResponseBody string          `json:"responseBody,omitempty"`
}
