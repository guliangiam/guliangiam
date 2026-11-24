package storage

import (
	"bytes"
	"context"
	"io"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Storage stores objects in AWS S3 (or S3-compatible storage).
//
// 校准（OPEN-API-75 P7）：签名链由 aws-sdk-go-v2 实现 SigV4
// （派生密钥 SigningKey = HMAC 链 secret→date→region→service→aws4_request，
// Authorization: AWS4-HMAC-SHA256，已按 https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_sigv4.html
// 复核）。STS 临时凭据留待接入：本构造器用静态凭据，若需 STS 应改用
// config.WithCredentialsProvider(stscreds)。默认 virtual-host 寻址；path-style
// 仅当 endpoint 非空且提供方要求时经 newS3Storage 的 pathStyle 开启。
// region endpoint 现行规则（https://docs.aws.amazon.com/general/latest/gr/s3.html）：
// 标准区直接 s3.<region>.amazonaws.com，SDK 按 region 自动解析，无需自定义 endpoint。
type S3Storage struct {
	client    *s3.Client
	bucket    string
	presigner *s3.PresignClient
}

// NewS3Storage creates a new S3 storage provider.
func NewS3Storage(ctx context.Context, accessKey, secretKey, region, endpoint, bucket string) (Provider, error) {
	return newS3Storage(ctx, accessKey, secretKey, region, endpoint, bucket, false)
}

// newS3Storage builds an S3 client; pathStyle forces bucket-in-path addressing
// (required by MinIO and several S3-compatible endpoints).
func newS3Storage(ctx context.Context, accessKey, secretKey, region, endpoint, bucket string, pathStyle bool) (Provider, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "s3 load config")
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		if pathStyle {
			o.UsePathStyle = true
		}
	})

	return &S3Storage{
		client:    client,
		bucket:    bucket,
		presigner: s3.NewPresignClient(client),
	}, nil
}

func (s *S3Storage) Put(ctx context.Context, key string, data []byte, contentType string) error {
	if err := s.putObject(ctx, key, bytes.NewReader(data), contentType, aws.Int64(int64(len(data)))); err != nil {
		return apperrors.WrapInternalf(err, "s3 put")
	}
	return nil
}

func (s *S3Storage) Get(ctx context.Context, key string) ([]byte, error) {
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "s3 get")
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "s3 read body")
	}
	return data, nil
}

func (s *S3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return apperrors.WrapInternalf(err, "s3 delete")
	}
	return nil
}

func (s *S3Storage) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	resp, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "s3 list")
	}

	infos := make([]ObjectInfo, 0, len(resp.Contents))
	for _, obj := range resp.Contents {
		infos = append(infos, ObjectInfo{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
		})
	}
	return infos, nil
}

func (s *S3Storage) GetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	req, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expires
	})
	if err != nil {
		return "", apperrors.WrapInternalf(err, "s3 presign")
	}
	return req.URL, nil
}

func (s *S3Storage) PutStream(ctx context.Context, key string, reader io.Reader, contentType string) error {
	if err := s.putObject(ctx, key, reader, contentType, nil); err != nil {
		return apperrors.WrapInternalf(err, "s3 put stream")
	}
	return nil
}

// putObject is shared by Put and PutStream. contentLength is optional: when nil
// the SDK streams the reader and derives length from a seekable body.
func (s *S3Storage) putObject(ctx context.Context, key string, body io.Reader, contentType string, contentLength *int64) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentType:   aws.String(contentType),
		ContentLength: contentLength,
	})
	return err
}

// ensure S3Storage implements Provider at compile time.
var _ Provider = (*S3Storage)(nil)

// unused import guard for types package
var _ types.Object
