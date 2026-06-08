package grpcx

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
)

// MetadataRequestID 是通过 gRPC 透传请求 ID 的元数据键。
const MetadataRequestID = "x-request-id"

// UnaryServerInterceptor 将应用错误映射为 gRPC status 错误，并记录 RPC 维度日志。
func UnaryServerInterceptor(log *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		if err != nil {
			err = apperrors.ToGRPC(err)
		}

		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}
		log.Info("grpc request completed",
			zap.String("request_id", RequestIDFromContext(ctx)),
			zap.String("trace_id", firstMetadata(ctx, "traceparent")),
			zap.String("method", info.FullMethod),
			zap.String("peer", peerAddress(ctx)),
			zap.String("status_code", code.String()),
			zap.String("error_code", errorCode(err)),
			zap.Float64("latency_ms", float64(time.Since(start).Microseconds())/1000),
		)
		return resp, err
	}
}

// UnaryClientInterceptor 将当前 HTTP 请求 ID 添加到出站 gRPC 调用。
func UnaryClientInterceptor(requestID string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req interface{}, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if requestID != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, MetadataRequestID, requestID)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// RequestIDFromContext 从入站元数据中提取透传的请求 ID。
func RequestIDFromContext(ctx context.Context) string {
	return firstMetadata(ctx, MetadataRequestID)
}

func firstMetadata(ctx context.Context, key string) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func peerAddress(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return ""
	}
	return p.Addr.String()
}

func errorCode(err error) string {
	if err == nil {
		return ""
	}
	appErr := apperrors.FromGRPC(err)
	if appErr == nil {
		return ""
	}
	return appErr.Code
}
