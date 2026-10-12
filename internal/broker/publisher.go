package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"time"
	"uuid"

	"github.com/avast/retry-go/v5"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type RabbitmqPublisherConfig struct {
	Host                           string
	Port                           string
	User                           string
	Password                       string
	PublisherConfirmsRetryDelay    time.Duration
	PublisherConfirmsRetryAttempts uint
}

type RabbitmqPublisher struct {
	conn    *amqp.Connection
	ch      *amqp.Channel
	retrier *retry.Retrier
	tracer  trace.Tracer
}

func NewRabbitmqPublisher(config RabbitmqPublisherConfig) (*RabbitmqPublisher, error) {
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

	if err := ch.Confirm(false); err != nil {
		return nil, fmt.Errorf("enable confirm: %w", err)
	}

	if err := declareTopology(ch); err != nil {
		return nil, fmt.Errorf("declare topology: %w", err)
	}

	retrier := retry.New(
		retry.Attempts(config.PublisherConfirmsRetryAttempts),
		retry.Delay(config.PublisherConfirmsRetryDelay),
		retry.DelayType(retry.CombineDelay(retry.BackOffDelay, retry.RandomDelay)),
		retry.LastErrorOnly(true),
	)

	tracer := otel.Tracer("github.com/sudeeya/gophprofile/internal/broker")

	return &RabbitmqPublisher{
		conn:    conn,
		ch:      ch,
		retrier: retrier,
		tracer:  tracer,
	}, nil
}

func (r *RabbitmqPublisher) Close() error {
	_ = r.ch.Close()
	return r.conn.Close()
}

func (r *RabbitmqPublisher) Ping(_ context.Context) error {
	if r.conn.IsClosed() {
		return ErrConnClosed
	}
	return nil
}

func (r *RabbitmqPublisher) PublishAvatarUploadEvent(ctx context.Context, event AvatarUploadEvent) error {
	var (
		operation   = "publish"
		destination = fmt.Sprintf("%s:%s", ExchangeAvatar, EventKeyAvatarUpload)
		spanName    = fmt.Sprintf("%s %s", operation, destination)
		messageId   = uuid.New().String()
	)

	ctx, span := r.tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.operation.name", operation),
			attribute.String("messaging.operation.type", "send"),
			attribute.String("messaging.destination.name", destination),
			attribute.String("messaging.rabbitmq.destination.routing_key", EventKeyAvatarUpload),
			attribute.String("messaging.message.id", messageId),
		),
	)
	defer span.End()

	body, err := json.Marshal(event)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("marshal json: %w", err)
	}

	headers := make(amqp.Table)
	otel.GetTextMapPropagator().Inject(ctx, AMQPTableCarrier{Table: headers})

	if err := r.publish(ctx, ExchangeAvatar, EventKeyAvatarUpload, amqp.Publishing{
		Headers:      headers,
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		Body:         body,
		MessageId:    messageId,
	}); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (r *RabbitmqPublisher) PublishAvatarDeleteEvent(ctx context.Context, event AvatarDeleteEvent) error {
	var (
		operation   = "publish"
		destination = fmt.Sprintf("%s:%s", ExchangeAvatar, EventKeyAvatarDelete)
		spanName    = fmt.Sprintf("%s %s", operation, destination)
		messageId   = uuid.New().String()
	)

	ctx, span := r.tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.operation.name", operation),
			attribute.String("messaging.operation.type", "send"),
			attribute.String("messaging.destination.name", destination),
			attribute.String("messaging.rabbitmq.destination.routing_key", EventKeyAvatarDelete),
			attribute.String("messaging.message.id", messageId),
		),
	)
	defer span.End()

	body, err := json.Marshal(event)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("marshal json: %w", err)
	}

	headers := make(amqp.Table)
	otel.GetTextMapPropagator().Inject(ctx, AMQPTableCarrier{Table: headers})

	if err := r.publish(ctx, ExchangeAvatar, EventKeyAvatarDelete, amqp.Publishing{
		Headers:      headers,
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		Body:         body,
		MessageId:    messageId,
	}); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (r *RabbitmqPublisher) publish(ctx context.Context, exchange, key string, msg amqp.Publishing) error {
	if err := r.retrier.Do(func() error {
		tctx, tcancel := context.WithTimeout(ctx, 10*time.Second)
		defer tcancel()

		confirmation, err := r.ch.PublishWithDeferredConfirmWithContext(tctx, exchange, key, false, false, msg)
		if err != nil {
			return fmt.Errorf("publish: %w", err)
		}

		ack, err := confirmation.WaitContext(tctx)
		if err != nil {
			return retry.Unrecoverable(fmt.Errorf("wait confirmation: %w", err))
		}
		if !ack {
			return ErrNACKReceived
		}
		return nil
	}); err != nil {
		return fmt.Errorf("retry: %w", err)
	}

	return nil
}
