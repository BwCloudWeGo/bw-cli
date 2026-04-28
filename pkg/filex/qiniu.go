package filex

import (
	"context"
	"errors"
	"strings"

	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
)

type qiniuBackend struct {
	uploader *storage.FormUploader
	mac      *qbox.Mac
	cfg      QiniuConfig
}

// newQiniuBackend validates Qiniu Kodo configuration and prepares the form uploader.
func newQiniuBackend(cfg Config) (backend, error) {
	qiniuCfg := cfg.Qiniu
	if qiniuCfg.AccessKey == "" {
		return nil, errors.New("file storage qiniu access_key is required")
	}
	if qiniuCfg.SecretKey == "" {
		return nil, errors.New("file storage qiniu secret_key is required")
	}
	if qiniuCfg.Bucket == "" {
		return nil, errors.New("file storage qiniu bucket is required")
	}
	storageCfg := storage.Config{
		UseHTTPS:      qiniuCfg.UseHTTPS,
		UseCdnDomains: qiniuCfg.UseCdnDomains,
	}
	if qiniuCfg.Region != "" {
		if region, ok := storage.GetRegionByID(storage.RegionID(qiniuCfg.Region)); ok {
			storageCfg.Region = &region
		}
	}
	return &qiniuBackend{
		uploader: storage.NewFormUploader(&storageCfg),
		mac:      qbox.NewMac(qiniuCfg.AccessKey, qiniuCfg.SecretKey),
		cfg:      qiniuCfg,
	}, nil
}

func (b *qiniuBackend) Provider() string {
	return ProviderQiniu
}

func (b *qiniuBackend) Bucket() string {
	return b.cfg.Bucket
}

// Put uploads the object to Qiniu Kodo and applies provider-side size and MIME limits.
func (b *qiniuBackend) Put(ctx context.Context, req preparedUpload) (string, error) {
	policy := storage.PutPolicy{
		Scope:      b.cfg.Bucket + ":" + req.Key,
		FsizeLimit: req.MaxSizeByte,
		MimeLimit:  req.MimeLimit,
	}
	token := policy.UploadToken(b.mac)
	ret := storage.PutRet{}
	extra := storage.PutExtra{
		MimeType: req.Content,
		Params:   qiniuMetadata(req.Metadata),
	}
	if err := b.uploader.Put(ctx, &ret, token, req.Key, req.Reader, req.Size, &extra); err != nil {
		return "", err
	}
	return ret.Hash, nil
}

// qiniuMetadata normalizes custom metadata names to Qiniu's accepted prefixes.
func qiniuMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	values := make(map[string]string, len(metadata))
	for key, value := range metadata {
		key = strings.TrimSpace(key)
		if key == "" || value == "" {
			continue
		}
		if !strings.HasPrefix(key, "x-qn-meta-") && !strings.HasPrefix(key, "x:") {
			key = "x-qn-meta-" + key
		}
		values[key] = value
	}
	return values
}
