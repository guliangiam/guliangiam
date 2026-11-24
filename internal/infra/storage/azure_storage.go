package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// AzureBlobStorage stores objects in Azure Blob Storage using the REST API.
//
// 校准（OPEN-API-75 P7）：授权为 SharedKey（HMAC-SHA256，
// https://learn.microsoft.com/en-us/rest/api/storageservices/authorize-with-shared-key）；
// SAS（https://learn.microsoft.com/en-us/rest/api/storageservices/delegate-access-with-shared-access-signature）
// 未接入——控制台凭据模型只承载账号名+账号密钥，SAS 属后续凭据形态扩展。
// x-ms-version 固定 2023-01-03，仍在服务版本支持列表内
// （https://learn.microsoft.com/en-us/rest/api/storageservices/versioning-for-the-azure-storage-services）。
type AzureBlobStorage struct {
	accountName   string
	accountKey    string
	containerName string
	baseURL       string
	httpClient    *http.Client
}

// NewAzureBlobStorage creates a new Azure Blob Storage provider.
// accountName is the Azure storage account name.
// accountKey is the Azure storage account key (base64-encoded).
// containerName is the blob container name.
func NewAzureBlobStorage(accountName, accountKey, containerName string) Provider {
	baseURL := fmt.Sprintf("https://%s.blob.core.windows.net", accountName)
	return &AzureBlobStorage{
		accountName:   accountName,
		accountKey:    accountKey,
		containerName: containerName,
		baseURL:       baseURL,
		httpClient:    ssrf.SafeHTTPClient(30 * time.Second),
	}
}

// buildSharedKeySignature builds the SharedKey authorization header for Azure Blob REST API.
func (a *AzureBlobStorage) buildSharedKeySignature(method, contentEncoding, contentLanguage, contentLength, contentMD5, contentType, date, ifModifiedSince, ifMatch, ifNoneMatch, ifUnmodifiedSince, rangeHeader, resource string) (string, error) {
	stringToSign := strings.Join([]string{
		method,
		contentEncoding,
		contentLanguage,
		contentLength,
		contentMD5,
		contentType,
		date,
		ifModifiedSince,
		ifMatch,
		ifNoneMatch,
		ifUnmodifiedSince,
		rangeHeader,
	}, "\n")

	// Append the canonicalized resource string.
	canonicalizedResource := fmt.Sprintf("/%s/%s", a.accountName, resource)
	stringToSign += "\n" + canonicalizedResource

	keyBytes, err := base64.StdEncoding.DecodeString(a.accountKey)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "azure decode account key")
	}

	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("SharedKey %s:%s", a.accountName, signature), nil
}

func (a *AzureBlobStorage) Put(ctx context.Context, key string, data []byte, contentType string) error {
	resource := fmt.Sprintf("%s/%s", a.containerName, key)
	date := time.Now().UTC().Format(http.TimeFormat)
	contentLength := fmt.Sprintf("%d", len(data))

	auth, err := a.buildSharedKeySignature(
		"PUT", "", "", contentLength, "", contentType, date, "", "", "", "", "", resource,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "azure put sign")
	}

	reqURL := fmt.Sprintf("%s/%s/%s", a.baseURL, a.containerName, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, reqURL, bytes.NewReader(data))
	if err != nil {
		return apperrors.WrapInternalf(err, "azure put request")
	}

	req.Header.Set("x-ms-version", "2023-01-03")
	req.Header.Set("x-ms-blob-type", "BlockBlob")
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Content-Length", contentLength)
	req.Header.Set("Date", date)
	req.Header.Set("Authorization", auth)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "azure put")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("azure put failed (status %d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (a *AzureBlobStorage) Get(ctx context.Context, key string) ([]byte, error) {
	resource := fmt.Sprintf("%s/%s", a.containerName, key)
	date := time.Now().UTC().Format(http.TimeFormat)

	auth, err := a.buildSharedKeySignature(
		"GET", "", "", "", "", "", date, "", "", "", "", "", resource,
	)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "azure get sign")
	}

	reqURL := fmt.Sprintf("%s/%s/%s", a.baseURL, a.containerName, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "azure get request")
	}

	req.Header.Set("x-ms-version", "2023-01-03")
	req.Header.Set("Date", date)
	req.Header.Set("Authorization", auth)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "azure get")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, apperrors.NewNotFoundf("azure get: object not found: %s", key)
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperrors.NewInternalf("azure get failed (status %d): %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "azure get read body")
	}
	return data, nil
}

