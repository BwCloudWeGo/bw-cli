package handler

import (
	"context"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/metadata"

	"github.com/BwCloudWeGo/bw-cli/pkg/grpcx"
	"github.com/BwCloudWeGo/bw-cli/pkg/httpx"
)

// outgoingContext 将请求 ID 等 gateway 元数据转发到下游 gRPC 调用。
func outgoingContext(c *gin.Context) context.Context {
	return metadata.AppendToOutgoingContext(c.Request.Context(), grpcx.MetadataRequestID, httpx.RequestID(c))
}
