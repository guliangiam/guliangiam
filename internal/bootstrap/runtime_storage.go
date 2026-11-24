package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/guliangiam/guliangiam/internal/infra/storage"
)

// buildStorageProvider initializes the storage backend from environment.
//
// Supports 10 providers: local / s3 / minio / oss / cos / huawei / cucloud / qiniu / azure / gcs / synology.
// Defaults to local if STORAGE_TYPE is unset or unrecognized. Panics on provider-specific
// init failures — storage is fail-close at startup.
func buildStorageProvider() storage.Provider {
	// Storage infrastructure: initialize storage provider from environment.
	storageType := envOrEmpty("STORAGE_TYPE")
	if storageType == "" {
		storageType = "local"
	}
	var storageProvider storage.Provider
	switch storageType {
	case "local":
		baseDir := envOrEmpty("STORAGE_LOCAL_BASE_DIR")
		if baseDir == "" {
			baseDir = defaultUploadsDir
		}
		baseURL := envOrEmpty("STORAGE_LOCAL_BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8000/uploads"
		}
		storageProvider = storage.NewLocalStorage(baseDir, baseURL)
	case "s3":
		sp, err := storage.NewS3Storage(context.Background(),
			envOrEmpty("STORAGE_ACCESS_KEY"), envOrEmpty("STORAGE_SECRET_KEY"),
			envOrEmpty("STORAGE_REGION"), envOrEmpty("STORAGE_ENDPOINT"), envOrEmpty("STORAGE_BUCKET"))
		if err != nil {
			panic(fmt.Sprintf("failed to initialize S3 storage: %v", err))
		}
		storageProvider = sp
	case "minio":
		sp, err := storage.NewMinIOStorage(context.Background(),
			envOrEmpty("STORAGE_ACCESS_KEY"), envOrEmpty("STORAGE_SECRET_KEY"),
			envOrEmpty("STORAGE_REGION"), envOrEmpty("STORAGE_ENDPOINT"), envOrEmpty("STORAGE_BUCKET"))
		if err != nil {
			panic(fmt.Sprintf("failed to initialize MinIO storage: %v", err))
		}
		storageProvider = sp
	case "oss":
		sp, err := storage.NewOSSStorage(context.Background(),
			envOrEmpty("STORAGE_ACCESS_KEY"), envOrEmpty("STORAGE_SECRET_KEY"),
			envOrEmpty("STORAGE_REGION"), envOrEmpty("STORAGE_BUCKET"))
		if err != nil {
			panic(fmt.Sprintf("failed to initialize OSS storage: %v", err))
		}
		storageProvider = sp
	case "cos":
		sp, err := storage.NewCOSStorage(context.Background(),
			envOrEmpty("STORAGE_ACCESS_KEY"), envOrEmpty("STORAGE_SECRET_KEY"),
			envOrEmpty("STORAGE_REGION"), envOrEmpty("STORAGE_BUCKET"))
		if err != nil {
			panic(fmt.Sprintf("failed to initialize COS storage: %v", err))
		}
		storageProvider = sp
	case "huawei":
		sp, err := storage.NewHuaweiOBSStorage(context.Background(),
			envOrEmpty("STORAGE_ACCESS_KEY"), envOrEmpty("STORAGE_SECRET_KEY"),
			envOrEmpty("STORAGE_REGION"), envOrEmpty("STORAGE_BUCKET"))
		if err != nil {
			panic(fmt.Sprintf("failed to initialize Huawei OBS storage: %v", err))
		}
		storageProvider = sp
	case "cucloud":
		region := envOrEmpty("STORAGE_REGION")
		endpoint := envOrEmpty("STORAGE_ENDPOINT")
		if endpoint == "" && region != "" {
			endpoint = fmt.Sprintf("https://oos-%s.ctyunapi.cn", strings.ToLower(region))
		}
		sp, err := storage.NewS3Storage(context.Background(),
			envOrEmpty("STORAGE_ACCESS_KEY"), envOrEmpty("STORAGE_SECRET_KEY"),
			region, endpoint, envOrEmpty("STORAGE_BUCKET"))
		if err != nil {
			panic(fmt.Sprintf("failed to initialize CUCloud storage: %v", err))
		}
		storageProvider = storage.NewCUCloudStorage(sp)
	case "qiniu":
		sp, err := storage.NewQiniuStorage(context.Background(),
			envOrEmpty("STORAGE_ACCESS_KEY"), envOrEmpty("STORAGE_SECRET_KEY"),
			envOrEmpty("STORAGE_REGION"), envOrEmpty("STORAGE_BUCKET"))
		if err != nil {
			panic(fmt.Sprintf("failed to initialize Qiniu storage: %v", err))
		}
		storageProvider = sp
	case "azure":
		storageProvider = storage.NewAzureBlobStorage(
			envOrEmpty("STORAGE_AZURE_ACCOUNT_NAME"), envOrEmpty("STORAGE_AZURE_ACCOUNT_KEY"),
			envOrEmpty("STORAGE_AZURE_CONTAINER"))
	case "gcs":
		bucketName := envOrEmpty("STORAGE_BUCKET")
		sp, err := storage.NewGCSStorage(bucketName)
		if err != nil {
			panic(fmt.Sprintf("failed to initialize GCS storage: %v", err))
		}
		storageProvider = sp
	case "synology":
		storageProvider = storage.NewSynologyNASStorage(
			envOrEmpty("STORAGE_ENDPOINT"), envOrEmpty("STORAGE_ACCESS_KEY"), envOrEmpty("STORAGE_SECRET_KEY"))
	default:
		// Default to local storage if the provider type is not recognized.
		baseDir := envOrEmpty("STORAGE_LOCAL_BASE_DIR")
		if baseDir == "" {
			baseDir = defaultUploadsDir
		}
		baseURL := envOrEmpty("STORAGE_LOCAL_BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8000/uploads"
		}
		storageProvider = storage.NewLocalStorage(baseDir, baseURL)
	}
	return storageProvider
}
