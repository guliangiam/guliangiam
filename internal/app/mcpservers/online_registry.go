package mcpservers

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/redact"
	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// OnlineServer represents an MCP server from a remote registry.
type OnlineServer struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Url         string `json:"url"`
	Category    string `json:"category"`
}

// GetOnlineServers fetches available MCP servers from a remote JSON registry.
// The registry URL is configurable; if empty, returns an empty list.
func (s Service) GetOnlineServers(ctx context.Context, registryURL string) ([]OnlineServer, error) {
	if registryURL == "" {
		return []OnlineServer{}, nil
	}

	// Check cache first
	cached := onlineRegistryCache.get(registryURL)
	if cached != nil {
		return cached.servers, nil
	}

	// Fetch from remote — registryURL 可能来自外部配置，走 SSRF 防护
	if err := ssrf.ValidateURL(registryURL); err != nil {
		// Fail open：返回空列表
		return []OnlineServer{}, nil
	}
	client := ssrf.SafeHTTPClient(10 * time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryURL, http.NoBody)
	if err != nil {
		// The *url.Error text includes the caller-supplied registry URL (which
		// may carry userinfo or a token query). Keep the chain, sanitize the text.
		return nil, redact.WrapError(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		// Fail open: return empty list if remote registry is unreachable
		return []OnlineServer{}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return []OnlineServer{}, nil
	}

	var servers []OnlineServer
	if err := json.NewDecoder(resp.Body).Decode(&servers); err != nil {
		return []OnlineServer{}, nil
	}

	onlineRegistryCache.set(registryURL, servers)
	return servers, nil
}

// onlineRegistryCache caches the remote registry for 5 minutes.
var onlineRegistryCache = &registryCache{
	ttl: 5 * time.Minute,
}

type registryCacheEntry struct {
	servers []OnlineServer
	fetched time.Time
}

type registryCache struct {
	mu      sync.RWMutex
	entries map[string]*registryCacheEntry
	ttl     time.Duration
}

func (c *registryCache) get(url string) *registryCacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.entries == nil {
		return nil
	}
	entry, ok := c.entries[url]
	if !ok {
		return nil
	}
	if time.Since(entry.fetched) > c.ttl {
		return nil
	}
	return entry
}

func (c *registryCache) set(url string, servers []OnlineServer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[string]*registryCacheEntry)
	}
	c.entries[url] = &registryCacheEntry{servers: servers, fetched: time.Now()}
}
