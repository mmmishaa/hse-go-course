package rabbitmq

import (
	"clinic-service/internal/config"
	"context"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Client struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	log     *slog.Logger
}

func NewRabbitMQ(cfg *config.RabbitMQConfig, ctx context.Context, log *slog.Logger) (*Client, error) {
	url := fmt.Sprintf("amqp://%s:%s@%s:%s%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.VHost)

	var conn *amqp.Connection
	var err error

	for i := 0; i < 30; i++ {
		conn, err = amqp.Dial(url)
		if err == nil {
			break
		}
		log.Warn("waiting for RabbitMQ", slog.Int("attempt", i+1))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("channel: %w", err)
	}

	queues := []string{"appointments.new", "appointments.status"}
	for _, q := range queues {
		if _, err := ch.QueueDeclare(q, true, false, false, false, nil); err != nil {
			return nil, fmt.Errorf("queue %s: %w", q, err)
		}
	}

	log.Info("connected to RabbitMQ")

	return &Client{conn: conn, channel: ch, log: log}, nil
}

func (c *Client) Publish(queue string, body []byte) error {
	return c.channel.PublishWithContext(context.Background(),
		"",
		queue,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

func (c *Client) Consume(queue string) (<-chan amqp.Delivery, error) {
	return c.channel.Consume(
		queue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
}

func (c *Client) Close() {
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}