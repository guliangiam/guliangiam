package mcpservers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRegistryCache_Get_Empty(t *testing.T) {
	cache := &registryCache{ttl: 5 * time.Minute}
	assert.Nil(t, cache.get("http://example.com"))
}

func TestRegistryCache_SetAndGet(t *testing.T) {
	cache := &registryCache{ttl: 5 * time.Minute}
	servers := []OnlineServer{{Name: "test", Url: "http://test.com"}}
	cache.set("http://example.com", servers)

	entry := cache.get("http://example.com")
	assert.NotNil(t, entry)
	assert.Len(t, entry.servers, 1)
	assert.Equal(t, "test", entry.servers[0].Name)
}

func TestRegistryCache_Get_Miss(t *testing.T) {
	cache := &registryCache{ttl: 5 * time.Minute}
	cache.set("http://example.com", []OnlineServer{})
	assert.Nil(t, cache.get("http://other.com"))
}

func TestRegistryCache_Get_Expired(t *testing.T) {
	cache := &registryCache{ttl: 1 * time.Nanosecond}
	cache.set("http://example.com", []OnlineServer{{Name: "test"}})
	// Wait for TTL to expire
	time.Sleep(1 * time.Millisecond)
	assert.Nil(t, cache.get("http://example.com"))
}

func TestRegistryCache_Set_Overwrite(t *testing.T) {
	cache := &registryCache{ttl: 5 * time.Minute}
	cache.set("http://example.com", []OnlineServer{{Name: "first"}})
	cache.set("http://example.com", []OnlineServer{{Name: "second"}})

	entry := cache.get("http://example.com")
	assert.NotNil(t, entry)
	assert.Equal(t, "second", entry.servers[0].Name)
}

func TestGetOnlineServers_EmptyURL(t *testing.T) {
	svc := Service{}
	servers, err := svc.GetOnlineServers(context.Background(), "")
	assert.NoError(t, err)
	assert.Empty(t, servers)
}

func TestOnlineServer_Fields(t *testing.T) {
	s := OnlineServer{Name: "test", Description: "desc", Url: "http://test.com", Category: "cat"}
	assert.Equal(t, "test", s.Name)
	assert.Equal(t, "desc", s.Description)
	assert.Equal(t, "http://test.com", s.Url)
	assert.Equal(t, "cat", s.Category)
}

func TestSyncIntranetServersRequest_Fields(t *testing.T) {
	req := SyncIntranetServersRequest{
		OrganizationID: "org1",
		CIDRs:          []string{"192.168.1.0/24"},
		Ports:          []int{8080},
		Concurrency:    50,
	}
	assert.Equal(t, "org1", req.OrganizationID)
	assert.Len(t, req.CIDRs, 1)
	assert.Len(t, req.Ports, 1)
	assert.Equal(t, 50, req.Concurrency)
}

func TestSyncIntranetServersResult_Fields(t *testing.T) {
	r := SyncIntranetServersResult{
		Discovered: []string{"http://192.168.1.1:8080"},
		Errors:     []string{},
	}
	assert.Len(t, r.Discovered, 1)
	assert.Empty(t, r.Errors)
}
