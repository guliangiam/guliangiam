package syncer

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// AWSIAMSyncer syncs users and groups from AWS IAM.
type AWSIAMSyncer struct{}

// awsIAMListUsersResponse represents the response from AWS IAM ListUsers API.
type awsIAMListUsersResponse struct {
	Users []struct {
		UserName   string `json:"UserName"`
		UserID     string `json:"UserId"`
		ARN        string `json:"Arn"`
		CreateDate string `json:"CreateDate"`
		Path       string `json:"Path"`
	} `json:"Users"`
	IsTruncated bool   `json:"IsTruncated"`
	Marker      string `json:"Marker"`
}

// awsIAMListGroupsResponse represents the response from AWS IAM ListGroups API.
type awsIAMListGroupsResponse struct {
	Groups []struct {
		GroupName  string `json:"GroupName"`
		GroupID    string `json:"GroupId"`
		ARN        string `json:"Arn"`
		CreateDate string `json:"CreateDate"`
		Path       string `json:"Path"`
	} `json:"Groups"`
	IsTruncated bool   `json:"IsTruncated"`
	Marker      string `json:"Marker"`
}

// SyncUsers synchronizes users from AWS IAM.
func (s *AWSIAMSyncer) SyncUsers(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	result := &SyncResult{}

	users, errs := s.listUsers(ctx, config)
	result.TotalUsers = len(users)
	result.Errors = append(result.Errors, errs...)

	// Map users and persist via write-back callback
	isIncremental := config.LastSyncAt != nil
	for _, u := range users {
		if u.UserName == "" {
			result.FailedUsers++
			result.Errors = append(result.Errors, fmt.Sprintf("awsiam user missing UserName: userId=%s", u.UserID))
			continue
		}

		mapped := s.mapUser(u, config.AttributeMap)

		if config.OnUserSynced != nil {
			created, err := config.OnUserSynced(ctx, "awsiam", u.UserID, mapped)
			if err != nil {
				result.FailedUsers++
				result.Errors = append(result.Errors, fmt.Sprintf("awsiam user write-back failed: userId=%s, err=%v", u.UserID, err))
				continue
			}
			if created {
				result.AddedUsers++
			} else {
				result.UpdatedUsers++
			}
		} else {
			// Read-only mode: count without persisting
			if isIncremental {
				result.UpdatedUsers++
			} else {
				result.AddedUsers++
			}
		}
	}

	return result, nil
}

// SyncGroups synchronizes groups from AWS IAM.
func (s *AWSIAMSyncer) SyncGroups(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	result := &SyncResult{}

	groups, errs := s.listGroups(ctx, config)
	result.TotalGroups = len(groups)
	result.Errors = append(result.Errors, errs...)

	// Map groups and persist via write-back callback
	isIncremental := config.LastSyncAt != nil
	for _, g := range groups {
		if g.GroupName == "" {
			result.FailedGroups++
			result.Errors = append(result.Errors, fmt.Sprintf("awsiam group missing GroupName: groupId=%s", g.GroupID))
			continue
		}

		mapped := s.mapGroup(g, config.AttributeMap)

		if config.OnGroupSynced != nil {
			created, err := config.OnGroupSynced(ctx, config.OrganizationID, g.GroupID, mapped)
			if err != nil {
				result.FailedGroups++
				result.Errors = append(result.Errors, fmt.Sprintf("awsiam group write-back failed: groupId=%s, err=%v", g.GroupID, err))
				continue
			}
			if created {
				result.AddedGroups++
			} else {
				result.UpdatedGroups++
			}
		} else {
			// Read-only mode: count without persisting
			if isIncremental {
				result.UpdatedGroups++
			} else {
				result.AddedGroups++
			}
		}
	}

	return result, nil
}

