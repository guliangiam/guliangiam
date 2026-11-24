package forms

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/forms"
)

// scanForm scans a full form row into a domain Form.
func scanForm(sc interface{ Scan(...interface{}) error }) (*domain.Form, error) {
	var f domain.Form
	err := sc.Scan(&f.ID, &f.OrganizationID, &f.Name, &f.DisplayName, &f.Description, &f.Type, &f.Properties, &f.IsEnabled, &f.CreatedAt, &f.UpdatedAt, &f.Revision)
	if err != nil {
		return nil, err
	}
	return &f, nil
}
