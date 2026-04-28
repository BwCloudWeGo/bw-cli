package filex

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	cos "github.com/tencentyun/cos-go-sdk-v5"
)

type cosBackend struct {
	client *cos.Client
	cfg    TencentCOSConfig
}

// newTencentCOSBackend validates Tencent Cloud COS configuration and builds the bucket URL.
func newTencentCOSBackend(cfg Config) (backend, error) {
	cosCfg := cfg.COS
	if cosCfg.SecretID == "" {
		return nil, errors.New("file storage cos secret_id is required")
	}
	if cosCfg.SecretKey == "" {
		return nil, errors.New("file storage cos secret_key is required")
	}
	if cosCfg.Bucket == "" {
		return nil, errors.New("file storage cos bucket is required")
	}
	if cosCfg.Region == "" && cosCfg.BucketURL == "" {
		return nil, errors.New("file storage cos region or bucket_url is required")
	}
	bucketURL := cosCfg.BucketURL
	if bucketURL == "" {
		bucketURL = "https://" + cosCfg.Bucket + ".cos." + cosCfg.Region + ".myqcloud.com"
	}
	parsedURL, err := url.Parse(bucketURL)
	if err != nil {
		return nil, err
	}
	client := cos.NewClient(&cos.BaseURL{BucketURL: parsedURL}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cosCfg.SecretID,
			SecretKey: cosCfg.SecretKey,
		},
	})
	return &cosBackend{client: client, cfg: cosCfg}, nil
}

func (b *cosBackend) Provider() string {
	return ProviderCOS
}

func (b *cosBackend) Bucket() string {
	return b.cfg.Bucket
}

// Put uploads the object to Tencent Cloud COS with content type and optional metadata.
func (b *cosBackend) Put(ctx context.Context, req preparedUpload) (string, error) {
	meta := http.Header{}
	for key, value := range req.Metadata {
		key = strings.TrimSpace(strings.ToLower(key))
		if key == "" || value == "" {
			continue
		}
		if !strings.HasPrefix(key, "x-cos-meta-") {
			key = "x-cos-meta-" + key
		}
		meta.Set(key, value)
	}
	resp, err := b.client.Object.Put(ctx, req.Key, req.Reader, &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType:   req.Content,
			ContentLength: req.Size,
			XCosMetaXXX:   &meta,
		},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return strings.Trim(resp.Header.Get("ETag"), `"`), nil
}
