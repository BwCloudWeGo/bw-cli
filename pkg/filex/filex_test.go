package filex_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/filex"
)

func TestDefaultConfigSupportsCommonBusinessFileTypes(t *testing.T) {
	cfg := filex.DefaultConfig()

	require.Contains(t, cfg.AllowedExtensions, ".doc")
	require.Contains(t, cfg.AllowedExtensions, ".docx")
	require.Contains(t, cfg.AllowedExtensions, ".pdf")
	require.Contains(t, cfg.AllowedExtensions, ".jpg")
	require.Contains(t, cfg.AllowedExtensions, ".png")
	require.Contains(t, cfg.AllowedExtensions, ".mp4")
	require.Contains(t, cfg.AllowedExtensions, ".mp3")
	require.Contains(t, cfg.AllowedContentTypes, "application/pdf")
	require.Contains(t, cfg.AllowedContentTypes, "image/jpeg")
	require.Contains(t, cfg.AllowedContentTypes, "video/mp4")
	require.Contains(t, cfg.AllowedContentTypes, "audio/mpeg")
	require.Equal(t, int64(100), cfg.MaxSizeMB)
}

func TestValidateUploadRejectsOversizedFile(t *testing.T) {
	cfg := filex.DefaultConfig()
	cfg.MaxSizeMB = 1

	err := filex.ValidateUpload(cfg, filex.UploadRequest{
		Filename:    "report.pdf",
		ContentType: "application/pdf",
		Size:        2 * 1024 * 1024,
	})

	require.EqualError(t, err, "file size exceeds limit: max 1 MB")
}

func TestValidateUploadRejectsUnsupportedExtension(t *testing.T) {
	cfg := filex.DefaultConfig()

	err := filex.ValidateUpload(cfg, filex.UploadRequest{
		Filename:    "shell.sh",
		ContentType: "text/x-shellscript",
		Size:        128,
	})

	require.EqualError(t, err, "unsupported file extension \".sh\"")
}

func TestValidateUploadInfersContentTypeFromExtension(t *testing.T) {
	cfg := filex.DefaultConfig()

	err := filex.ValidateUpload(cfg, filex.UploadRequest{
		Filename: "avatar.png",
		Size:     128,
	})

	require.NoError(t, err)
}

func TestNewObjectKeyKeepsPrefixAndExtension(t *testing.T) {
	key := filex.NewObjectKey("uploads/demo", "季度 报告.PDF")

	require.True(t, strings.HasPrefix(key, "uploads/demo/"))
	require.True(t, strings.HasSuffix(key, ".pdf"))
	require.NotContains(t, key, " ")
}

func TestNewUploaderRejectsUnknownProvider(t *testing.T) {
	cfg := filex.DefaultConfig()
	cfg.Provider = "ftp"

	uploader, err := filex.NewUploader(cfg)

	require.Nil(t, uploader)
	require.EqualError(t, err, "unsupported file storage provider \"ftp\"")
}

func TestNewUploaderRejectsIncompleteMinIOConfig(t *testing.T) {
	cfg := filex.DefaultConfig()
	cfg.Provider = filex.ProviderMinIO
	cfg.MinIO.Endpoint = ""

	uploader, err := filex.NewUploader(cfg)

	require.Nil(t, uploader)
	require.EqualError(t, err, "file storage minio endpoint is required")
}
