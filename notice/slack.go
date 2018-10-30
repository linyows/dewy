package notice

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/lestrrat-go/slack"
	"github.com/lestrrat-go/slack/objects"
)

var (
	defaultSlackUsername string = "Dewy"
	defaultSlackChannel  string = "general"
	defaultSlackMessage  string = "Hi guys, This is a default message."
	defaultSlackIconURL  string = "https://raw.githubusercontent.com/linyows/dewy/master/misc/dewy-icon.512.png"
	SlackFooter          string = "Dewy notice/slack"
	SlackFooterIcon      string = defaultSlackIconURL
)

type Slack struct {
	Token         string
	Username      string
	Channel       string
	IconURL       string
	Message       string
	RepoOwner     string
	RepoName      string
	RepoLink      string
	RepoOwnerIcon string
	RepoOwnerLink string
	Host          string
}

func (s *Slack) String() string {
	return "slack"
}

func (s *Slack) setDefault() {
	if s.Username == "" {
		s.Username = defaultSlackUsername
	}
	if s.Channel == "" {
		s.Channel = defaultSlackChannel
	}
	if s.IconURL == "" {
		s.IconURL = defaultSlackIconURL
	}
	if s.Message == "" {
		s.Message = defaultSlackMessage
	}
}

func (s *Slack) Notify(m string, fields []*Field, ctx context.Context) {
	if s.Token == "" {
		err := errors.New(fmt.Sprintf("Slack token not found"))
		log.Printf("[ERROR] Failed %s notice: %#v", s, err)
		return
	}

	s.setDefault()

	cl := slack.New(s.Token)

	var at objects.Attachment
	at.Color = s.genColor()

	if len(fields) > 0 {
		at.Title = s.RepoName
		at.TitleLink = s.RepoLink
		at.Text = m
		at.AuthorName = s.RepoOwner
		at.AuthorLink = s.RepoOwnerLink
		at.AuthorIcon = s.RepoOwnerIcon
		at.Footer = SlackFooter
		at.FooterIcon = SlackFooterIcon
		at.Timestamp = objects.Timestamp(time.Now().Unix())
		at.Fields.Append(&objects.AttachmentField{
			Title: "Host",
			Value: s.Host,
			Short: true,
		})
		for _, f := range fields {
			at.Fields.Append(&objects.AttachmentField{
				Title: f.Title,
				Value: f.Value,
				Short: f.Short,
			})
		}
	} else {
		at.Text = fmt.Sprintf("%s of <%s|%s> on %s", m, s.RepoLink, s.RepoName, s.Host)
	}

	_, err := cl.Chat().PostMessage(s.Channel).
		Username(s.Username).
		IconURL(s.IconURL).
		Attachment(&at).
		Text("").
		Do(ctx)

	if err != nil {
		log.Printf("[ERROR] Failed %s notice: %#v", s, err)
	}
}

func (s *Slack) genColor() string {
	return strings.ToUpper(fmt.Sprintf("#%x", md5.Sum([]byte(s.Host)))[0:7])
}
