package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	domaingateway "github.com/guliangiam/guliangiam/internal/domain/gateway"
)

// Proxy is an MCP (Model Context Protocol) proxy server.
type Proxy struct {
	mu       sync.RWMutex
	servers  map[string]*MCPServerConfig
	client   *http.Client
	tokenURL string // OAuth2 token endpoint for MCP server authentication
}

// MCPServerConfig holds configuration for a proxied MCP server.
type MCPServerConfig struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Endpoint    string            `json:"endpoint"`
	AccessToken string            `json:"accessToken"`
	Headers     map[string]string `json:"headers"`
	Tools       []Tool            `json:"tools"`
	// OAuth2 configuration for token acquisition
	TokenURL     string `json:"tokenUrl,omitempty"`
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	Scopes       string `json:"scopes,omitempty"`
}

type Tool = domaingateway.MCPTool

// NewProxy creates a new MCP proxy.
func NewProxy() *Proxy {
	return &Proxy{
		servers: make(map[string]*MCPServerConfig),
		client:  ssrf.SafeHTTPClient(30 * time.Second),
	}
}

// NewProxyWithTokenURL creates a new MCP proxy with an OAuth2 token endpoint.
func NewProxyWithTokenURL(tokenURL string) *Proxy {
	return &Proxy{
		servers:  make(map[string]*MCPServerConfig),
		client:   ssrf.SafeHTTPClient(30 * time.Second),
		tokenURL: tokenURL,
	}
}

// RegisterServer registers an MCP server with the proxy.
func (p *Proxy) RegisterServer(config *MCPServerConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.servers[config.ID] = config
}

// SetHTTPClient overrides the HTTP client used for MCP requests.
// Useful for tests that need to bypass SSRF protection or inject
// a httptest server-backed client.
func (p *Proxy) SetHTTPClient(c *http.Client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.client = c
}

// RemoveServer removes an MCP server from the proxy.
func (p *Proxy) RemoveServer(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.servers, id)
}

// GetServers returns all registered MCP servers.
func (p *Proxy) GetServers() []*MCPServerConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	servers := make([]*MCPServerConfig, 0, len(p.servers))
	for _, s := range p.servers {
		servers = append(servers, s)
	}
	return servers
}

// SyncToolsFromServer fetches and caches the tool list from a specific MCP server.
// This implements "tool sync" - keeping the proxy's tool list up-to-date with backend servers.
func (p *Proxy) SyncToolsFromServer(ctx context.Context, serverID string) error {
	p.mu.RLock()
	server, ok := p.servers[serverID]
	p.mu.RUnlock()

	if !ok {
		return apperrors.NewNotFoundf("mcp server not found: %s", serverID)
	}

	// SSRF 前置校验：server.Endpoint 来自用户注册
	if err := ssrf.ValidateURL(server.Endpoint); err != nil {
		return apperrors.WrapInvalidArgumentf(err, "mcp server %s endpoint rejected", serverID)
	}

	tools, err := p.fetchTools(ctx, server)
	if err != nil {
		return apperrors.WrapInternalf(err, serverID, "mcp sync tools from %s")
	}

	p.mu.Lock()
	server.Tools = tools
	p.mu.Unlock()

	return nil
}

