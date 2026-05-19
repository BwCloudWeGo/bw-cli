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
	User   userv1.UserServiceClient
	Note   notev1.NoteServiceClient
	Config *config.Config

	conns []*grpc.ClientConn
}

// New dials configured gRPC targets and builds typed service clients.
func New(cfg *config.Config, log *zap.Logger) (*Clients, error) {
	userTarget := cfg.ServiceTarget("user", 9001)
	noteTarget := cfg.ServiceTarget("note", 9002)
	userConn, err := grpc.Dial(userTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial user service: %w", err)
	}
	noteConn, err := grpc.Dial(noteTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		userConn.Close()
		return nil, fmt.Errorf("dial note service: %w", err)
	}

	log.Info("grpc clients initialized",
		zap.String("user_target", userTarget),
		zap.String("note_target", noteTarget),
	)
	return &Clients{
		User:   userv1.NewUserServiceClient(userConn),
		Note:   notev1.NewNoteServiceClient(noteConn),
		Config: cfg,
		conns:  []*grpc.ClientConn{userConn, noteConn},
	}, nil
}

// Close releases all gateway gRPC client connections.
func (c *Clients) Close() {
	for _, conn := range c.conns {
		_ = conn.Close()
	}
}
