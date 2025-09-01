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

// SendHookResult sends hook result with detailed attachment.
func (s *Slack) SendHookResult(ctx context.Context, hookType string, result *HookResult) {
	at := s.BuildHookAttachment(hookType, result)

	var err error
	if s.sender != nil {
		err = s.sender.SendMessage(ctx, s.Channel, SlackUsername, SlackIconURL, "", &at)
	} else {
		cl := slack.New(s.token)
		_, err = cl.Chat().PostMessage(s.Channel).Username(SlackUsername).
			IconURL(SlackIconURL).Attachment(&at).Text("").Do(ctx)
	}

	if err != nil {
		s.logger.Error("Slack hook result notification failure", slog.String("error", err.Error()))
	}
}

// BuildHookAttachment returns attachment for hook result.
func (s *Slack) BuildHookAttachment(hookType string, result *HookResult) objects.Attachment {
	var at objects.Attachment

	// Set color based on success/failure
	if result.Success {
		at.Color = "#36a64f" // Green for success
	} else {
		at.Color = "#dd0000" // Red for failure
	}

	// Set title with status icon at the end
	at.Title = fmt.Sprintf("%s Hook", hookType)

	// Set command in text field
	at.Text = fmt.Sprintf("`%s`", result.Command)

	// Add exit code and duration fields (short)
	at.Fields = append(at.Fields, &objects.AttachmentField{
		Title: "Exit Code",
		Value: fmt.Sprintf("%d", result.ExitCode),
		Short: true,
	})

	at.Fields = append(at.Fields, &objects.AttachmentField{
		Title: "Duration",
		Value: result.Duration.String(),
		Short: true,
	})

	// Add fields for stdout and stderr if they exist
	if result.Stdout != "" {
		at.Fields = append(at.Fields, &objects.AttachmentField{
			Title: "Stdout",
			Value: s.formatOutput(result.Stdout),
			Short: false,
		})
	}

	if result.Stderr != "" {
		at.Fields = append(at.Fields, &objects.AttachmentField{
			Title: "Stderr",
			Value: s.formatOutput(result.Stderr),
			Short: false,
		})
	}

	// Set footer
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

// formatOutput formats long output text for Slack display with proper truncation.
func (s *Slack) formatOutput(output string) string {
	const maxFieldLength = 2000 // Slack attachment field limit is ~3000, leave some buffer
	const maxLines = 50         // Limit number of lines to prevent very long outputs

	lines := strings.Split(output, "\n")

	// Limit number of lines
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, fmt.Sprintf("... (%d more lines truncated)", len(strings.Split(output, "\n"))-maxLines))
	}

	truncatedOutput := strings.Join(lines, "\n")

	// If still too long, truncate by character count
	if len(truncatedOutput) > maxFieldLength {
		// Find a good truncation point (prefer newline)
		truncateAt := maxFieldLength - 100 // Leave space for truncation message
		for truncateAt > 0 && truncatedOutput[truncateAt] != '\n' {
			truncateAt--
		}
		if truncateAt <= 0 {
			truncateAt = maxFieldLength - 100
		}

		truncatedOutput = truncatedOutput[:truncateAt]
		truncatedOutput += fmt.Sprintf("\n... (%d more characters truncated)", len(output)-truncateAt)
	}

	// Wrap in code block, ensuring it's properly closed
	return fmt.Sprintf("```\n%s\n```", truncatedOutput)
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
