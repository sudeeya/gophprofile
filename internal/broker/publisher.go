package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/avast/retry-go/v5"
	amqp "github.com/rabbitmq/amqp091-go"
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
}

func NewRabbitmq(config RabbitmqPublisherConfig) (*RabbitmqPublisher, error) {
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

	if err := ch.ExchangeDeclare(_exchange, "direct", true, false, false, false, nil); err != nil {
		return nil, fmt.Errorf("declare exchange: %w", err)
	}

	if _, err := ch.QueueDeclare(_queueUpload, true, false, false, false, nil); err != nil {
		return nil, fmt.Errorf("declare queue: %w", err)
	}

	if err := ch.QueueBind(_queueUpload, _keyUpload, _exchange, false, nil); err != nil {
		return nil, fmt.Errorf("bind queue: %w", err)
	}

	if _, err := ch.QueueDeclare(_queueDelete, true, false, false, false, nil); err != nil {
		return nil, fmt.Errorf("declare queue: %w", err)
	}

	if err := ch.QueueBind(_queueDelete, _keyDelete, _exchange, false, nil); err != nil {
		return nil, fmt.Errorf("bind queue: %w", err)
	}

	retrier := retry.New(
		retry.Attempts(config.PublisherConfirmsRetryAttempts),
		retry.Delay(config.PublisherConfirmsRetryDelay),
		retry.DelayType(retry.CombineDelay(retry.BackOffDelay, retry.RandomDelay)),
		retry.LastErrorOnly(true),
	)

	return &RabbitmqPublisher{
		conn:    conn,
		ch:      ch,
		retrier: retrier,
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
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	return r.publish(ctx, _exchange, _keyUpload, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		Body:         body,
	})
}

func (r *RabbitmqPublisher) PublishAvatarDeleteEvent(ctx context.Context, event AvatarDeleteEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	return r.publish(ctx, _exchange, _keyDelete, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		Body:         body,
	})
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