// TestConnection tests the connection to AWS IAM by calling ListUsers with MaxItems=1.
func (s *AWSIAMSyncer) TestConnection(ctx context.Context, config *SyncerConfig) error {
	iamURL := s.getIAMEndpoint(config)

	params := url.Values{}
	params.Set("Action", "ListUsers")
	params.Set("Version", "2010-05-08")
	params.Set("MaxItems", "1")

	req, err := s.signRequest(ctx, "GET", iamURL, params, config)
	if err != nil {
		return apperrors.WrapInternalf(err, "awsiam test connection sign request")
	}

	client := ssrf.SafeHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "awsiam test connection request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("awsiam test connection HTTP error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// getIAMEndpoint returns the AWS IAM endpoint URL.
func (s *AWSIAMSyncer) getIAMEndpoint(config *SyncerConfig) string {
	if config.APIURL != "" {
		return config.APIURL
	}
	if config.Host != "" {
		return config.Host
	}
	return "https://iam.amazonaws.com/"
}

// listUsers lists all IAM users via the AWS IAM API.
func (s *AWSIAMSyncer) listUsers(ctx context.Context, config *SyncerConfig) (users []awsIAMUserItem, errs []string) {
	return fetchAWSIAMPages[awsIAMUserItem](ctx, s, config, "ListUsers", "users",
		func(body []byte) ([]awsIAMUserItem, bool, string, error) {
			var listResp awsIAMListUsersResponse
			if err := json.Unmarshal(body, &listResp); err != nil {
				return nil, false, "", err
			}
			items := make([]awsIAMUserItem, 0, len(listResp.Users))
			for _, u := range listResp.Users {
				items = append(items, awsIAMUserItem{
					UserName:   u.UserName,
					UserID:     u.UserID,
					ARN:        u.ARN,
					CreateDate: u.CreateDate,
					Path:       u.Path,
				})
			}
			return items, listResp.IsTruncated, listResp.Marker, nil
		})
}

type awsIAMUserItem = struct {
	UserName   string `json:"UserName"`
	UserID     string `json:"UserId"`
	ARN        string `json:"Arn"`
	CreateDate string `json:"CreateDate"`
	Path       string `json:"Path"`
}

// listGroups lists all IAM groups via the AWS IAM API.
func (s *AWSIAMSyncer) listGroups(ctx context.Context, config *SyncerConfig) (groups []awsIAMGroupItem, errs []string) {
	return fetchAWSIAMPages[awsIAMGroupItem](ctx, s, config, "ListGroups", "groups",
		func(body []byte) ([]awsIAMGroupItem, bool, string, error) {
			var listResp awsIAMListGroupsResponse
			if err := json.Unmarshal(body, &listResp); err != nil {
				return nil, false, "", err
			}
			items := make([]awsIAMGroupItem, 0, len(listResp.Groups))
			for _, g := range listResp.Groups {
				items = append(items, awsIAMGroupItem{
					GroupName:  g.GroupName,
					GroupID:    g.GroupID,
					ARN:        g.ARN,
					CreateDate: g.CreateDate,
					Path:       g.Path,
				})
			}
			return items, listResp.IsTruncated, listResp.Marker, nil
		})
}

// fetchAWSIAMPages follows the AWS IAM Marker pagination and returns the
// accumulated items plus per-page errors.
func fetchAWSIAMPages[T any](
	ctx context.Context,
	s *AWSIAMSyncer,
	config *SyncerConfig,
	action, kind string,
	decode func([]byte) ([]T, bool, string, error),
) (items []T, errs []string) {
	iamURL := s.getIAMEndpoint(config)
	marker := ""

	// Note: AWS IAM API does not support a timestamp filter for incremental sync.
	// Full sync is performed regardless of LastSyncAt setting.
	for {
		params := url.Values{}
		params.Set("Action", action)
		params.Set("Version", "2010-05-08")
		if config.Filter != "" {
			params.Set("PathPrefix", config.Filter)
		}
		if marker != "" {
			params.Set("Marker", marker)
		}

		req, err := s.signRequest(ctx, "GET", iamURL, params, config)
		if err != nil {
			errs = append(errs, fmt.Sprintf("awsiam sign list %s request: %v", kind, err))
			break
		}

		client := ssrf.SafeHTTPClient(30 * time.Second)
		resp, err := client.Do(req)
		if err != nil {
			errs = append(errs, fmt.Sprintf("awsiam list %s request: %v", kind, err))
			break
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			errs = append(errs, fmt.Sprintf("awsiam read list %s response: %v", kind, err))
			break
		}

		if resp.StatusCode != http.StatusOK {
			errs = append(errs, fmt.Sprintf("awsiam list %s HTTP error: status=%d, body=%s", kind, resp.StatusCode, string(body)))
			break
		}

		pageItems, truncated, next, err := decode(body)
		if err != nil {
			errs = append(errs, fmt.Sprintf("awsiam parse list %s response: %v", kind, err))
			break
		}
		items = append(items, pageItems...)
		if !truncated {
			break
		}
		marker = next
	}

	return items, errs
}

type awsIAMGroupItem = struct {
	GroupName  string `json:"GroupName"`
	GroupID    string `json:"GroupId"`
	ARN        string `json:"Arn"`
	CreateDate string `json:"CreateDate"`
	Path       string `json:"Path"`
}

// signRequest creates and signs an AWS IAM API request using Signature Version 4.
// config.ClientID is used as AWS Access Key ID.
// config.ClientSecret is used as AWS Secret Access Key.
// config.TenantID is used as AWS Region (defaults to "us-east-1").
func (s *AWSIAMSyncer) signRequest(ctx context.Context, method, endpoint string, params url.Values, config *SyncerConfig) (*http.Request, error) {
	accessKeyID := config.ClientID
	secretAccessKey := config.ClientSecret
	region := config.TenantID
	if region == "" {
		region = "us-east-1"
	}

	// AWS IAM service name
	service := "iam"

	// Get current time
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	// Set required parameters for AWS Signature Version 4
	params.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	params.Set("X-Amz-Credential", fmt.Sprintf("%s/%s/%s/%s/aws4_request", accessKeyID, dateStamp, region, service))
	params.Set("X-Amz-Date", amzDate)
	params.Set("X-Amz-Expires", "30")
	params.Set("X-Amz-SignedHeaders", "host")

	// Build the canonical request
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "awsiam parse endpoint URL")
	}
	host := parsedURL.Host

	// Sort the query parameters (url.Values.Encode sorts them)
	canonicalQueryString := params.Encode()

	// Construct canonical URI
	canonicalURI := parsedURL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	canonicalHeaders := fmt.Sprintf("host:%s\n", host)
	signedHeaders := "host"

	// Payload hash for GET request (empty body)
	payloadHash := sha256Hex([]byte(""))

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	)

	// Build string to sign
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, region, service)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	)

	// Calculate signing key
	signingKey := getSignatureKey(secretAccessKey, dateStamp, region)

	// Calculate signature
	signature := hmacSHA256Hex(signingKey, []byte(stringToSign))

	// Add signature to query parameters
	params.Set("X-Amz-Signature", signature)

	// Build final URL
	fullURL := fmt.Sprintf("%s?%s", strings.TrimRight(endpoint, "/"), params.Encode())

	req, err := http.NewRequestWithContext(ctx, method, fullURL, http.NoBody)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "awsiam create signed request")
	}

	return req, nil
}

