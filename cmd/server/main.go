package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"VkTest/internal/config"
	"VkTest/internal/logger"
	"VkTest/internal/server"
	"VkTest/internal/subpub"
	"VkTest/proto"
)

func main() {
	// Разбор флагов командной строки
	configPath := flag.String("config", "configs/config.yaml", "Path to configuration file")
	flag.Parse()

	// Загружаем конфигурацию
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Настройка логгера
	log, err := logger.NewLogger(cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	// Игнорируем ошибку при вызове Sync в defer
	defer func() { _ = log.Sync() }()

	// Dependency Injection: Создаем компоненты приложения
	sp := subpub.NewSubPub()
	pubSubServer := server.NewPubSubServer(cfg, sp, log)

	// Настраиваем и запускаем gRPC сервер
	address := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal("Failed to listen", zap.Error(err))
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPubSubServer(grpcServer, pubSubServer)
	
	// Включаем reflection для отладки с помощью grpcurl
	reflection.Register(grpcServer)

	// Graceful Shutdown: Настраиваем обработку сигналов для корректного завершения
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	// Запускаем сервер в отдельной горутине
	go func() {
		log.Info("Starting gRPC server", zap.String("address", address))
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatal("Failed to serve", zap.Error(err))
		}
	}()

	// Ждем сигнал остановки
	<-stopCh
	log.Info("Shutting down server...")

	// Graceful Shutdown: Останавливаем gRPC сервер
	grpcServer.GracefulStop()

	// Graceful Shutdown: Закрываем subpub с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sp.Close(ctx); err != nil {
		log.Error("Error closing SubPub", zap.Error(err))
	}

	log.Info("Server stopped gracefully")
}
