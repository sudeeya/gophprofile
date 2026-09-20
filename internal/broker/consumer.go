package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"

	amqp "github.com/rabbitmq/amqp091-go"
	"golang.org/x/sync/errgroup"
)

type RabbitmqConsumerConfig struct {
	Host     string
	Port     string
	User     string
	Password string
}

type RabbitmqConsumer struct {
	conn          *amqp.Connection
	subscriptions []subscription
}

type subscription struct {
	queue   string
	consume func(ctx context.Context) error
}

func NewRabbitmqConsumer(config RabbitmqConsumerConfig) (*RabbitmqConsumer, error) {
	u := url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(config.User, config.Password),
		Host:   net.JoinHostPort(config.Host, config.Port),
	}

	conn, err := amqp.Dial(u.String())
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open channel: %w", err)
	}

	if err := declareTopology(ch); err != nil {
		return nil, fmt.Errorf("declare topology: %w", err)
	}

	return &RabbitmqConsumer{
		conn: conn,
	}, nil
}

func (r *RabbitmqConsumer) Close() error {
	return r.conn.Close()
}

func (r *RabbitmqConsumer) Register[T any](queue string, handler EventHandler[T]) {
	r.subscriptions = append(r.subscriptions, subscription{
		queue: queue,
		consume: func(ctx context.Context) error {
			return r.consume(ctx, queue, handler)
		},
	})
}

func (r *RabbitmqConsumer) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)

	for _, sub := range r.subscriptions {
		g.Go(func() error {
			if err := sub.consume(gctx); err != nil {
				return err
			}
			return nil
		})
	}

	return g.Wait()
}

type EventHandler[T any] func(ctx context.Context, event T) error

func (r *RabbitmqConsumer) consume[T any](ctx context.Context, queue string, handler EventHandler[T]) error {
	ch, err := r.conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer ch.Close()

	msgs, err := ch.ConsumeWithContext(ctx, queue, "worker", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume: %w", err)
	}

	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				return nil
			}

			var event T
			if err := json.Unmarshal(msg.Body, &event); err != nil {
				_ = msg.Nack(false, false)
				continue
			}

			if err := handler(ctx, event); err != nil {
				_ = msg.Nack(false, false)
			} else {
				_ = msg.Ack(false)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
