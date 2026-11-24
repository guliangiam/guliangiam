package syncer

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSCIMSyncer(t *testing.T) {
	s := NewSCIMSyncer()
	assert.NotNil(t, s)
	assert.NotNil(t, s.client)
}

func TestParseSCIMFilter(t *testing.T) {
	tests := []struct {
		name      string
		filter    string
		expected  int
		firstAttr string
		firstOp   string
		firstVal  interface{}
	}{
		{"empty", "", 0, "", "", nil},
		{"whitespace", "   ", 0, "", "", nil},
		{"eq_string", `userName eq "john"`, 1, "userName", "eq", "john"},
		{"eq_bool_true", "active eq true", 1, "active", "eq", true},
		{"eq_bool_false", "active eq false", 1, "active", "eq", false},
		{"eq_null", "title eq null", 1, "title", "eq", nil},
		{"eq_number", "age eq 25", 1, "age", "eq", 25},
		{"eq_float", "score eq 3.5", 1, "score", "eq", 3.5},
		{"co_operator", `displayName co "admin"`, 1, "displayName", "co", "admin"},
		{"sw_operator", `userName sw "j"`, 1, "userName", "sw", "j"},
		{"ew_operator", `userName ew "n"`, 1, "userName", "ew", "n"},
		{"gt_operator", "age gt 18", 1, "age", "gt", 18},
		{"lt_operator", "age lt 65", 1, "age", "lt", 65},
		{"ge_operator", "age ge 21", 1, "age", "ge", 21},
		{"le_operator", "age le 65", 1, "age", "le", 65},
		{"ne_operator", `userName ne "admin"`, 1, "userName", "ne", "admin"},
		{"dotted_attr", `emails.value eq "john@example.com"`, 1, "emails.value", "eq", "john@example.com"},
		{"and_conditions", `userName eq "john" and active eq true`, 2, "userName", "eq", "john"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			conditions := ParseSCIMFilter(tc.filter)
			assert.Len(t, conditions, tc.expected)
			if tc.expected > 0 {
				assert.Equal(t, tc.firstAttr, conditions[0].AttributePath)
				assert.Equal(t, tc.firstOp, conditions[0].Operator)
				assert.Equal(t, tc.firstVal, conditions[0].Value)
			}
		})
	}
}

func TestParseSCIMValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{"null", "null", nil},
		{"true", "true", true},
		{"false", "false", false},
		{"quoted_string", `"hello"`, "hello"},
		{"integer", "42", 42},
		{"float", "3.14", 3.14},
		{"unquoted_string", "something", "something"},
		{"whitespace_trimmed", `  "test"  `, "test"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parseSCIMValue(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSplitSCIMFilterAND(t *testing.T) {
	tests := []struct {
		name     string
		filter   string
		expected int
	}{
		{"single", `userName eq "john"`, 1},
		{"double", `userName eq "john" and active eq true`, 2},
		{"triple", `a eq 1 and b eq 2 and c eq 3`, 3},
		{"and_in_quotes", `userName eq "this and that"`, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parts := splitSCIMFilterAND(tc.filter)
			assert.Len(t, parts, tc.expected)
		})
	}
}

func TestSCIMSyncer_ResolveBaseURL(t *testing.T) {
	s := &SCIMSyncer{}

	tests := []struct {
		name     string
		config   *SyncerConfig
		expected string
	}{
		{"api_url", &SyncerConfig{APIURL: "https://scim.example.com/v2"}, "https://scim.example.com/v2"},
		{"api_url_trailing_slash", &SyncerConfig{APIURL: "https://scim.example.com/v2/"}, "https://scim.example.com/v2"},
		{"host_only", &SyncerConfig{Host: "scim.example.com"}, "https://scim.example.com"},
		{"host_with_https", &SyncerConfig{Host: "https://scim.example.com"}, "https://scim.example.com"},
		{"host_trailing_slash", &SyncerConfig{Host: "scim.example.com/"}, "https://scim.example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := s.resolveBaseURL(tc.config)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSCIMSyncer_SetAuthHeaders(t *testing.T) {
	s := &SCIMSyncer{}

	t.Run("bearer_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		config := &SyncerConfig{ClientSecret: "my-token"}
		s.setAuthHeaders(req, config)
		assert.Equal(t, "Bearer my-token", req.Header.Get("Authorization"))
	})

	t.Run("basic_auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		config := &SyncerConfig{ClientID: "user", Password: "pass"}
		s.setAuthHeaders(req, config)
		user, pass, ok := req.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "user", user)
		assert.Equal(t, "pass", pass)
	})

	t.Run("no_auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		config := &SyncerConfig{}
		s.setAuthHeaders(req, config)
		assert.Empty(t, req.Header.Get("Authorization"))
	})

	t.Run("bearer_takes_precedence", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		config := &SyncerConfig{ClientSecret: "token", ClientID: "user", Password: "pass"}
		s.setAuthHeaders(req, config)
		assert.Equal(t, "Bearer token", req.Header.Get("Authorization"))
	})
}

func TestSCIMSyncer_SyncUsers_MissingURL(t *testing.T) {
	s := NewSCIMSyncer()
	_, err := s.SyncUsers(t.Context(), &SyncerConfig{})
	assert.Error(t, err)
}

func TestSCIMSyncer_SyncGroups_MissingURL(t *testing.T) {
	s := NewSCIMSyncer()
	_, err := s.SyncGroups(t.Context(), &SyncerConfig{})
	assert.Error(t, err)
}

func TestSCIMSyncer_TestConnection_MissingURL(t *testing.T) {
	s := NewSCIMSyncer()
	err := s.TestConnection(t.Context(), &SyncerConfig{})
	assert.Error(t, err)
}
