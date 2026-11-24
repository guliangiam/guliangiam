package mcpservers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/redact"
	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// Hard limits for sync-intranet (E-01). The endpoint expands CIDRs × ports into
// live TCP probes and auto-registers every hit, so an unbounded request is a
// resource-amplification primitive. Values follow the E1 audit recommendation:
// eight ranges, a /16 per range, 64 ports, 100 probes in flight and 65536
// targets overall. They intentionally are constants, not configuration: raising
// them requires a review of the memory/goroutine ceiling, not an env tweak.
const (
	maxIntranetCIDRs        = 8
	maxIntranetHostsPerCIDR = 1 << 16 // = /16 (65536 addresses, 65534 scanned)
	maxIntranetPorts        = 64
	maxIntranetConcurrency  = 100
	maxIntranetTargets      = 1 << 16
	minIntranetPort         = 1
	maxIntranetPort         = 65535
	maxIntranetCIDRHostBits = 16
)

// SyncIntranetServersRequest defines the CIDR and port ranges to scan.
type SyncIntranetServersRequest struct {
	OrganizationID string
	CIDRs          []string // e.g. ["192.168.1.0/24"]
	Ports          []int    // e.g. [8080, 3000]
	Concurrency    int      // max parallel probes (default 50)
}

// SyncIntranetServersResult contains the discovered servers.
type SyncIntranetServersResult struct {
	Discovered []string // URLs of discovered MCP servers
	Errors     []string // scan errors encountered
}

// SyncIntranetServers scans an intranet CIDR range for MCP servers.
// It performs TCP connect probes and attempts to identify MCP endpoints.
func (s Service) SyncIntranetServers(ctx context.Context, req SyncIntranetServersRequest) (*SyncIntranetServersResult, error) {
	if err := validateIntranetScanRequest(req); err != nil {
		return nil, err
	}
	if len(req.CIDRs) == 0 {
		return &SyncIntranetServersResult{}, nil
	}
	if req.Concurrency <= 0 {
		req.Concurrency = 50
	}

	type probeResult struct {
		addr string
		open bool
	}

	var targets []string
	for _, cidr := range req.CIDRs {
		ips, err := expandCIDR(cidr)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			for _, port := range req.Ports {
				targets = append(targets, fmt.Sprintf("%s:%d", ip, port))
			}
		}
	}

	// Probe targets with limited concurrency
	sem := make(chan struct{}, req.Concurrency)
	results := make(chan probeResult, len(targets))
	var wg sync.WaitGroup

	dialer := &net.Dialer{Timeout: 3 * time.Second}
	for _, target := range targets {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			conn, err := dialer.DialContext(ctx, "tcp", addr)
			if err != nil {
				results <- probeResult{addr: addr, open: false}
				return
			}
			conn.Close()
			results <- probeResult{addr: addr, open: true}
		}(target)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var discovered []string
	var errors []string
	for r := range results {
		if r.open {
			discovered = append(discovered, "http://"+r.addr)
		}
	}

	// Auto-register discovered servers
	for _, url := range discovered {
		id, err := s.UUID.NewString()
		if err != nil {
			errors = append(errors, redact.SanitizeText(fmt.Sprintf("failed to generate UUID for %s: %v", url, err)))
			continue
		}
		now := s.Clock.Now()
		server := &domain.McpServer{
			ID:             id,
			OrganizationID: req.OrganizationID,
			Name:           fmt.Sprintf("intranet-%s", id[:8]),
			DisplayName:    fmt.Sprintf("Intranet Server %s", url),
			URL:            url,
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		if err := s.UOW.McpServers().Create(ctx, server); err != nil {
			errors = append(errors, redact.SanitizeText(fmt.Sprintf("failed to register %s: %v", url, err)))
		}
	}

	return &SyncIntranetServersResult{
		Discovered: discovered,
		Errors:     errors,
	}, nil
}

// validateIntranetScanRequest rejects a scan whose declared surface exceeds the
// hard limits before any IP list, target slice or goroutine is allocated.
func validateIntranetScanRequest(req SyncIntranetServersRequest) error {
	if len(req.CIDRs) > maxIntranetCIDRs {
		return apperrors.NewUnprocessableEntityf(
			"too many CIDRs: %d, maximum %d", len(req.CIDRs), maxIntranetCIDRs)
	}
	if len(req.Ports) > maxIntranetPorts {
		return apperrors.NewUnprocessableEntityf(
			"too many ports: %d, maximum %d", len(req.Ports), maxIntranetPorts)
	}
	if req.Concurrency > maxIntranetConcurrency {
		return apperrors.NewUnprocessableEntityf(
			"concurrency %d exceeds maximum %d", req.Concurrency, maxIntranetConcurrency)
	}
	for _, port := range req.Ports {
		if port < minIntranetPort || port > maxIntranetPort {
			return apperrors.NewInvalidArgumentf(
				"port %d is outside the range %d-%d", port, minIntranetPort, maxIntranetPort)
		}
	}

	totalTargets := 0
	for _, cidr := range req.CIDRs {
		hosts, err := cidrHostCount(cidr)
		if err != nil {
			return err
		}
		totalTargets += hosts * len(req.Ports)
		if totalTargets > maxIntranetTargets {
			return apperrors.NewUnprocessableEntityf(
				"scan target count %d exceeds maximum %d", totalTargets, maxIntranetTargets)
		}
	}
	return nil
}

// cidrHostCount returns the number of probes a single CIDR contributes after
// trimming network/broadcast addresses. It rejects ranges wider than /16 so one
// entry cannot expand to more than maxIntranetHostsPerCIDR addresses.
func cidrHostCount(cidr string) (int, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return 0, apperrors.NewInvalidArgumentf("invalid CIDR %q", cidr)
	}
	ones, bits := ipNet.Mask.Size()
	hostBits := bits - ones
	if hostBits > maxIntranetCIDRHostBits {
		return 0, apperrors.NewUnprocessableEntityf(
			"CIDR %q spans more than %d addresses (/16 maximum)", cidr, maxIntranetHostsPerCIDR)
	}
	hosts := 1 << uint(hostBits)
	if hosts > 2 {
		hosts -= 2 // network + broadcast
	}
	return hosts, nil
}

// expandCIDR expands a CIDR notation to individual IPs (limited to /16 = max 65536 IPs).
func expandCIDR(cidr string) ([]string, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, apperrors.NewInvalidArgumentf("invalid CIDR %q", cidr)
	}

	// Limit scan scope
	if _, err := cidrHostCount(cidr); err != nil {
		return nil, err
	}

	var ips []string
	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); incIP(ip) {
		ips = append(ips, ip.String())
	}

	// Skip network and broadcast addresses for /24 and smaller
	if len(ips) > 2 {
		ips = ips[1 : len(ips)-1]
	}

	return ips, nil
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// IsMcpEndpoint probes the given URL to check if it responds as an MCP server.
// 这是内网扫描的合法场景，因此 allowlist 放行私有网段，但仍阻止 metadata（169.254.169.0/24）。
func IsMcpEndpoint(ctx context.Context, url string) bool {
	if err := ssrf.ValidateURL(url); err != nil {
		return false
	}
	// 合法内网扫描：放行 RFC1918 + loopback + ULA，注册于 ssrf allowlist 注册表。
	// 不放 metadata 段（registry.go 审查红线明确禁止）。
	client := ssrf.NamedHTTPClient("mcpservers.intranet_scan", 5*time.Second)
	// MCP servers typically respond to GET /mcp or POST /mcp
	probeURL := strings.TrimRight(url, "/") + "/mcp"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, http.NoBody)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}
