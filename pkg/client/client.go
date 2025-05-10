package client

import (
	"context"
	"io"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"VkTest/proto"
)

// PubSubClient представляет клиент для взаимодействия с сервисом PubSub
type PubSubClient struct {
	conn   *grpc.ClientConn
	client pb.PubSubClient
	logger *zap.Logger
}

// NewPubSubClient создает новый экземпляр PubSubClient
func NewPubSubClient(ctx context.Context, address string, logger *zap.Logger) (*PubSubClient, error) {
	// Если логгер не предоставлен, создаем по умолчанию
	if logger == nil {
		var err error
		logger, err = zap.NewProduction()
		if err != nil {
			return nil, err
		}
	}

	// Используем DialContext вместо устаревшего Dial
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &PubSubClient{
		conn:   conn,
		client: pb.NewPubSubClient(conn),
		logger: logger,
	}, nil
}

// Subscribe подписывается на события по ключу
func (c *PubSubClient) Subscribe(ctx context.Context, key string, callback func(data string)) error {
	req := &pb.SubscribeRequest{
		Key: key,
	}

	c.logger.Info("Subscribing to key", zap.String("key", key))
	stream, err := c.client.Subscribe(ctx, req)
	if err != nil {
		c.logger.Error("Failed to subscribe", zap.Error(err))
		return err
	}

	go func() {
		for {
			event, err := stream.Recv()
			if err == io.EOF {
				c.logger.Info("Stream closed")
				return
			}
			if err != nil {
				c.logger.Error("Error receiving event", zap.Error(err))
				return
			}

			c.logger.Debug("Received event", zap.String("data", event.GetData()))
			callback(event.GetData())
		}
	}()

	return nil
}

// Publish публикует сообщение по ключу
func (c *PubSubClient) Publish(ctx context.Context, key string, data string) error {
	req := &pb.PublishRequest{
		Key:  key,
		Data: data,
	}

	c.logger.Info("Publishing message", zap.String("key", key), zap.String("data", data))
	_, err := c.client.Publish(ctx, req)
	if err != nil {
		c.logger.Error("Failed to publish", zap.Error(err))
	}
	return err
}

// Close закрывает соединение с сервером
func (c *PubSubClient) Close() error {
	c.logger.Info("Closing client connection")
	return c.conn.Close()
}
