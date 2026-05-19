package handler

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/metadata"

	"github.com/BwCloudWeGo/bw-cli/pkg/grpcx"
	"github.com/BwCloudWeGo/bw-cli/pkg/httpx"
)

// outgoingContext forwards gateway metadata such as request id to downstream gRPC calls.
func outgoingContext(c *gin.Context) context.Context {
	return metadata.AppendToOutgoingContext(c.Request.Context(), grpcx.MetadataRequestID, httpx.RequestID(c))
}

func gatewayGRPCTarget(envName string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(envName))
	if value == "" {
		return fallback
	}
	return value
}

func configuredGatewayGRPCTarget(envName string, target string, port int) string {
	value := strings.TrimSpace(os.Getenv(envName))
	if value != "" {
		return value
	}
	if strings.TrimSpace(target) != "" {
		return target
	}
	if port > 0 {
		return "127.0.0.1:" + strconv.Itoa(port)
	}
	return ""
}
