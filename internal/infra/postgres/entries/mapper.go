package entries

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/entries"
)

// scanEntry scans a full entry row into a domain Entry.
func scanEntry(sc interface{ Scan(...interface{}) error }) (*domain.Entry, error) {
	var e domain.Entry
	err := sc.Scan(&e.ID, &e.OrganizationID, &e.Name, &e.DisplayName, &e.Provider, &e.Application, &e.Type, &e.ClientIP, &e.UserAgent, &e.Message, &e.CreatedAt, &e.UpdatedAt, &e.Revision)
	if err != nil {
		return nil, err
	}
	return &e, nil
}
