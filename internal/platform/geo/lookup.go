package geo

import (
	"fmt"
	"log/slog"
	"net"
	"strings"

	ip2regionservice "github.com/lionsoul2014/ip2region/binding/golang/service"
)

// LookupResult contains the geolocation information for an IP address.
type LookupResult struct {
	Country  string `json:"country"`
	Region   string `json:"region"`
	Province string `json:"province"`
	City     string `json:"city"`
	ISP      string `json:"isp"`
}

// LookupService provides IP geolocation lookup.
type LookupService struct {
	region *ip2regionservice.Ip2Region
	logger *slog.Logger
}

// NewLookupService creates a new geo lookup service using ip2region.
// If v4XdbPath is empty, IPv4 lookup will be disabled.
// If the database file cannot be loaded, the service operates in degraded mode
// (all lookups return empty results).
func NewLookupService(v4XdbPath string, logger *slog.Logger) *LookupService {
	s := &LookupService{logger: logger}

	if v4XdbPath == "" {
		if logger != nil {
			logger.Info("geo: ip2region disabled, no xdb path configured")
		}
		return s
	}

	region, err := ip2regionservice.NewIp2RegionWithPath(v4XdbPath, "")
	if err != nil {
		if logger != nil {
			logger.Error("geo: failed to load ip2region database, running in degraded mode",
				"xdb_path", v4XdbPath,
				"error", err,
			)
		}
		return s
	}

	s.region = region
	if logger != nil {
		logger.Info("geo: ip2region initialized", "xdb_path", v4XdbPath)
	}
	return s
}

// Close releases resources held by the lookup service.
func (s *LookupService) Close() {
	if s.region != nil {
		s.region.Close()
	}
}

// Lookup returns geolocation information for the given IP address.
// Returns nil if the IP is private/reserved or lookup is disabled.
func (s *LookupService) Lookup(ip string) *LookupResult {
	if s.region == nil {
		return nil
	}

	// Skip private/reserved IP addresses
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil
	}
	if parsedIP.IsPrivate() || parsedIP.IsLoopback() || parsedIP.IsUnspecified() {
		return nil
	}

	result, err := s.region.Search(ip)
	if err != nil {
		if s.logger != nil {
			s.logger.Debug("geo: ip2region lookup failed",
				"ip", ip,
				"error", err,
			)
		}
		return nil
	}

	return parseRegionResult(result)
}

// LookupCountry returns only the country code/name for the given IP.
// Returns empty string if lookup fails or is disabled.
func (s *LookupService) LookupCountry(ip string) string {
	result := s.Lookup(ip)
	if result == nil {
		return ""
	}
	return result.Country
}

// parseRegionResult parses the ip2region search result string.
// ip2region returns results in the format: "Country|Region|Province|City|ISP"
func parseRegionResult(result string) *LookupResult {
	if result == "" {
		return nil
	}

	parts := strings.Split(result, "|")
	// Pad to 5 fields
	for len(parts) < 5 {
		parts = append(parts, "")
	}

	return &LookupResult{
		Country:  cleanField(parts[0]),
		Region:   cleanField(parts[1]),
		Province: cleanField(parts[2]),
		City:     cleanField(parts[3]),
		ISP:      cleanField(parts[4]),
	}
}

// cleanField replaces the "0" placeholder that ip2region uses for empty fields.
func cleanField(s string) string {
	s = strings.TrimSpace(s)
	if s == "0" {
		return ""
	}
	return s
}

// FormatLocation returns a human-readable location string.
func FormatLocation(r *LookupResult) string {
	if r == nil {
		return ""
	}
	var parts []string
	if r.Country != "" {
		parts = append(parts, r.Country)
	}
	if r.Province != "" && r.Province != r.Country {
		parts = append(parts, r.Province)
	}
	if r.City != "" && r.City != r.Province && r.City != r.Country {
		parts = append(parts, r.City)
	}
	return strings.Join(parts, " ")
}

// IsNewCountry checks if the IP's country differs from any of the known countries.
func IsNewCountry(geo *LookupService, ip string, knownCountries []string) bool {
	country := geo.LookupCountry(ip)
	if country == "" {
		return false
	}
	for _, c := range knownCountries {
		if strings.EqualFold(c, country) {
			return false
		}
	}
	return true
}

// ExtractCountriesFromIPs returns a deduplicated list of countries from the given IPs.
func ExtractCountriesFromIPs(geo *LookupService, ips []string) []string {
	seen := make(map[string]bool)
	var countries []string
	for _, ip := range ips {
		country := geo.LookupCountry(ip)
		if country != "" && !seen[country] {
			seen[country] = true
			countries = append(countries, country)
		}
	}
	return countries
}

// Ensure LookupService has a String method for logging.
func (s *LookupService) String() string {
	if s.region == nil {
		return "LookupService(disabled)"
	}
	return "LookupService(active)"
}

// Ensure the package compiles with the ip2region import.
var _ = fmt.Sprintf
