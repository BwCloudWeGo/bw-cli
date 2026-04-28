package filex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// ProviderMinIO stores files in MinIO or another S3-compatible endpoint.
	ProviderMinIO = "minio"
	// ProviderOSS stores files in Alibaba Cloud OSS.
	ProviderOSS = "oss"
	// ProviderQiniu stores files in Qiniu Kodo.
	ProviderQiniu = "qiniu"
	// ProviderCOS stores files in Tencent Cloud COS.
	ProviderCOS = "cos"
)

// Config controls validation, object naming and the selected storage provider.
type Config struct {
	Provider            string           `mapstructure:"provider" yaml:"provider"`
	MaxSizeMB           int64            `mapstructure:"max_size_mb" yaml:"max_size_mb"`
	ObjectPrefix        string           `mapstructure:"object_prefix" yaml:"object_prefix"`
	PublicBaseURL       string           `mapstructure:"public_base_url" yaml:"public_base_url"`
	AllowedExtensions   []string         `mapstructure:"allowed_extensions" yaml:"allowed_extensions"`
	AllowedContentTypes []string         `mapstructure:"allowed_content_types" yaml:"allowed_content_types"`
	MinIO               MinIOConfig      `mapstructure:"minio" yaml:"minio"`
	OSS                 OSSConfig        `mapstructure:"oss" yaml:"oss"`
	Qiniu               QiniuConfig      `mapstructure:"qiniu" yaml:"qiniu"`
	COS                 TencentCOSConfig `mapstructure:"cos" yaml:"cos"`
}

// MinIOConfig contains MinIO/S3-compatible upload settings.
type MinIOConfig struct {
	Endpoint        string `mapstructure:"endpoint" yaml:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id" yaml:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key" yaml:"secret_access_key"`
	Bucket          string `mapstructure:"bucket" yaml:"bucket"`
	Region          string `mapstructure:"region" yaml:"region"`
	UseSSL          bool   `mapstructure:"use_ssl" yaml:"use_ssl"`
}

// OSSConfig contains Alibaba Cloud OSS upload settings.
type OSSConfig struct {
	Endpoint        string `mapstructure:"endpoint" yaml:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id" yaml:"access_key_id"`
	AccessKeySecret string `mapstructure:"access_key_secret" yaml:"access_key_secret"`
	Bucket          string `mapstructure:"bucket" yaml:"bucket"`
}

// QiniuConfig contains Qiniu Kodo upload settings.
type QiniuConfig struct {
	AccessKey     string `mapstructure:"access_key" yaml:"access_key"`
	SecretKey     string `mapstructure:"secret_key" yaml:"secret_key"`
	Bucket        string `mapstructure:"bucket" yaml:"bucket"`
	Region        string `mapstructure:"region" yaml:"region"`
	UseHTTPS      bool   `mapstructure:"use_https" yaml:"use_https"`
	UseCdnDomains bool   `mapstructure:"use_cdn_domains" yaml:"use_cdn_domains"`
}

// TencentCOSConfig contains Tencent Cloud COS upload settings.
type TencentCOSConfig struct {
	SecretID  string `mapstructure:"secret_id" yaml:"secret_id"`
	SecretKey string `mapstructure:"secret_key" yaml:"secret_key"`
	Bucket    string `mapstructure:"bucket" yaml:"bucket"`
	Region    string `mapstructure:"region" yaml:"region"`
	BucketURL string `mapstructure:"bucket_url" yaml:"bucket_url"`
}

// UploadRequest describes one upload operation.
type UploadRequest struct {
	Reader      io.Reader
	Filename    string
	ContentType string
	Size        int64
	ObjectKey   string
	Metadata    map[string]string
}

