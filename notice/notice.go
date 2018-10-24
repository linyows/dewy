package notice

import "context"

type Notice interface {
	Name() string
	Notify(message string, ctx context.Context)
	Default()
}

func New(n Notice) Notice {
	switch n.Name() {
	case "slack":
		n.Default()
		return n
	default:
		panic("no noticer")
	}
}