func (a *AzureBlobStorage) Delete(ctx context.Context, key string) error {
	resource := fmt.Sprintf("%s/%s", a.containerName, key)
	date := time.Now().UTC().Format(http.TimeFormat)

	auth, err := a.buildSharedKeySignature(
		"DELETE", "", "", "", "", "", date, "", "", "", "", "", resource,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "azure delete sign")
	}

	reqURL := fmt.Sprintf("%s/%s/%s", a.baseURL, a.containerName, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, http.NoBody)
	if err != nil {
		return apperrors.WrapInternalf(err, "azure delete request")
	}

	req.Header.Set("x-ms-version", "2023-01-03")
	req.Header.Set("Date", date)
	req.Header.Set("Authorization", auth)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "azure delete")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("azure delete failed (status %d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// azureBlob is used for XML unmarshaling of list results.
type azureBlob struct {
	Name       string `xml:"Name"`
	Properties struct {
		ContentLength int64  `xml:"Content-Length"`
		LastModified  string `xml:"Last-Modified"`
		ContentType   string `xml:"Content-Type"`
	} `xml:"Properties"`
}

// azureEnumerationResults is used for XML unmarshaling of list results.
type azureEnumerationResults struct {
	Blobs struct {
		Blob []azureBlob `xml:"Blob"`
	} `xml:"Blobs"`
}

func (a *AzureBlobStorage) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	date := time.Now().UTC().Format(http.TimeFormat)

	// Build query parameters for listing.
	params := url.Values{}
	params.Set("restype", "container")
	params.Set("comp", "list")
	if prefix != "" {
		params.Set("prefix", prefix)
	}

	// The canonicalized resource for list includes the query parameters.
	canonicalizedResource := fmt.Sprintf("/%s/%s\ncomp:list\nrestype:container", a.accountName, a.containerName)
	if prefix != "" {
		canonicalizedResource += fmt.Sprintf("\nprefix:%s", prefix)
	}

	stringToSign := strings.Join([]string{
		"GET",
		"", // Content-Encoding
		"", // Content-Language
		"", // Content-Length
		"", // Content-MD5
		"", // Content-Type
		date,
		"", // If-Modified-Since
		"", // If-Match
		"", // If-None-Match
		"", // If-Unmodified-Since
		"", // Range
	}, "\n")
	stringToSign += "\n" + canonicalizedResource

	keyBytes, err := base64.StdEncoding.DecodeString(a.accountKey)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "azure list decode key")
	}

	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	auth := fmt.Sprintf("SharedKey %s:%s", a.accountName, signature)

	reqURL := fmt.Sprintf("%s/%s?%s", a.baseURL, a.containerName, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "azure list request")
	}

	req.Header.Set("x-ms-version", "2023-01-03")
	req.Header.Set("Date", date)
	req.Header.Set("Authorization", auth)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "azure list")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperrors.NewInternalf("azure list failed (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "azure list read body")
	}

	var results azureEnumerationResults
	if err := xml.Unmarshal(body, &results); err != nil {
		return nil, apperrors.WrapInternalf(err, "azure list unmarshal")
	}

	infos := make([]ObjectInfo, 0, len(results.Blobs.Blob))
	for _, blob := range results.Blobs.Blob {
		lastModified, _ := time.Parse(time.RFC1123, blob.Properties.LastModified)
		infos = append(infos, ObjectInfo{
			Key:          blob.Name,
			Size:         blob.Properties.ContentLength,
			LastModified: lastModified,
			ContentType:  blob.Properties.ContentType,
		})
	}
	return infos, nil
}

func (a *AzureBlobStorage) GetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	// Azure Blob SAS token approach for generating a pre-signed URL.
	start := time.Now().UTC()
	expiry := start.Add(expires)

	sasParams := url.Values{}
	sasParams.Set("sv", "2023-01-03")
	sasParams.Set("sr", "b")
	sasParams.Set("sp", "r")
	sasParams.Set("st", start.Format("2006-01-02T15:04:05Z"))
	sasParams.Set("se", expiry.Format("2006-01-02T15:04:05Z"))

	// Build the string to sign for SAS.
	stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s",
		a.accountName, // signedServices
		"b",           // signedResourceType
		"r",           // signedPermission
		start.Format("2006-01-02T15:04:05Z"),
		expiry.Format("2006-01-02T15:04:05Z"),
		"",                       // signedIP
		"https",                  // signedProtocol
		"2023-01-03",             // signedVersion
		a.containerName+"\n"+key, // canonicalName
		"",                       // signedEncryptionScope
		"",                       // signedDirectoryDepth
		"",                       // authorizedStorageAccountObjectId
		"",                       // authorizedServiceObjectId
		"",                       // authorizedUserObjectId
		"",                       // signedAuthorizedAcl
	)

	keyBytes, err := base64.StdEncoding.DecodeString(a.accountKey)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "azure geturl decode key")
	}

	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	sasParams.Set("sig", signature)

	blobURL := fmt.Sprintf("%s/%s/%s?%s", a.baseURL, a.containerName, key, sasParams.Encode())
	return blobURL, nil
}

func (a *AzureBlobStorage) PutStream(ctx context.Context, key string, reader io.Reader, contentType string) error {
	// Read all data from the reader and delegate to Put.
	data, err := io.ReadAll(reader)
	if err != nil {
		return apperrors.WrapInternalf(err, "azure put stream read")
	}
	return a.Put(ctx, key, data, contentType)
}

// ensure AzureBlobStorage implements Provider at compile time.
var _ Provider = (*AzureBlobStorage)(nil)
