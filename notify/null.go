package notify

import "context"

type Null struct {
}

func (n *Null) Send(ctx context.Context, message string) {
}
