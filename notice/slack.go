package notice

import (
	"context"

	"github.com/lestrrat-go/slack"
)

type Slack struct {
	token    string
	username string
	message  string
}

func (s *Slack) Notify(ctx context.Context) err {
	cl := slack.New(s.token)

	authres, err := cl.Auth().Test().Do(ctx)
	if err != nil {
		return err
	}

	_, err := cl.Chat().PostMessage("@" + c.username).Text(c.message).Do(ctx)

	return err
}
