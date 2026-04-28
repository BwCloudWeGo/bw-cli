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

	notev1 "github.com/BwCloudWeGo/bw-cli/api/gen/note/v1"
	notehandler "github.com/BwCloudWeGo/bw-cli/internal/note/handler"
	noterepo "github.com/BwCloudWeGo/bw-cli/internal/note/repo"
	noteservice "github.com/BwCloudWeGo/bw-cli/internal/note/service"
	"github.com/BwCloudWeGo/bw-cli/pkg/config"
	"github.com/BwCloudWeGo/bw-cli/pkg/database"
	"github.com/BwCloudWeGo/bw-cli/pkg/grpcx"
	"github.com/BwCloudWeGo/bw-cli/pkg/logger"
)

func main() {
	// Load service identity, database and logging settings from YAML/env.
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		panic(err)
	}
	cfg.Log.Service = cfg.App.NoteServiceName
	cfg.Log = logger.WithDailyFileName(cfg.Log, time.Now())

	log, err := logger.New(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	// Database.Open chooses SQLite, MySQL or PostgreSQL using the configured driver.
	db, err := database.Open(cfg.Database, cfg.MySQL, cfg.PostgreSQL, log)
	if err != nil {
		log.Fatal("open database failed", zap.Error(err))
	}
	if err := noterepo.AutoMigrate(db); err != nil {
		log.Fatal("migrate note database failed", zap.Error(err))
	}

	repo := noterepo.NewGormRepository(db, log)
	svc := noteservice.NewService(repo)
	server := grpc.NewServer(grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor(log)))
	notev1.RegisterNoteServiceServer(server, notehandler.NewServer(svc, log))

	addr := fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.GRPC.NotePort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("listen failed", zap.String("addr", addr), zap.Error(err))
	}

	go shutdownOnSignal(server, log)
	log.Info("note service listening", zap.String("addr", addr))
	if err := server.Serve(listener); err != nil {
		log.Fatal("note service stopped unexpectedly", zap.Error(err))
	}
}

func shutdownOnSignal(server *grpc.Server, log *zap.Logger) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Info("note service shutting down")
	server.GracefulStop()
}
