package filex

import (
	"context"
	"errors"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

type ossBackend struct {
	bucket *oss.Bucket
	cfg    OSSConfig
}

// newOSSBackend validates Alibaba Cloud OSS configuration and resolves the target bucket.
func newOSSBackend(cfg Config) (backend, error) {
	ossCfg := cfg.OSS
	if ossCfg.Endpoint == "" {
		return nil, errors.New("file storage oss endpoint is required")
	}
	if ossCfg.AccessKeyID == "" {
		return nil, errors.New("file storage oss access_key_id is required")
	}
	if ossCfg.AccessKeySecret == "" {
		return nil, errors.New("file storage oss access_key_secret is required")
	}
	if ossCfg.Bucket == "" {
		return nil, errors.New("file storage oss bucket is required")
	}
	client, err := oss.New(ossCfg.Endpoint, ossCfg.AccessKeyID, ossCfg.AccessKeySecret)
	if err != nil {
		return nil, err
	}
	bucket, err := client.Bucket(ossCfg.Bucket)
	if err != nil {
		return nil, err
	}
	return &ossBackend{bucket: bucket, cfg: ossCfg}, nil
}

func (b *ossBackend) Provider() string {
	return ProviderOSS
}

func (b *ossBackend) Bucket() string {
	return b.cfg.Bucket
}

// Put uploads the object to Alibaba Cloud OSS with content type and optional metadata.
func (b *ossBackend) Put(ctx context.Context, req preparedUpload) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	options := []oss.Option{oss.ContentType(req.Content)}
	for key, value := range req.Metadata {
		key = strings.TrimSpace(key)
		if key == "" || value == "" {
			continue
		}
		options = append(options, oss.Meta(key, value))
	}
	if err := b.bucket.PutObject(req.Key, req.Reader, options...); err != nil {
		return "", err
	}
	return "", nil
}