// UploadResult is returned after a provider successfully stores the file.
type UploadResult struct {
	Provider    string `json:"provider"`
	Bucket      string `json:"bucket"`
	Key         string `json:"key"`
	URL         string `json:"url"`
	ETag        string `json:"etag"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

// Uploader is the unified file upload interface used by application services.
type Uploader interface {
	Upload(ctx context.Context, req UploadRequest) (UploadResult, error)
}

type backend interface {
	Provider() string
	Bucket() string
	Put(ctx context.Context, req preparedUpload) (string, error)
}

type preparedUpload struct {
	UploadRequest
	Key         string
	Content     string
	Bucket      string
	Provider    string
	PublicURL   string
	MaxSizeByte int64
	MimeLimit   string
}

type uploader struct {
	cfg     Config
	backend backend
}

// DefaultConfig returns conservative upload defaults for common business files.
func DefaultConfig() Config {
	return Config{
		Provider:            ProviderMinIO,
		MaxSizeMB:           100,
		ObjectPrefix:        "uploads",
		AllowedExtensions:   DefaultAllowedExtensions(),
		AllowedContentTypes: DefaultAllowedContentTypes(),
	}
}

// DefaultAllowedExtensions returns common document, image, video and audio file extensions.
func DefaultAllowedExtensions() []string {
	return []string{
		".doc", ".docx", ".pdf",
		".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg",
		".mp4", ".mov", ".avi", ".mkv", ".webm",
		".mp3", ".wav", ".ogg", ".m4a", ".flac", ".aac",
	}
}

// DefaultAllowedContentTypes returns common document, image, video and audio MIME types.
func DefaultAllowedContentTypes() []string {
	return []string{
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/pdf",
		"image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp", "image/svg+xml",
		"video/mp4", "video/quicktime", "video/x-msvideo", "video/x-matroska", "video/webm",
		"audio/mpeg", "audio/wav", "audio/x-wav", "audio/ogg", "audio/mp4", "audio/flac", "audio/aac",
	}
}

// NewUploader creates an uploader for the provider selected by Config.Provider.
func NewUploader(cfg Config) (Uploader, error) {
	cfg = normalizeConfig(cfg)
	backend, err := newBackend(cfg)
	if err != nil {
		return nil, err
	}
	return &uploader{cfg: cfg, backend: backend}, nil
}

// Upload validates a file, creates an object key when needed, and stores it with the selected provider.
func (u *uploader) Upload(ctx context.Context, req UploadRequest) (UploadResult, error) {
	if req.Reader == nil {
		return UploadResult{}, errors.New("file reader is required")
	}
	if err := ValidateUpload(u.cfg, req); err != nil {
		return UploadResult{}, err
	}
	contentType := normalizeContentType(req.ContentType)
	if contentType == "" {
		contentType = DetectContentType(req.Filename)
	}
	key := strings.TrimLeft(req.ObjectKey, "/")
	if key == "" {
		key = NewObjectKey(u.cfg.ObjectPrefix, req.Filename)
	}

	prepared := preparedUpload{
		UploadRequest: req,
		Key:           key,
		Content:       contentType,
		Bucket:        u.backend.Bucket(),
		Provider:      u.backend.Provider(),
		PublicURL:     publicURL(u.cfg.PublicBaseURL, key),
		MaxSizeByte:   maxSizeBytes(u.cfg),
		MimeLimit:     strings.Join(u.cfg.AllowedContentTypes, ";"),
	}
	etag, err := u.backend.Put(ctx, prepared)
	if err != nil {
		return UploadResult{}, err
	}
	return UploadResult{
		Provider:    prepared.Provider,
		Bucket:      prepared.Bucket,
		Key:         prepared.Key,
		URL:         prepared.PublicURL,
		ETag:        etag,
		Size:        req.Size,
		ContentType: prepared.Content,
	}, nil
}

// ValidateUpload checks file name, size, extension and content type against Config.
func ValidateUpload(cfg Config, req UploadRequest) error {
	cfg = normalizeConfig(cfg)
	if strings.TrimSpace(req.Filename) == "" {
		return errors.New("file name is required")
	}
	if req.Size <= 0 {
		return errors.New("file size must be greater than 0")
	}
	if req.Size > maxSizeBytes(cfg) {
		return fmt.Errorf("file size exceeds limit: max %d MB", cfg.MaxSizeMB)
	}
	ext := strings.ToLower(filepath.Ext(req.Filename))
	if ext == "" {
		return errors.New("file extension is required")
	}
	if !containsFold(cfg.AllowedExtensions, ext) {
		return fmt.Errorf("unsupported file extension %q", ext)
	}
	contentType := normalizeContentType(req.ContentType)
	if contentType == "" {
		contentType = DetectContentType(req.Filename)
	}
	if contentType == "" {
		return errors.New("file content type is required")
	}
	if !contentTypeAllowed(cfg.AllowedContentTypes, contentType) {
		return fmt.Errorf("unsupported file content type %q", contentType)
	}
	return nil
}

// DetectContentType infers a MIME type from a file extension.
func DetectContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".mkv":
		return "video/x-matroska"
	case ".m4a":
		return "audio/mp4"
	}
	return normalizeContentType(mime.TypeByExtension(ext))
}

// NewObjectKey creates a date-partitioned object key and preserves the file extension.
func NewObjectKey(prefix string, filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	key := time.Now().UTC().Format("2006/01/02") + "/" + uuid.NewString() + ext
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return key
	}
	return prefix + "/" + key
}

// normalizeConfig merges caller configuration with framework defaults.
func normalizeConfig(cfg Config) Config {
	defaults := DefaultConfig()
	if cfg.Provider == "" {
		cfg.Provider = defaults.Provider
	}
	if cfg.MaxSizeMB <= 0 {
		cfg.MaxSizeMB = defaults.MaxSizeMB
	}
	if len(cfg.AllowedExtensions) == 0 {
		cfg.AllowedExtensions = defaults.AllowedExtensions
	}
	if len(cfg.AllowedContentTypes) == 0 {
		cfg.AllowedContentTypes = defaults.AllowedContentTypes
	}
	if cfg.ObjectPrefix == "" {
		cfg.ObjectPrefix = defaults.ObjectPrefix
	}
	cfg.Provider = strings.ToLower(strings.TrimSpace(cfg.Provider))
	cfg.AllowedExtensions = normalizeExtensions(cfg.AllowedExtensions)
	cfg.AllowedContentTypes = normalizeContentTypes(cfg.AllowedContentTypes)
	return cfg
}

// maxSizeBytes converts the human-readable MB limit into provider SDK byte limits.
func maxSizeBytes(cfg Config) int64 {
	if cfg.MaxSizeMB <= 0 {
		cfg.MaxSizeMB = DefaultConfig().MaxSizeMB
	}
	return cfg.MaxSizeMB * 1024 * 1024
}

// normalizeExtensions makes extension matching case-insensitive and dot-prefixed.
func normalizeExtensions(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if !strings.HasPrefix(value, ".") {
			value = "." + value
		}
		out = append(out, value)
	}
	return out
}

// normalizeContentTypes prepares MIME types for exact and wildcard matching.
func normalizeContentTypes(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = normalizeContentType(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

// normalizeContentType strips optional parameters such as charset from MIME values.
func normalizeContentType(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err == nil {
		return mediaType
	}
	if i := strings.Index(value, ";"); i >= 0 {
		return strings.TrimSpace(value[:i])
	}
	return value
}

// containsFold performs case-insensitive membership checks for extensions.
func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

// contentTypeAllowed supports exact MIME matches and wildcard entries such as image/*.
func contentTypeAllowed(allowed []string, contentType string) bool {
	contentType = normalizeContentType(contentType)
	for _, item := range allowed {
		item = normalizeContentType(item)
		if item == contentType {
			return true
		}
		if strings.HasSuffix(item, "/*") && strings.HasPrefix(contentType, strings.TrimSuffix(item, "*")) {
			return true
		}
	}
	return false
}

// publicURL builds the externally accessible URL when a CDN or public bucket domain is configured.
func publicURL(baseURL string, key string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return ""
	}
	return baseURL + "/" + strings.TrimLeft(key, "/")
}

// newBackend selects the concrete object storage provider from configuration.
func newBackend(cfg Config) (backend, error) {
	switch cfg.Provider {
	case ProviderMinIO, "s3":
		return newMinIOBackend(cfg)
	case ProviderOSS, "aliyun", "aliyun_oss":
		return newOSSBackend(cfg)
	case ProviderQiniu, "kodo":
		return newQiniuBackend(cfg)
	case ProviderCOS, "tencent", "tencent_cos":
		return newTencentCOSBackend(cfg)
	default:
		return nil, fmt.Errorf("unsupported file storage provider %q", cfg.Provider)
	}
}
