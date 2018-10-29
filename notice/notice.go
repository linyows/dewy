package notice

import "context"

type Notice interface {
	Name() string
	Notify(message string, ctx context.Context)
}

func New(n Notice) Notice {
	switch n.Name() {
	case "slack":
		return n
	default:
		panic("no noticer")
	}
}
