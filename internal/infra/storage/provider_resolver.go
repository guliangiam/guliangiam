package storage

import (
	"context"
	"strings"
	"sync"

	domainproviders "github.com/guliangiam/guliangiam/internal/domain/providers"
	domainresources "github.com/guliangiam/guliangiam/internal/domain/resources"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// providerLister is the narrow repository surface the resolver needs. Keeping
// it narrow lets tests use a stub and keeps the resolver independent of the
// full provider repository contract.
type providerLister interface {
	List(ctx context.Context, query domainproviders.ListQuery) ([]*domainproviders.Provider, int64, error)
}

type providerSinkEntry struct {
	revision int64
	provider domainresources.StorageProvider
}

// ProviderResolver builds an organization-scoped storage backend from the
// enabled Storage provider row. Supported types are the B2/P7 kinds (aws_s3,
// azure_blob, ali_oss, tencent_cos, cu_cloud, huawei_obs, cloudflare_r2,
// backblaze_b2, digitalocean_spaces); other types
// (Local/MinIO/GCS/Custom/Synology/Qiniu) fall back to the deployment backend
// so existing deployments keep working.
type ProviderResolver struct {
	repo providerLister

	mu    sync.Mutex
	sinks map[string]providerSinkEntry
}

var _ domainresources.StorageProviderResolver = (*ProviderResolver)(nil)

func NewProviderResolver(repo providerLister) *ProviderResolver {
	return &ProviderResolver{repo: repo, sinks: make(map[string]providerSinkEntry)}
}

func (r *ProviderResolver) Resolve(ctx context.Context, organizationID string) (domainresources.StorageProvider, error) {
	if r == nil || r.repo == nil || strings.TrimSpace(organizationID) == "" {
		recordStorageResolution("unconfigured")
		return nil, domainresources.ErrStorageProviderUnconfigured
	}

	rows, _, err := r.repo.List(ctx, domainproviders.ListQuery{
		OrganizationID: organizationID,
		Category:       string(domainproviders.CategoryStorage),
		Page:           1,
		PageSize:       10,
	})
	if err != nil {
		recordStorageResolution("error")
		return nil, apperrors.WrapInternalf(err, "list storage providers")
	}

	var disabled *domainproviders.Provider
	for _, row := range rows {
		if row == nil || !isResolvableStorageType(row.ProviderType) {
			continue
		}
		if !row.Enabled {
			if disabled == nil {
				disabled = row
			}
			continue
		}
		sink, err := r.sinkFor(row)
		if err != nil {
			recordStorageResolution("invalid")
			return nil, err
		}
		recordStorageResolution("database")
		return sink, nil
	}
	if disabled != nil {
		recordStorageResolution("disabled")
		return nil, apperrors.NewUnavailablef("storage provider %q is disabled", disabled.Name)
	}
	recordStorageResolution("unconfigured")
	return nil, domainresources.ErrStorageProviderUnconfigured
}

func (r *ProviderResolver) sinkFor(provider *domainproviders.Provider) (domainresources.StorageProvider, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entry, ok := r.sinks[provider.ID]; ok && entry.revision == provider.Revision {
		return entry.provider, nil
	}

	sink, err := buildStorageProvider(provider)
	if err != nil {
		return nil, err
	}
	r.sinks[provider.ID] = providerSinkEntry{revision: provider.Revision, provider: sink}
	return sink, nil
}

// isResolvableStorageType reports whether this batch owns the provider type.
// The check is case/space/underscore-insensitive because the console stores
// vendor spellings ("AWS S3", "HuaweiOBS") while the registry uses snake keys.
func isResolvableStorageType(providerType domainproviders.ProviderType) bool {
	switch normalizeStorageType(string(providerType)) {
	case "awss3", "azureblob", "alioss", "tencentcos", "cucloud", "huaweiobs",
		"cloudflarer2", "backblazeb2", "digitaloceanspaces":
		return true
	default:
		return false
	}
}

func normalizeStorageType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer(" ", "", "_", "", "-", "").Replace(value)
	return value
}

