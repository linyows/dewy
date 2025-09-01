package notifier

import (
	"context"
	"crypto/md5" //nolint:gosec
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/schema"
	"github.com/lestrrat-go/slack"
	"github.com/lestrrat-go/slack/objects"
)

var (
	defaultSlackChannel = "randam"
	// SlackUsername variable.
	SlackUsername = "Dewy"
	// SlackIconURL variable.
	SlackIconURL = "https://raw.githubusercontent.com/linyows/dewy/main/misc/dewy-icon.512.png"
	// SlackFooterIcon variable.
	SlackFooterIcon = "https://raw.githubusercontent.com/linyows/dewy/main/misc/dewy-icon.32.png"

	decoder = schema.NewDecoder()
)

// SlackSender interface for dependency injection and testing.
type SlackSender interface {
	SendMessage(ctx context.Context, channel, username, iconURL, text string, attachment *objects.Attachment) error
}

// Slack struct.
type Slack struct {
	Channel  string `schema:"-"`
	Title    string `schema:"title"`
	TitleURL string `schema:"url"`
	token    string
	sender   SlackSender // for testing
	logger   *slog.Logger
}

func NewSlack(schema string, logger *slog.Logger) (*Slack, error) {
	u, err := url.Parse(schema)
	if err != nil {
		return nil, err
	}

	s := &Slack{Channel: u.Path, logger: logger}
	if err := decoder.Decode(s, u.Query()); err != nil {
		return nil, err
	}

	if s.Channel == "" {
		s.Channel = defaultSlackChannel
	}
	if t := os.Getenv("SLACK_TOKEN"); t != "" {
		s.token = t
	}
	if s.token == "" {
		return nil, fmt.Errorf("slack token is required")
	}

	return s, nil
}

// SetSender sets the slack sender for testing purposes.
func (s *Slack) SetSender(sender SlackSender) {
	s.sender = sender
}

// Send posts message to Slack channel.
func (s *Slack) Send(ctx context.Context, message string) {
	at := s.BuildAttachment(message)

	var err error
	if s.sender != nil {
		err = s.sender.SendMessage(ctx, s.Channel, SlackUsername, SlackIconURL, "", &at)
	} else {
		cl := slack.New(s.token)
		_, err = cl.Chat().PostMessage(s.Channel).Username(SlackUsername).
			IconURL(SlackIconURL).Attachment(&at).Text("").Do(ctx)
	}

	if err != nil {
		s.logger.Error("Slack postMessage failure", slog.String("error", err.Error()))
	}
}

func (s *Slack) genColor() string {
	return strings.ToUpper(fmt.Sprintf("#%x", md5.Sum([]byte(hostname())))[0:7]) //nolint:gosec
}

// BuildAttachment returns attachment for slack.
func (s *Slack) BuildAttachment(message string) objects.Attachment {
	var at objects.Attachment
	at.Color = s.genColor()
	at.Text = message

	// Set message text based on title configuration
	if s.Title != "" && s.TitleURL != "" {
		at.Footer = fmt.Sprintf("<%s|%s>/%s", s.TitleURL, s.Title, hostname())
	} else if s.Title != "" {
		at.Footer = fmt.Sprintf("%s/%s", s.Title, hostname())
	} else {
		at.Footer = hostname()
	}

	at.FooterIcon = SlackFooterIcon
	at.Timestamp = objects.Timestamp(time.Now().Unix())

	return at
}
