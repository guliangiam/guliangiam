package groups

type ListQuery struct {
	Page     int
	PageSize int
	// Offset, when > 0, overrides the page-derived row offset (SCIM
	// startIndex semantics, BE-E-07). Mirrors may ignore it.
	Offset         int
	OrganizationID string
	Keyword        string
	Status         string
	// Type filters by the closed group-type set; empty means no filter.
	Type      string
	SortField string
	SortOrder string
}
