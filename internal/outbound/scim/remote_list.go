package scim

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	defaultReconcilePageSize = 100
	maxReconcileResources    = 10000
)

type remoteEmail struct {
	Value   string `json:"value"`
	Primary bool   `json:"primary"`
}

type remoteUserResource struct {
	ID         string        `json:"id"`
	UserName   string        `json:"userName"`
	ExternalID string        `json:"externalId"`
	Active     bool          `json:"active"`
	Title      string        `json:"title"`
	Emails     []remoteEmail `json:"emails"`
}

type remoteGroupResource struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	ExternalID  string `json:"externalId"`
	Members     []struct {
		Value string `json:"value"`
	} `json:"members"`
}

type listResponse[T any] struct {
	TotalResults int `json:"totalResults"`
	StartIndex   int `json:"startIndex"`
	ItemsPerPage int `json:"itemsPerPage"`
	Resources    []T `json:"Resources"`
}

// ListRemoteUsers pages through GET /Users and normalizes each resource into a
// UserRecord. A zero pageSize means the SCIM default (100). The loop is bounded
// so a malicious or broken downstream that ignores startIndex cannot spin
// forever.
func ListRemoteUsers(ctx context.Context, client *Client, pageSize int) ([]UserRecord, error) {
	if client == nil {
		return nil, apperrors.NewInvalidArgument("scim client is required")
	}
	if pageSize <= 0 {
		pageSize = defaultReconcilePageSize
	}
	var users []UserRecord
	startIndex := 1
	for pages := 0; pages < maxReconcileResources/pageSize+1; pages++ {
		page, err := fetchPage[remoteUserResource](ctx, client, "/Users", startIndex, pageSize)
		if err != nil {
			return nil, err
		}
		for _, resource := range page.Resources {
			users = append(users, remoteUserToRecord(resource))
		}
		startIndex += len(page.Resources)
		// Keep paging until the reported total is collected; a downstream that
		// returns fewer rows than requested is not treated as the end.
		if len(page.Resources) == 0 || len(users) >= page.TotalResults {
			return users, nil
		}
	}
	return users, apperrors.NewUnavailablef("scim list users exceeded %d resources", maxReconcileResources)
}

// ListRemoteGroups pages through GET /Groups and normalizes each resource.
func ListRemoteGroups(ctx context.Context, client *Client, pageSize int) ([]GroupRecord, error) {
	if client == nil {
		return nil, apperrors.NewInvalidArgument("scim client is required")
	}
	if pageSize <= 0 {
		pageSize = defaultReconcilePageSize
	}
	var groups []GroupRecord
	startIndex := 1
	for pages := 0; pages < maxReconcileResources/pageSize+1; pages++ {
		page, err := fetchPage[remoteGroupResource](ctx, client, "/Groups", startIndex, pageSize)
		if err != nil {
			return nil, err
		}
		for _, resource := range page.Resources {
			groups = append(groups, remoteGroupToRecord(resource))
		}
		startIndex += len(page.Resources)
		if len(page.Resources) == 0 || len(groups) >= page.TotalResults {
			return groups, nil
		}
	}
	return groups, apperrors.NewUnavailablef("scim list groups exceeded %d resources", maxReconcileResources)
}

func fetchPage[T any](ctx context.Context, client *Client, path string, startIndex, pageSize int) (listResponse[T], error) {
	var page listResponse[T]
	query := url.Values{}
	query.Set("startIndex", strconv.Itoa(startIndex))
	query.Set("count", strconv.Itoa(pageSize))
	status, body, err := client.Do(ctx, http.MethodGet, path+"?"+query.Encode(), nil)
	if err != nil {
		return page, err
	}
	if status < 200 || status >= 300 {
		return page, apperrors.NewUnavailablef("scim %s returned status %d", path, status)
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return page, apperrors.WrapInternalf(err, "scim decode %s", path)
	}
	return page, nil
}

func remoteUserToRecord(resource remoteUserResource) UserRecord {
	attributes := map[string]string{}
	if email := primaryEmail(resource.Emails); email != "" {
		attributes["email"] = email
	}
	if title := strings.TrimSpace(resource.Title); title != "" {
		attributes["title"] = title
	}
	return UserRecord{
		ID:         resource.ID,
		UserName:   resource.UserName,
		ExternalID: resource.ExternalID,
		Active:     resource.Active,
		Attributes: attributes,
	}
}

func remoteGroupToRecord(resource remoteGroupResource) GroupRecord {
	members := make([]string, 0, len(resource.Members))
	for _, member := range resource.Members {
		if value := strings.TrimSpace(member.Value); value != "" {
			members = append(members, value)
		}
	}
	return GroupRecord{
		ID:          resource.ID,
		DisplayName: resource.DisplayName,
		ExternalID:  resource.ExternalID,
		Members:     members,
	}
}

func primaryEmail(emails []remoteEmail) string {
	for _, email := range emails {
		if email.Primary && strings.TrimSpace(email.Value) != "" {
			return strings.TrimSpace(email.Value)
		}
	}
	for _, email := range emails {
		if strings.TrimSpace(email.Value) != "" {
			return strings.TrimSpace(email.Value)
		}
	}
	return ""
}

// FetchAndReconcile fetches downstream Users/Groups and diffs them against the
// local expected state. It is the integration seam for a scheduled reconcile
// job; persistence/audit of the report remain the caller's responsibility.
func FetchAndReconcile(ctx context.Context, client *Client, expectedUsers []UserRecord, expectedGroups []GroupRecord, pageSize int) (ReconciliationReport, GroupReconciliationReport, error) {
	remoteUsers, err := ListRemoteUsers(ctx, client, pageSize)
	if err != nil {
		return ReconciliationReport{}, GroupReconciliationReport{}, err
	}
	remoteGroups, err := ListRemoteGroups(ctx, client, pageSize)
	if err != nil {
		return ReconciliationReport{}, GroupReconciliationReport{}, err
	}
	return ReconcileUsers(expectedUsers, remoteUsers), ReconcileGroups(expectedGroups, remoteGroups), nil
}
