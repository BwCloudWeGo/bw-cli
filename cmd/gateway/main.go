package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/internal/gateway/client"
	"github.com/BwCloudWeGo/bw-cli/internal/gateway/router"
	"github.com/BwCloudWeGo/bw-cli/pkg/config"
	"github.com/BwCloudWeGo/bw-cli/pkg/logger"
	"github.com/BwCloudWeGo/bw-cli/pkg/observability"
)

func main() {
	// Load all runtime settings from YAML/env before constructing dependencies.
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		panic(err)
	}
	cfg.Log.Service = cfg.App.GatewayServiceName
	cfg.Log = logger.WithDailyFileName(cfg.Log, time.Now())

	log, err := logger.New(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer log.Sync()
	observability.Register(cfg.App.GatewayServiceName, log)

	// gRPC targets are read from configuration so deployments can change them without recompilation.
	clients, err := client.New(cfg.GRPC, log)
	if err != nil {
		log.Fatal("initialize grpc clients failed", zap.Error(err))
	}
	defer clients.Close()

	engine := router.New(clients, log, cfg.Middleware)
	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      engine,
		ReadTimeout:  time.Duration(cfg.HTTP.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.HTTP.WriteTimeoutSeconds) * time.Second,
	}

	go func() {
		log.Info("gateway listening", zap.String("addr", addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("gateway stopped unexpectedly", zap.Error(err))
		}
	}()

	waitForShutdown(server, log)
}

func waitForShutdown(server *http.Server, log *zap.Logger) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Info("gateway shutting down")
	if err := server.Shutdown(ctx); err != nil {
		log.Error("gateway shutdown failed", zap.Error(err))
	}
}
