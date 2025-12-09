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

// isRed checks if the given RGB color is considered "red" that might indicate failure.
// Red is defined as: r >= 0xCC and g <= 0x33 and b <= 0x33.
func isRed(r, g, b uint8) bool {
	return r >= 0xCC && g <= 0x33 && b <= 0x33
}

// rgbToHSL converts RGB color values to HSL.
// r, g, b should be in range 0-255.
// Returns h (0-360), s (0-1), l (0-1).
func rgbToHSL(r, g, b uint8) (h, s, l float64) {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0

	max := rf
	if gf > max {
		max = gf
	}
	if bf > max {
		max = bf
	}

	min := rf
	if gf < min {
		min = gf
	}
	if bf < min {
		min = bf
	}

	l = (max + min) / 2.0

	if max == min {
		h = 0
		s = 0
	} else {
		d := max - min
		if l > 0.5 {
			s = d / (2.0 - max - min)
		} else {
			s = d / (max + min)
		}

		switch max {
		case rf:
			h = (gf - bf) / d
			if gf < bf {
				h += 6
			}
		case gf:
			h = (bf-rf)/d + 2
		case bf:
			h = (rf-gf)/d + 4
		}
		h *= 60
	}

	return h, s, l
}

// hslToRGB converts HSL color values to RGB.
// h should be in range 0-360, s and l in range 0-1.
// Returns r, g, b in range 0-255.
func hslToRGB(h, s, l float64) (r, g, b uint8) {
	var rf, gf, bf float64

	if s == 0 {
		rf = l
		gf = l
		bf = l
	} else {
		hueToRGB := func(p, q, t float64) float64 {
			if t < 0 {
				t += 1
			}
			if t > 1 {
				t -= 1
			}
			if t < 1.0/6.0 {
				return p + (q-p)*6*t
			}
			if t < 1.0/2.0 {
				return q
			}
			if t < 2.0/3.0 {
				return p + (q-p)*(2.0/3.0-t)*6
			}
			return p
		}

		var q float64
		if l < 0.5 {
			q = l * (1 + s)
		} else {
			q = l + s - l*s
		}
		p := 2*l - q

		h = h / 360.0
		rf = hueToRGB(p, q, h+1.0/3.0)
		gf = hueToRGB(p, q, h)
		bf = hueToRGB(p, q, h-1.0/3.0)
	}

	r = uint8(rf*255 + 0.5)
	g = uint8(gf*255 + 0.5)
	b = uint8(bf*255 + 0.5)

	return r, g, b
}

func (s *Slack) genColor() string {
	// Generate initial color from hostname
	hashBytes := md5.Sum([]byte(hostname())) //nolint:gosec
	r := hashBytes[0]
	g := hashBytes[1]
	b := hashBytes[2]

	// Avoid red colors that might indicate failure
	if isRed(r, g, b) {
		h, sat, l := rgbToHSL(r, g, b)
		// Rotate hue by +35 degrees to shift away from red
		h = h + 35.0
		if h >= 360.0 {
			h -= 360.0
		}
		r, g, b = hslToRGB(h, sat, l)
	}

	return strings.ToUpper(fmt.Sprintf("#%02X%02X%02X", r, g, b))
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
	at.Text = fmt.Sprintf("```\n%s\n```", result.Command)

	// Add fields for stdout and stderr first if they exist
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

	// Add exit code and duration fields (short)
	at.Fields = append(at.Fields, &objects.AttachmentField{
		Title: "Exit Code",
		Value: fmt.Sprintf("`%d`", result.ExitCode),
		Short: true,
	})

	at.Fields = append(at.Fields, &objects.AttachmentField{
		Title: "Duration",
		Value: result.Duration.String(),
		Short: true,
	})

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