// sha256Hex computes the SHA256 hash of data and returns it as a hex string.
func sha256Hex(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// hmacSHA256 computes HMAC-SHA256 with the given key and data.
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// hmacSHA256Hex computes HMAC-SHA256 and returns the result as a hex string.
func hmacSHA256Hex(key, data []byte) string {
	return hex.EncodeToString(hmacSHA256(key, data))
}

// getSignatureKey derives the signing key for AWS Signature Version 4.
func getSignatureKey(secretKey, dateStamp, region string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(awsIAMServiceName))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}

// mapUser maps an AWS IAM user to a field map using the configured attribute mapping.
// Default mapping (when AttributeMap is empty):
//   - UserName -> username, Arn -> arn, Path -> path
func (s *AWSIAMSyncer) mapUser(item awsIAMUserItem, attributeMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	if len(attributeMap) == 0 {
		// Default AWSIAM -> GuliangIAM field mapping
		result["id"] = item.UserID
		result["username"] = item.UserName
		result["arn"] = item.ARN
		result["path"] = item.Path
		result["createDate"] = item.CreateDate
		return result
	}

	// Apply custom mapping: AttributeMap key is the AWSIAM source field name
	if target, ok := attributeMap["UserId"]; ok {
		result[target] = item.UserID
	}
	if target, ok := attributeMap["UserName"]; ok {
		result[target] = item.UserName
	}
	if target, ok := attributeMap["Arn"]; ok {
		result[target] = item.ARN
	}
	if target, ok := attributeMap["Path"]; ok {
		result[target] = item.Path
	}
	if target, ok := attributeMap["CreateDate"]; ok {
		result[target] = item.CreateDate
	}

	return result
}

// mapGroup maps an AWS IAM group to a field map using the configured attribute mapping.
// Default mapping (when AttributeMap is empty):
//   - GroupName -> name, Arn -> arn, Path -> path
func (s *AWSIAMSyncer) mapGroup(item awsIAMGroupItem, attributeMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	if len(attributeMap) == 0 {
		result["id"] = item.GroupID
		result["name"] = item.GroupName
		result["arn"] = item.ARN
		result["path"] = item.Path
		result["createDate"] = item.CreateDate
		return result
	}

	if target, ok := attributeMap["GroupId"]; ok {
		result[target] = item.GroupID
	}
	if target, ok := attributeMap["GroupName"]; ok {
		result[target] = item.GroupName
	}
	if target, ok := attributeMap["Arn"]; ok {
		result[target] = item.ARN
	}
	if target, ok := attributeMap["Path"]; ok {
		result[target] = item.Path
	}
	if target, ok := attributeMap["CreateDate"]; ok {
		result[target] = item.CreateDate
	}

	return result
}

// Ensure AWSIAMSyncer implements Engine.
var _ Engine = (*AWSIAMSyncer)(nil)
