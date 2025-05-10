package server

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"VkTest/internal/config"
	"VkTest/internal/subpub"
	pb "VkTest/proto"
)

// PubSubServer реализует gRPC сервис PubSub
type PubSubServer struct {
	pb.UnimplementedPubSubServer
	cfg     *config.Config
	sp      subpub.SubPub
	logger  *zap.Logger
	clients sync.Map // Для отслеживания активных клиентов
}

// NewPubSubServer создает новый экземпляр PubSubServer
func NewPubSubServer(cfg *config.Config, sp subpub.SubPub, logger *zap.Logger) *PubSubServer {
	return &PubSubServer{
		cfg:    cfg,
		sp:     sp,
		logger: logger,
	}
}

// Subscribe реализует метод подписки на события
func (s *PubSubServer) Subscribe(req *pb.SubscribeRequest, stream pb.PubSub_SubscribeServer) error {
	if req.Key == "" {
		s.logger.Warn("Empty key in subscription request")
		return status.Error(codes.InvalidArgument, "key is required")
	}

	s.logger.Info("New subscription",
		zap.String("key", req.Key),
		zap.String("client_id", fmt.Sprintf("%p", stream)))

	// Создаем канал для сообщений
	msgCh := make(chan string, 100)
	clientID := fmt.Sprintf("%p", stream) // Уникальный ID для клиента
	s.clients.Store(clientID, msgCh)

	// Подписываемся на события с использованием subpub
	sub, err := s.sp.Subscribe(req.Key, func(msg interface{}) {
		// Обрабатываем сообщения и отправляем в канал
		if data, ok := msg.(string); ok {
			select {
			case msgCh <- data:
				// сообщение отправлено
			default:
				// канал заполнен, пропускаем
			}
		}
	})

	if err != nil {
		s.clients.Delete(clientID)
		close(msgCh)
		s.logger.Error("Failed to subscribe",
			zap.String("key", req.Key),
			zap.Error(err))
		return status.Error(codes.Internal, "failed to subscribe")
	}

	// Отписываемся при завершении
	defer func() {
		sub.Unsubscribe()
		s.clients.Delete(clientID)
		close(msgCh)
		s.logger.Info("Subscription ended", zap.String("key", req.Key))
	}()

	// Отправляем сообщения клиенту
	for {
		select {
		case data := <-msgCh:
			event := &pb.Event{
				Data: data,
			}
			if err := stream.Send(event); err != nil {
				s.logger.Error("Failed to send event",
					zap.String("key", req.Key),
					zap.Error(err))
				return status.Error(codes.Internal, "failed to send event")
			}
		case <-stream.Context().Done():
			// Клиент отключился
			s.logger.Info("Client disconnected", zap.String("key", req.Key))
			return stream.Context().Err()
		}
	}
}

// Publish реализует метод публикации событий
func (s *PubSubServer) Publish(ctx context.Context, req *pb.PublishRequest) (*emptypb.Empty, error) {
	if req.Key == "" {
		s.logger.Warn("Empty key in publish request")
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}

	s.logger.Info("Publishing message",
		zap.String("key", req.Key),
		zap.String("data_length", fmt.Sprintf("%d bytes", len(req.Data))))

	err := s.sp.Publish(req.Key, req.Data)
	if err != nil {
		s.logger.Error("Failed to publish message",
			zap.String("key", req.Key),
			zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to publish message")
	}

	return &emptypb.Empty{}, nil
}
