package notice

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/lestrrat-go/slack"
	"github.com/lestrrat-go/slack/objects"
)

var (
	defaultSlackUsername        string = "Dewy"
	defaultSlackChannel         string = "general"
	defaultSlackMessage         string = "Hi guys, This is a default message."
	defaultSlackIconURL         string = "https://raw.githubusercontent.com/linyows/dewy/master/misc/dewy-icon.512.png"
	defaultSlackAttachmentColor string = "#3e7c96"
)

type Slack struct {
	Token    string
	Username string
	Channel  string
	IconURL  string
}

func (s *Slack) Name() string {
	return "slack"
}

func (s *Slack) Default() {
	s.Username = defaultSlackUsername
	s.Channel = defaultSlackChannel
	s.IconURL = defaultSlackIconURL
}

func (s *Slack) hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return fmt.Sprintf("%#v", err)
	}
	return name
}

func (s *Slack) Notify(m string, ctx context.Context) {
	if s.Token == "" {
		err := errors.New(fmt.Sprintf("Slack Token not found"))
		log.Printf("[ERROR] Failed %s notice: %#v", s.Name(), err)
	}

	if m == "" {
		m = defaultSlackMessage
	}

	cl := slack.New(s.Token)
	var at objects.Attachment
	at.Color = defaultSlackAttachmentColor
	at.Text = fmt.Sprintf("%s on %s", m, s.hostname())

	_, err := cl.Chat().PostMessage(s.Channel).
		Username(s.Username).
		IconURL(s.IconURL).
		Attachment(&at).
		Text("").
		Do(ctx)

	if err != nil {
		log.Printf("[ERROR] Failed %s notice: %#v", s.Name(), err)
	}
}
