package notice

import "context"

type Notice interface {
	Notify(ctx context.Context) error
}

func New(t string, c Config) Notice {
	switch t {
	case "slack":
		return &Slack{}
	default:
		panic("no provider")
	}
}
