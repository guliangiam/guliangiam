package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	"github.com/guliangiam/guliangiam/internal/platform/geo"
)

// AnomalyReport describes the results of anomalous login detection.
type AnomalyReport struct {
	IsAnomalous bool     `json:"isAnomalous"`
	Reasons     []string `json:"reasons,omitempty"`
	NewIP       bool     `json:"newIp"`
	NewCountry  bool     `json:"newCountry"`
	UnusualTime bool     `json:"unusualTime"`
	Country     string   `json:"country,omitempty"`
	City        string   `json:"city,omitempty"`
}

// DetectAnomalousLogin detects unusual login patterns for the given user and IP.
// It checks if the IP address has been seen before and if the country differs
// from previously known countries using ip2region geolocation.
func (s *Service) DetectAnomalousLogin(ctx context.Context, userID, ip string) (*AnomalyReport, error) {
	report := &AnomalyReport{}

	ip = strings.TrimSpace(ip)
	if ip == "" {
		return report, nil
	}

	// Enrich report with geolocation if available
	if s.GeoLookup != nil {
		if result := s.GeoLookup.Lookup(ip); result != nil {
			report.Country = result.Country
			report.City = result.City
		}
	}

	// Query existing sessions for this user to find known IPs.
	sessions, _, err := s.UOW.Sessions().List(ctx, sessiondomain.ListQuery{
		UserID:     userID,
		PageSize:   50,
		ActiveOnly: boolPtr(false),
	})
	if err != nil {
		// If we cannot query sessions, return no anomaly rather than blocking login.
		slog.WarnContext(ctx, "failed to query sessions for anomaly detection", "error", err, "user_id", userID)
		return report, nil
	}

	ipKnown := false
	for _, sess := range sessions {
		if strings.TrimSpace(sess.IPAddress) == ip {
			ipKnown = true
			break
		}
	}

	if !ipKnown && len(sessions) > 0 {
		report.IsAnomalous = true
		report.NewIP = true
		report.Reasons = append(report.Reasons, "login from a new IP address")
	}

	// Check country anomaly using geolocation
	if s.GeoLookup != nil && report.Country != "" && len(sessions) > 0 {
		var knownIPs []string
		for _, sess := range sessions {
			if addr := strings.TrimSpace(sess.IPAddress); addr != "" {
				knownIPs = append(knownIPs, addr)
			}
		}
		knownCountries := geo.ExtractCountriesFromIPs(s.GeoLookup, knownIPs)
		if geo.IsNewCountry(s.GeoLookup, ip, knownCountries) {
			report.NewCountry = true
			if !report.IsAnomalous {
				report.IsAnomalous = true
			}
			report.Reasons = append(report.Reasons, fmt.Sprintf("login from a new country (%s)", report.Country))
		}
	}

	return report, nil
}

// DetectAnomalousLoginWithTx detects anomalous login patterns within an existing transaction.
func detectAnomalousLoginWithTx(ctx context.Context, tx appuow.UnitOfWork, geoLookup *geo.LookupService, userID, ip string) *AnomalyReport {
	report := &AnomalyReport{}

	ip = strings.TrimSpace(ip)
	if ip == "" {
		return report
	}

	// Enrich report with geolocation if available
	if geoLookup != nil {
		if result := geoLookup.Lookup(ip); result != nil {
			report.Country = result.Country
			report.City = result.City
		}
	}

	sessions, _, err := tx.Sessions().List(ctx, sessiondomain.ListQuery{
		UserID:     userID,
		PageSize:   50,
		ActiveOnly: boolPtr(false),
	})
	if err != nil {
		slog.WarnContext(ctx, "failed to query sessions for anomaly detection", "error", err, "user_id", userID)
		return report
	}

	ipKnown := false
	for _, sess := range sessions {
		if strings.TrimSpace(sess.IPAddress) == ip {
			ipKnown = true
			break
		}
	}

	if !ipKnown && len(sessions) > 0 {
		report.IsAnomalous = true
		report.NewIP = true
		report.Reasons = append(report.Reasons, "login from a new IP address")
	}

	// Check country anomaly using geolocation
	if geoLookup != nil && report.Country != "" && len(sessions) > 0 {
		var knownIPs []string
		for _, sess := range sessions {
			if addr := strings.TrimSpace(sess.IPAddress); addr != "" {
				knownIPs = append(knownIPs, addr)
			}
		}
		knownCountries := geo.ExtractCountriesFromIPs(geoLookup, knownIPs)
		if geo.IsNewCountry(geoLookup, ip, knownCountries) {
			report.NewCountry = true
			if !report.IsAnomalous {
				report.IsAnomalous = true
			}
			report.Reasons = append(report.Reasons, fmt.Sprintf("login from a new country (%s)", report.Country))
		}
	}

	return report
}

// boolPtr returns a pointer to the given bool value.
func boolPtr(v bool) *bool {
	return &v
}
