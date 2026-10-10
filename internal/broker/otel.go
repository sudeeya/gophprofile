package broker

import (
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/propagation"
)

var _ propagation.TextMapCarrier = (*AMQPTableCarrier)(nil)

type AMQPTableCarrier struct {
	amqp.Table
}

func (a AMQPTableCarrier) Get(key string) string {
	v, _ := a.Table[key].(string)
	return v
}

func (a AMQPTableCarrier) Set(key string, value string) {
	a.Table[key] = value
}

func (a AMQPTableCarrier) Keys() []string {
	keys := make([]string, 0, len(a.Table))
	for k := range a.Table {
		keys = append(keys, k)
	}
	return keys
}
