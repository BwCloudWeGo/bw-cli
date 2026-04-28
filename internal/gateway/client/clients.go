package client

import (
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	notev1 "github.com/BwCloudWeGo/bw-cli/api/gen/note/v1"
	userv1 "github.com/BwCloudWeGo/bw-cli/api/gen/user/v1"
	"github.com/BwCloudWeGo/bw-cli/pkg/config"
)

// Clients groups all gRPC clients used by the HTTP gateway.
type Clients struct {
	User userv1.UserServiceClient
	Note notev1.NoteServiceClient

	conns []*grpc.ClientConn
}

// New dials configured gRPC targets and builds typed service clients.
func New(cfg config.GRPCConfig, log *zap.Logger) (*Clients, error) {
	userConn, err := grpc.Dial(cfg.UserTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial user service: %w", err)
	}
	noteConn, err := grpc.Dial(cfg.NoteTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		userConn.Close()
		return nil, fmt.Errorf("dial note service: %w", err)
	}

	log.Info("grpc clients initialized",
		zap.String("user_target", cfg.UserTarget),
		zap.String("note_target", cfg.NoteTarget),
	)
	return &Clients{
		User:  userv1.NewUserServiceClient(userConn),
		Note:  notev1.NewNoteServiceClient(noteConn),
		conns: []*grpc.ClientConn{userConn, noteConn},
	}, nil
}

// Close releases all gateway gRPC client connections.
func (c *Clients) Close() {
	for _, conn := range c.conns {
		_ = conn.Close()
	}
}