// SyncAllTools fetches and caches the tool lists from all registered MCP servers.
func (p *Proxy) SyncAllTools(ctx context.Context) error {
	p.mu.RLock()
	servers := make([]*MCPServerConfig, 0, len(p.servers))
	for _, s := range p.servers {
		servers = append(servers, s)
	}
	p.mu.RUnlock()

	var errs []string
	for _, server := range servers {
		if err := p.SyncToolsFromServer(ctx, server.ID); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return apperrors.NewInternalf("mcp sync all tools: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ListTools lists all tools from all registered MCP servers.
func (p *Proxy) ListTools(ctx context.Context) ([]Tool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var allTools []Tool
	for _, server := range p.servers {
		// Use cached tools if available, otherwise fetch
		if len(server.Tools) > 0 {
			allTools = append(allTools, server.Tools...)
			continue
		}

		tools, err := p.fetchTools(ctx, server)
		if err != nil {
			continue
		}
		allTools = append(allTools, tools...)
	}
	return allTools, nil
}

// CallTool calls a tool on the appropriate MCP server.
// Per MCP spec: sends tools/call JSON-RPC payload, validates HTTP 2xx,
// decodes response, and surfaces MCP error objects as actionable errors.
func (p *Proxy) CallTool(ctx context.Context, serverID, toolName string, arguments map[string]interface{}) (interface{}, error) {
	p.mu.RLock()
	server, ok := p.servers[serverID]
	p.mu.RUnlock()

	if !ok {
		return nil, apperrors.NewNotFoundf("mcp server not found: %s", serverID)
	}

	// SSRF 前置校验：server.Endpoint 来自用户注册
	if err := ssrf.ValidateURL(server.Endpoint); err != nil {
		return nil, apperrors.WrapInvalidArgumentf(err, "mcp server %s endpoint rejected", serverID)
	}

	// Get access token if needed
	accessToken, err := p.getAccessToken(ctx, server)
	if err == nil && accessToken != "" {
		server = p.serverWithToken(server, accessToken)
	}

	payload := map[string]interface{}{
		"method": "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": arguments,
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "mcp marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.Endpoint+"/mcp", strings.NewReader(string(jsonPayload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if server.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+server.AccessToken)
	}
	for k, v := range server.Headers {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "mcp call tool %s/%s", serverID, toolName)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "mcp read response for %s/%s", serverID, toolName)
	}

	// Non-2xx → actionable upstream error with status + body snippet
	if resp.StatusCode >= 400 {
		snippet := truncateBody(body, 256)
		return nil, apperrors.NewInternalf(
			"mcp call tool %s/%s: upstream returned %d %s — %s",
			serverID, toolName, resp.StatusCode, resp.Status, snippet,
		)
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		snippet := truncateBody(body, 256)
		return nil, apperrors.WrapInternalf(err, "mcp decode response for %s/%s: %s", serverID, toolName, snippet)
	}

	// MCP error shape: {"error": {"code": int, "message": string}}
	if errInfo, ok := extractMCPError(result); ok {
		return nil, apperrors.NewInternalf(
			"mcp call tool %s/%s: %s", serverID, toolName, errInfo,
		)
	}

	return result, nil
}

// truncateBody returns the first n bytes of body as a string, or the whole body
// if shorter. Used in error messages to keep log volume bounded.
func truncateBody(body []byte, n int) string {
	if len(body) <= n {
		return string(body)
	}
	return string(body[:n]) + "..."
}

// extractMCPError checks if the decoded JSON response is an MCP error object
// {"error": {"code": ..., "message": ..., "data": ...}}. Returns (summary, true)
// when found, (empty, false) otherwise.
func extractMCPError(result interface{}) (string, bool) {
	obj, ok := result.(map[string]interface{})
	if !ok {
		return "", false
	}
	errVal, ok := obj["error"]
	if !ok {
		return "", false
	}
	errObj, ok := errVal.(map[string]interface{})
	if !ok {
		return "", false
	}
	msg, _ := errObj["message"].(string)
	code := errObj["code"]
	if msg == "" && code == nil {
		return "", false
	}
	return fmt.Sprintf("code=%v message=%s", code, msg), true
}

// getAccessToken obtains an access token for an MCP server using OAuth2 client credentials.
func (p *Proxy) getAccessToken(ctx context.Context, server *MCPServerConfig) (string, error) {
	// If the server already has an access token, use it
	if server.AccessToken != "" {
		return server.AccessToken, nil
	}

	// If the server has OAuth2 configuration, obtain a token
	tokenURL := server.TokenURL
	if tokenURL == "" {
		tokenURL = p.tokenURL
	}

	if tokenURL == "" || server.ClientID == "" || server.ClientSecret == "" {
		return "", nil // No OAuth2 config, skip token acquisition
	}

	// SSRF 校验 tokenURL
	if err := ssrf.ValidateURL(tokenURL); err != nil {
		return "", apperrors.WrapInvalidArgumentf(err, "mcp tokenURL rejected")
	}

	payload := fmt.Sprintf("grant_type=client_credentials&client_id=%s&client_secret=%s",
		server.ClientID, server.ClientSecret)
	if server.Scopes != "" {
		payload += "&scope=" + server.Scopes
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(payload))
	if err != nil {
		return "", apperrors.WrapInternalf(err, "mcp create token request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "mcp get access token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "mcp read token response")
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", apperrors.WrapInternalf(err, "mcp parse token response")
	}

	return tokenResp.AccessToken, nil
}

// serverWithToken returns a copy of the server config with the given access token.
func (p *Proxy) serverWithToken(server *MCPServerConfig, token string) *MCPServerConfig {
	cp := *server
	cp.AccessToken = token
	return &cp
}

func (p *Proxy) fetchTools(ctx context.Context, server *MCPServerConfig) ([]Tool, error) {
	payload := map[string]interface{}{
		"method": "tools/list",
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.Endpoint+"/mcp", bytes.NewReader(jsonPayload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if server.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+server.AccessToken)
	}
	for k, v := range server.Headers {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Tools, nil
}

// Ensure Proxy compiles
var _ = time.Second

var _ domaingateway.MCPProxy = (*Proxy)(nil)
