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
	// ProviderMinIO 将文件存储到 MinIO 或其他 S3 兼容端点。
	ProviderMinIO = "minio"
	// ProviderOSS 将文件存储到阿里云 OSS。
	ProviderOSS = "oss"
	// ProviderQiniu 将文件存储到七牛 Kodo。
	ProviderQiniu = "qiniu"
	// ProviderCOS 将文件存储到腾讯云 COS。
	ProviderCOS = "cos"
)

// Config 控制校验、对象命名和选中的存储提供商。
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

// MinIOConfig 包含 MinIO/S3 兼容上传配置。
type MinIOConfig struct {
	Endpoint        string `mapstructure:"endpoint" yaml:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id" yaml:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key" yaml:"secret_access_key"`
	Bucket          string `mapstructure:"bucket" yaml:"bucket"`
	Region          string `mapstructure:"region" yaml:"region"`
	UseSSL          bool   `mapstructure:"use_ssl" yaml:"use_ssl"`
}

// OSSConfig 包含阿里云 OSS 上传配置。
type OSSConfig struct {
	Endpoint        string `mapstructure:"endpoint" yaml:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id" yaml:"access_key_id"`
	AccessKeySecret string `mapstructure:"access_key_secret" yaml:"access_key_secret"`
	Bucket          string `mapstructure:"bucket" yaml:"bucket"`
}

// QiniuConfig 包含七牛 Kodo 上传配置。
type QiniuConfig struct {
	AccessKey     string `mapstructure:"access_key" yaml:"access_key"`
	SecretKey     string `mapstructure:"secret_key" yaml:"secret_key"`
	Bucket        string `mapstructure:"bucket" yaml:"bucket"`
	Region        string `mapstructure:"region" yaml:"region"`
	UseHTTPS      bool   `mapstructure:"use_https" yaml:"use_https"`
	UseCdnDomains bool   `mapstructure:"use_cdn_domains" yaml:"use_cdn_domains"`
}

// TencentCOSConfig 包含腾讯云 COS 上传配置。
type TencentCOSConfig struct {
	SecretID  string `mapstructure:"secret_id" yaml:"secret_id"`
	SecretKey string `mapstructure:"secret_key" yaml:"secret_key"`
	Bucket    string `mapstructure:"bucket" yaml:"bucket"`
	Region    string `mapstructure:"region" yaml:"region"`
	BucketURL string `mapstructure:"bucket_url" yaml:"bucket_url"`
}

// UploadRequest 描述一次上传操作。
type UploadRequest struct {
	Reader      io.Reader
	Filename    string
	ContentType string
	Size        int64
	ObjectKey   string
	Metadata    map[string]string
}

// UploadResult 是存储提供商成功保存文件后的返回结果。
type UploadResult struct {
	Provider    string `json:"provider"`
	Bucket      string `json:"bucket"`
	Key         string `json:"key"`
	URL         string `json:"url"`
	ETag        string `json:"etag"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

// Uploader 是应用服务使用的统一文件上传接口。
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

// DefaultConfig 返回适合常见业务文件的保守上传默认值。
func DefaultConfig() Config {
	return Config{
		Provider:            ProviderMinIO,
		MaxSizeMB:           100,
		ObjectPrefix:        "uploads",
		AllowedExtensions:   DefaultAllowedExtensions(),
		AllowedContentTypes: DefaultAllowedContentTypes(),
	}
}

// DefaultAllowedExtensions 返回常见文档、图片、视频和音频文件扩展名。
func DefaultAllowedExtensions() []string {
	return []string{
		".doc", ".docx", ".pdf",
		".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg",
		".mp4", ".mov", ".avi", ".mkv", ".webm",
		".mp3", ".wav", ".ogg", ".m4a", ".flac", ".aac",
	}
}

// DefaultAllowedContentTypes 返回常见文档、图片、视频和音频 MIME 类型。
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

// NewUploader 为 Config.Provider 选中的提供商创建上传器。
func NewUploader(cfg Config) (Uploader, error) {
	cfg = normalizeConfig(cfg)
	backend, err := newBackend(cfg)
	if err != nil {
		return nil, err
	}
	return &uploader{cfg: cfg, backend: backend}, nil
}

// Upload 校验文件，必要时创建对象 key，并使用选中的提供商存储文件。
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

// ValidateUpload 根据 Config 检查文件名、大小、扩展名和内容类型。
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

// DetectContentType 根据文件扩展名推断 MIME 类型。
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

// NewObjectKey 创建按日期分区的对象 key，并保留文件扩展名。
func NewObjectKey(prefix string, filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	key := time.Now().UTC().Format("2006/01/02") + "/" + uuid.NewString() + ext
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return key
	}
	return prefix + "/" + key
}

// normalizeConfig 合并调用方配置和框架默认值。
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

// maxSizeBytes 将人类可读的 MB 限制转换为提供商 SDK 使用的字节限制。
func maxSizeBytes(cfg Config) int64 {
	if cfg.MaxSizeMB <= 0 {
		cfg.MaxSizeMB = DefaultConfig().MaxSizeMB
	}
	return cfg.MaxSizeMB * 1024 * 1024
}

// normalizeExtensions 让扩展名匹配忽略大小写并统一带点前缀。
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

// normalizeContentTypes 准备精确匹配和通配匹配所需的 MIME 类型。
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

// normalizeContentType 去除 MIME 值中的 charset 等可选参数。
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

// containsFold 对扩展名执行忽略大小写的成员检查。
func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

// contentTypeAllowed 支持精确 MIME 匹配和 image/* 这类通配项。
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

// publicURL 在配置 CDN 或公开 bucket 域名时构建外部可访问 URL。
func publicURL(baseURL string, key string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return ""
	}
	return baseURL + "/" + strings.TrimLeft(key, "/")
}

// newBackend 根据配置选择具体对象存储提供商。
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
