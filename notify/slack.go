package notify

import (
	"context"
	"crypto/md5" //nolint:gosec
	"fmt"
	"log"
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
	// SlackFooter variable.
	SlackFooter = "Dewy notify/slack"
	// SlackFooterIcon variable.
	SlackFooterIcon = SlackIconURL

	decoder = schema.NewDecoder()
)

// Slack struct.
type Slack struct {
	Channel  string `schema:"-"`
	Title    string `schema:"title"`
	TitleURL string `schema:"url"`
	token    string
	github   *Github
}

func NewSlack(schema string) (*Slack, error) {
	u, err := url.Parse(schema)
	if err != nil {
		return nil, err
	}

	s := &Slack{Channel: u.Path}
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

// Send posts message to Slack channel.
func (s *Slack) Send(ctx context.Context, message string) {
	cl := slack.New(s.token)
	at := s.BuildAttachment(message)
	_, err := cl.Chat().PostMessage(s.Channel).Username(SlackUsername).
		IconURL(SlackIconURL).Attachment(&at).Text("").Do(ctx)
	if err != nil {
		log.Printf("[ERROR] Slack postMessage failure: %#v", err)
	}
}

func (s *Slack) genColor() string {
	return strings.ToUpper(fmt.Sprintf("#%x", md5.Sum([]byte(hostname())))[0:7]) //nolint:gosec
}

// Github
type Github struct {
	// linyows
	Owner string
	// dewy
	Repo string
	// appname_linux_amd64.tar.gz
	Artifact string
}

// OwnerURL returns owner URL.
func (g *Github) OwnerURL() string {
	return fmt.Sprintf("https://github.com/%s", g.Owner)
}

// OwnerIconURL returns owner icon URL.
func (g *Github) OwnerIconURL() string {
	return fmt.Sprintf("%s.png?size=200", g.OwnerURL())
}

// URL returns repository URL.
func (g *Github) RepoURL() string {
	return fmt.Sprintf("%s/%s", g.OwnerURL(), g.Repo)
}

// BuildAttachmentByGithubArgs returns slack an attachment
func (s *Slack) BuildAttachment(message string) objects.Attachment {
	var at objects.Attachment
	at.Color = s.genColor()

	if s.github != nil {
		at.Text = message
		at.Title = s.github.Repo
		at.TitleLink = s.github.RepoURL()
		at.AuthorName = s.github.Owner
		at.AuthorLink = s.github.OwnerURL()
		at.AuthorIcon = s.github.OwnerIconURL()
		at.Footer = SlackFooter
		at.FooterIcon = SlackFooterIcon
		at.Timestamp = objects.Timestamp(time.Now().Unix())
		at.Fields.
			Append(&objects.AttachmentField{Title: "Host", Value: hostname(), Short: true}).
			Append(&objects.AttachmentField{Title: "User", Value: username(), Short: true}).
			Append(&objects.AttachmentField{Title: "Source", Value: s.github.Artifact, Short: true}).
			Append(&objects.AttachmentField{Title: "Working directory", Value: cwd(), Short: false})
	} else if s.Title != "" && s.TitleURL != "" {
		at.Text = fmt.Sprintf("%s of <%s|%s> on %s", message, s.TitleURL, s.Title, hostname())
	} else if s.Title != "" {
		at.Text = fmt.Sprintf("%s of %s on %s", message, s.Title, hostname())
	} else {
		at.Text = fmt.Sprintf("%s on %s", message, hostname())
	}

	return at
}
