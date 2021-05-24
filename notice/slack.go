package notice

import (
	"context"
	"crypto/md5"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/lestrrat-go/slack"
	"github.com/lestrrat-go/slack/objects"
)

var (
	defaultSlackChannel = "general"
	// SlackUsername variable
	SlackUsername = "Dewy"
	// SlackIconURL variable
	SlackIconURL = "https://raw.githubusercontent.com/linyows/dewy/main/misc/dewy-icon.512.png"
	// SlackFooter variable
	SlackFooter = "Dewy notice/slack"
	// SlackFooterIcon variable
	SlackFooterIcon = SlackIconURL
)

type key int

// MetaContextKey for context key
const MetaContextKey key = iota

// Slack struct
type Slack struct {
	Token   string
	Channel string
	Meta    *Config
}

func (s *Slack) String() string {
	return "slack"
}

// Notify posts message to Slack channel
func (s *Slack) Notify(ctx context.Context, message string) {
	if t := os.Getenv("SLACK_TOKEN"); t != "" {
		s.Token = t
	}
	if c := os.Getenv("SLACK_CHANNEL"); c != "" {
		s.Channel = c
	}
	if s.Channel == "" {
		s.Channel = defaultSlackChannel
	}
	if s.Token == "" {
		log.Printf("[ERROR] Slack token is required")
		return
	}

	cl := slack.New(s.Token)
	at := s.buildAttachment(message, ctx.Value(MetaContextKey) != nil)

	_, err := cl.Chat().PostMessage(s.Channel).Username(SlackUsername).
		IconURL(SlackIconURL).Attachment(&at).Text("").Do(ctx)
	if err != nil {
		log.Printf("[ERROR] Slack postMessage failure: %#v", err)
	}
}

func (s *Slack) genColor() string {
	return strings.ToUpper(fmt.Sprintf("#%x", md5.Sum([]byte(hostname())))[0:7])
}

func (s *Slack) buildAttachment(message string, meta bool) objects.Attachment {
	var at objects.Attachment
	at.Color = s.genColor()

	if meta {
		at.Text = message
		at.Title = s.Meta.RepoName
		at.TitleLink = s.Meta.RepoLink
		at.AuthorName = s.Meta.RepoOwner
		at.AuthorLink = s.Meta.RepoOwnerLink
		at.AuthorIcon = s.Meta.RepoOwnerIcon
		at.Footer = SlackFooter
		at.FooterIcon = SlackFooterIcon
		at.Timestamp = objects.Timestamp(time.Now().Unix())
		at.Fields.
			Append(&objects.AttachmentField{Title: "Command", Value: s.Meta.Command, Short: true}).
			Append(&objects.AttachmentField{Title: "Host", Value: hostname(), Short: true}).
			Append(&objects.AttachmentField{Title: "User", Value: username(), Short: true}).
			Append(&objects.AttachmentField{Title: "Source", Value: s.Meta.Source, Short: true}).
			Append(&objects.AttachmentField{Title: "Working directory", Value: cwd(), Short: false})
	} else {
		at.Text = fmt.Sprintf("%s of <%s|%s> on %s", message, s.Meta.RepoLink, s.Meta.RepoName, hostname())
	}

	return at
}
