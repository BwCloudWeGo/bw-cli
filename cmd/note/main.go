package main

import (
	"context"
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
	"github.com/BwCloudWeGo/bw-cli/pkg/grpcx"
	"github.com/BwCloudWeGo/bw-cli/pkg/logger"
	"github.com/BwCloudWeGo/bw-cli/pkg/mongox"
)

func main() {
	// Load service identity, database and logging settings from YAML/env.
	if err := config.InitGlobal("configs/config.yaml"); err != nil {
		panic(err)
	}
	cfg := config.MustGlobal()
	cfg.Log.Service = cfg.App.NoteServiceName
	cfg.Log = logger.WithDailyFileName(cfg.Log, time.Now())

	log, err := logger.New(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	//  1. 实例化mongodb
	mongoClient, err := mongox.NewClient(cfg.MongoDB.MongoxConfig())
	if err != nil {
		log.Fatal("create mongodb client failed", zap.Error(err))
	}
	defer disconnectMongo(mongoClient, log)
	// 2. 测试是否能调通
	if err := mongox.Ping(context.Background(), mongoClient); err != nil {
		log.Fatal("ping mongodb failed", zap.Error(err))
	}
	// 3. 设置mongodb要操作的数据库
	mongoDB := mongox.Database(mongoClient, cfg.MongoDB.Database)
	// 4. 在note服务中实例化 对mongo的操作类
	repo := noterepo.NewMongoRepository(mongoDB, log)
	// 5. 要让程序在service层能够调用上面的操作类
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

func disconnectMongo(client interface {
	Disconnect(context.Context) error
}, log *zap.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Disconnect(ctx); err != nil {
		log.Error("disconnect mongodb failed", zap.Error(err))
	}
}

func shutdownOnSignal(server *grpc.Server, log *zap.Logger) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Info("note service shutting down")
	server.GracefulStop()
}
