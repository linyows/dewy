package notice

import "context"

type Notice interface {
	String() string
	Notify(message string, fields []*Field, ctx context.Context)
}

type Field struct {
	Title string
	Value string
	Short bool
}

func New(n Notice) Notice {
	switch n.String() {
	case "slack":
		return n
	default:
		panic("no noticer")
	}
}
