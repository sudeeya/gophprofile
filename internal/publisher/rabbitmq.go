package publisher

import "context"

type Rabbitmq struct {
}

func NewRabbitmq() (*Rabbitmq, error) {
	return nil, nil
}

func (p *Rabbitmq) Ping(ctx context.Context) error {
	return nil
}

func (p *Rabbitmq) Publish(ctx context.Context, event AvatarEvent) error {
	return nil
}