func buildStorageProvider(provider *domainproviders.Provider) (domainresources.StorageProvider, error) {
	bucket := strings.TrimSpace(provider.AppId)
	region := strings.TrimSpace(provider.Region)
	endpoint := strings.TrimSpace(provider.Host)
	accessKey := strings.TrimSpace(provider.Admin)
	secretKey := strings.TrimSpace(provider.Password)

	switch normalizeStorageType(string(provider.ProviderType)) {
	case "awss3":
		if bucket == "" || accessKey == "" || secretKey == "" {
			return nil, apperrors.NewUnavailable("AWS S3 storage provider requires bucket, accessKeyId and accessKeySecret")
		}
		return NewS3Storage(context.Background(), accessKey, secretKey, defaultRegion(region), endpoint, bucket)
	case "alioss":
		if bucket == "" || accessKey == "" || secretKey == "" {
			return nil, apperrors.NewUnavailable("Ali OSS storage provider requires bucket, accessKeyId and accessKeySecret")
		}
		return NewOSSStorage(context.Background(), accessKey, secretKey, region, bucket)
	case "tencentcos":
		if bucket == "" || accessKey == "" || secretKey == "" {
			return nil, apperrors.NewUnavailable("Tencent COS storage provider requires bucket, accessKeyId and accessKeySecret")
		}
		return NewCOSStorage(context.Background(), accessKey, secretKey, region, bucket)
	case "huaweiobs":
		if bucket == "" || accessKey == "" || secretKey == "" {
			return nil, apperrors.NewUnavailable("Huawei OBS storage provider requires bucket, accessKeyId and accessKeySecret")
		}
		return NewHuaweiOBSStorage(context.Background(), accessKey, secretKey, region, bucket)
	case "cucloud":
		if bucket == "" || accessKey == "" || secretKey == "" {
			return nil, apperrors.NewUnavailable("CUCloud storage provider requires bucket, accessKeyId and accessKeySecret")
		}
		if endpoint == "" && region == "" {
			return nil, apperrors.NewUnavailable("CUCloud storage provider requires endpoint or region")
		}
		if endpoint == "" {
			endpoint = "https://" + cuCloudEndpoint(region)
		}
		s3, err := NewS3Storage(context.Background(), accessKey, secretKey, defaultRegion(region), endpoint, bucket)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "build CUCloud storage provider")
		}
		return NewCUCloudStorage(s3), nil
	case "azureblob":
		account := strings.TrimSpace(provider.Admin)
		accountKey := strings.TrimSpace(provider.Password)
		container := bucket
		if account == "" || accountKey == "" || container == "" {
			return nil, apperrors.NewUnavailable("Azure Blob storage provider requires account name, account key and container")
		}
		return NewAzureBlobStorage(account, accountKey, container), nil
	case "cloudflarer2":
		if bucket == "" || accessKey == "" || secretKey == "" || endpoint == "" {
			return nil, apperrors.NewUnavailable("Cloudflare R2 storage provider requires bucket, accessKeyId, accessKeySecret and endpoint")
		}
		return NewCloudflareR2Storage(context.Background(), accessKey, secretKey, region, endpoint, bucket)
	case "backblazeb2":
		if bucket == "" || accessKey == "" || secretKey == "" {
			return nil, apperrors.NewUnavailable("Backblaze B2 storage provider requires bucket, accessKeyId and accessKeySecret")
		}
		if endpoint == "" && region == "" {
			return nil, apperrors.NewUnavailable("Backblaze B2 storage provider requires endpoint or region")
		}
		return NewBackblazeB2Storage(context.Background(), accessKey, secretKey, region, endpoint, bucket)
	case "digitaloceanspaces":
		if bucket == "" || accessKey == "" || secretKey == "" {
			return nil, apperrors.NewUnavailable("DigitalOcean Spaces storage provider requires bucket, accessKeyId and accessKeySecret")
		}
		if endpoint == "" && region == "" {
			return nil, apperrors.NewUnavailable("DigitalOcean Spaces storage provider requires endpoint or region")
		}
		return NewDigitalOceanSpacesStorage(context.Background(), accessKey, secretKey, region, endpoint, bucket)
	default:
		return nil, domainresources.ErrStorageProviderUnconfigured
	}
}

func defaultRegion(region string) string {
	if strings.TrimSpace(region) == "" {
		return "us-east-1"
	}
	return region
}
