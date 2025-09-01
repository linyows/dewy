package notifier

import "context"

type Null struct {
}

func (n *Null) Send(ctx context.Context, message string) {
}

func (n *Null) SendHookResult(ctx context.Context, hookType string, result *HookResult) {
}
