package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
	tracer        trace.Tracer
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

	tracer := otel.Tracer("github.com/sudeeya/gophprofile/internal/broker")

	return &RabbitmqConsumer{
		conn:   conn,
		tracer: tracer,
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
			r.handleMessage(ctx, queue, msg, handler)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (r *RabbitmqConsumer) handleMessage[T any](ctx context.Context, queue string, msg amqp.Delivery, handler EventHandler[T]) {
	ctx = otel.GetTextMapPropagator().Extract(ctx, AMQPTableCarrier{Table: msg.Headers})

	var (
		operation   = "receive"
		destination = fmt.Sprintf("%s:%s", ExchangeAvatar, queue)
		spanName    = fmt.Sprintf("%s %s", operation, destination)
	)

	ctx, span := r.tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.operation.name", operation),
			attribute.String("messaging.operation.type", "receive"),
			attribute.String("messaging.destination.name", destination),
			attribute.String("messaging.message.id", msg.MessageId),
		),
	)
	defer span.End()

	var event T
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		span.SetStatus(codes.Error, err.Error())
		_ = msg.Nack(false, false)
		return
	}

	if err := handler(ctx, event); err != nil {
		span.SetStatus(codes.Error, err.Error())
		_ = msg.Nack(false, false)
		return
	}

	_ = msg.Ack(false)
}
