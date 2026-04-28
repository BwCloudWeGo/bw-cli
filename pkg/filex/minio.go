package filex

import (
	"context"
	"errors"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type minioBackend struct {
	client *minio.Client
	cfg    MinIOConfig
}

// newMinIOBackend validates MinIO/S3 configuration and creates the SDK client.
func newMinIOBackend(cfg Config) (backend, error) {
	minioCfg := cfg.MinIO
	if minioCfg.Endpoint == "" {
		return nil, errors.New("file storage minio endpoint is required")
	}
	if minioCfg.AccessKeyID == "" {
		return nil, errors.New("file storage minio access_key_id is required")
	}
	if minioCfg.SecretAccessKey == "" {
		return nil, errors.New("file storage minio secret_access_key is required")
	}
	if minioCfg.Bucket == "" {
		return nil, errors.New("file storage minio bucket is required")
	}
	client, err := minio.New(minioCfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioCfg.AccessKeyID, minioCfg.SecretAccessKey, ""),
		Secure: minioCfg.UseSSL,
		Region: minioCfg.Region,
	})
	if err != nil {
		return nil, err
	}
	return &minioBackend{client: client, cfg: minioCfg}, nil
}

func (b *minioBackend) Provider() string {
	return ProviderMinIO
}

func (b *minioBackend) Bucket() string {
	return b.cfg.Bucket
}

// Put uploads the object to the configured MinIO bucket.
func (b *minioBackend) Put(ctx context.Context, req preparedUpload) (string, error) {
	info, err := b.client.PutObject(ctx, b.cfg.Bucket, req.Key, req.Reader, req.Size, minio.PutObjectOptions{
		ContentType:  req.Content,
		UserMetadata: req.Metadata,
	})
	if err != nil {
		return "", err
	}
	return info.ETag, nil
}
