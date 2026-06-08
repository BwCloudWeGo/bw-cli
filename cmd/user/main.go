package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	userv1 "github.com/BwCloudWeGo/bw-cli/api/gen/user/v1"
	userhandler "github.com/BwCloudWeGo/bw-cli/internal/user/handler"
	userrepo "github.com/BwCloudWeGo/bw-cli/internal/user/repo"
	userservice "github.com/BwCloudWeGo/bw-cli/internal/user/service"
	"github.com/BwCloudWeGo/bw-cli/pkg/config"
	"github.com/BwCloudWeGo/bw-cli/pkg/database"
	"github.com/BwCloudWeGo/bw-cli/pkg/grpcx"
	"github.com/BwCloudWeGo/bw-cli/pkg/logger"
)

func main() {
	// 加载服务身份、数据库和日志配置。
	if err := config.InitGlobal("configs/config.yaml"); err != nil {
		panic(err)
	}
	cfg := config.MustGlobal()
	cfg.Log.Service = cfg.ServiceName("user")
	cfg.Log = logger.WithDailyFileName(cfg.Log, time.Now())

	log, err := logger.New(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer log.Sync()
	config.PrintSourceNotice(cfg, os.Stdout)

	// Database.Open 会根据配置的驱动选择 SQLite、MySQL 或 PostgreSQL。
	db, err := database.Open(cfg.Database, cfg.MySQL, cfg.PostgreSQL, log)
	if err != nil {
		log.Fatal("open database failed", zap.Error(err))
	}
	if err := userrepo.AutoMigrate(db); err != nil {
		log.Fatal("migrate user database failed", zap.Error(err))
	}

	repo := userrepo.NewGormRepository(db, log)
	svc := userservice.NewService(repo, userrepo.NewSHA256Hasher())
	server := grpc.NewServer(grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor(log)))
	userv1.RegisterUserServiceServer(server, userhandler.NewServer(svc, log))

	addr := fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.ServicePort("user", 9001))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("listen failed", zap.String("addr", addr), zap.Error(err))
	}

	go shutdownOnSignal(server, log)
	log.Info("user service listening", zap.String("addr", addr))
	if err := server.Serve(listener); err != nil {
		log.Fatal("user service stopped unexpectedly", zap.Error(err))
	}
}

func shutdownOnSignal(server *grpc.Server, log *zap.Logger) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Info("user service shutting down")
	server.GracefulStop()
}
