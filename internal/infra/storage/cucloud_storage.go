package storage

import (
	"fmt"
	"strings"
)

// CUCloudStorage implements storage using CUCloud (天翼云) OOS.
// CUCloud provides an S3-compatible API, so we delegate to S3Storage.
type CUCloudStorage struct {
	Provider // Embeds S3Storage via factory registration
}

// NewCUCloudStorage creates a new CUCloud OOS storage provider.
// CUCloud OOS endpoint format: oos-{region}.ctyunapi.cn
func NewCUCloudStorage(s3Provider Provider) *CUCloudStorage {
	return &CUCloudStorage{
		Provider: s3Provider,
	}
}

// cuCloudEndpoint returns the CUCloud OOS endpoint for a given region.
func cuCloudEndpoint(region string) string {
	region = strings.ToLower(strings.TrimSpace(region))
	return fmt.Sprintf("oos-%s.ctyunapi.cn", region)
}
