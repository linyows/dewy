package notifier

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/lestrrat-go/slack/objects"
)

// Mock implementations for testing

type MockSlackSender struct {
	SendMessageFunc func(ctx context.Context, channel, username, iconURL, text string, attachment *objects.Attachment) error
	LastCall        *SlackSendCall
}

type SlackSendCall struct {
	Channel    string
	Username   string
	IconURL    string
	Text       string
	Attachment *objects.Attachment
}

func (mss *MockSlackSender) SendMessage(ctx context.Context, channel, username, iconURL, text string, attachment *objects.Attachment) error {
	mss.LastCall = &SlackSendCall{
		Channel:    channel,
		Username:   username,
		IconURL:    iconURL,
		Text:       text,
		Attachment: attachment,
	}

	if mss.SendMessageFunc != nil {
		return mss.SendMessageFunc(ctx, channel, username, iconURL, text, attachment)
	}
	return nil
}

func TestNewSlack(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		envVars map[string]string
		want    *Slack
		wantErr bool
	}{
		{
			name:   "valid URL with channel",
			schema: "/general?title=Test+Project&url=https://example.com",
			envVars: map[string]string{
				"SLACK_TOKEN": "xoxb-test-token",
			},
			want: &Slack{
				Channel:  "/general",
				Title:    "Test Project",
				TitleURL: "https://example.com",
				token:    "xoxb-test-token",
			},
			wantErr: false,
		},
		{
			name:   "valid URL with default channel",
			schema: "?title=Test",
			envVars: map[string]string{
				"SLACK_TOKEN": "xoxb-test-token",
			},
			want: &Slack{
				Channel: defaultSlackChannel,
				Title:   "Test",
				token:   "xoxb-test-token",
			},
			wantErr: false,
		},
		{
			name:   "URL without query parameters",
			schema: "/random",
			envVars: map[string]string{
				"SLACK_TOKEN": "xoxb-test-token",
			},
			want: &Slack{
				Channel: "/random",
				token:   "xoxb-test-token",
			},
			wantErr: false,
		},
		{
			name:   "missing slack token",
			schema: "/general",
			envVars: map[string]string{
				"SLACK_TOKEN": "", // explicitly set to empty
			},
			wantErr: true,
		},
		{
			name:    "invalid URL",
			schema:  "://invalid-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables if provided
			originalEnv := make(map[string]string)
			for key, value := range tt.envVars {
				originalEnv[key] = os.Getenv(key)
				if value == "" {
					os.Unsetenv(key)
				} else {
					os.Setenv(key, value)
				}
			}

			// Clean up environment variables after test
			defer func() {
				for key := range tt.envVars {
					if original, exists := originalEnv[key]; exists && original != "" {
						os.Setenv(key, original)
					} else {
						os.Unsetenv(key)
					}
				}
			}()

			got, err := NewSlack(tt.schema, testLogger())
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSlack() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if got.Channel != tt.want.Channel {
				t.Errorf("NewSlack() Channel = %v, want %v", got.Channel, tt.want.Channel)
			}
			if got.Title != tt.want.Title {
				t.Errorf("NewSlack() Title = %v, want %v", got.Title, tt.want.Title)
			}
			if got.TitleURL != tt.want.TitleURL {
				t.Errorf("NewSlack() TitleURL = %v, want %v", got.TitleURL, tt.want.TitleURL)
			}
			if got.token != tt.want.token {
				t.Errorf("NewSlack() token = %v, want %v", got.token, tt.want.token)
			}
		})
	}
}

func TestSlack_Send(t *testing.T) {
	tests := []struct {
		name     string
		slack    *Slack
		message  string
		mockFunc func(ctx context.Context, channel, username, iconURL, text string, attachment *objects.Attachment) error
		wantErr  bool
	}{
		{
			name: "successful send",
			slack: &Slack{
				Channel: "/general",
				Title:   "Test",
				token:   "xoxb-test-token",
				logger:  testLogger(),
			},
			message: "Test message",
			mockFunc: func(ctx context.Context, channel, username, iconURL, text string, attachment *objects.Attachment) error {
				return nil
			},
			wantErr: false,
		},
		{
			name: "send with error (should not panic)",
			slack: &Slack{
				Channel: "/general",
				Title:   "Test",
				token:   "xoxb-test-token",
				logger:  testLogger(),
			},
			message: "Test message",
			mockFunc: func(ctx context.Context, channel, username, iconURL, text string, attachment *objects.Attachment) error {
				return errors.New("API error")
			},
			wantErr: false, // Send method logs errors but doesn't return them
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSender := &MockSlackSender{
				SendMessageFunc: tt.mockFunc,
			}
			tt.slack.SetSender(mockSender)

			ctx := context.Background()
			tt.slack.Send(ctx, tt.message)

			// Verify that the correct values were passed to the mock
			if mockSender.LastCall == nil {
				t.Error("SendMessage should have been called")
				return
			}

			call := mockSender.LastCall
			if call.Channel != tt.slack.Channel {
				t.Errorf("SendMessage called with channel = %v, want %v", call.Channel, tt.slack.Channel)
			}
			if call.Username != SlackUsername {
				t.Errorf("Username set to %v, want %v", call.Username, SlackUsername)
			}
			if call.IconURL != SlackIconURL {
				t.Errorf("IconURL set to %v, want %v", call.IconURL, SlackIconURL)
			}
			if call.Attachment == nil {
				t.Error("Attachment should not be nil")
			}
			if call.Text != "" {
				t.Errorf("Text set to %v, want empty string", call.Text)
			}
		})
	}
}

func TestSlack_BuildAttachment(t *testing.T) {
	tests := []struct {
		name    string
		slack   *Slack
		message string
		want    func(attachment objects.Attachment) bool
	}{
		{
			name: "attachment with title and URL",
			slack: &Slack{
				Channel:  "/general",
				Title:    "Test Project",
				TitleURL: "https://example.com",
			},
			message: "Test message",
			want: func(attachment objects.Attachment) bool {
				return strings.Contains(attachment.Text, "Test message") &&
					strings.Contains(attachment.Footer, "Test Project") &&
					strings.Contains(attachment.Footer, "https://example.com") &&
					attachment.Footer != "" &&
					attachment.Timestamp != 0
			},
		},
		{
			name: "attachment with title only",
			slack: &Slack{
				Channel: "/general",
				Title:   "Test Project",
			},
			message: "Test message",
			want: func(attachment objects.Attachment) bool {
				return strings.Contains(attachment.Text, "Test message") &&
					strings.Contains(attachment.Footer, "Test Project") &&
					attachment.Footer != "" &&
					attachment.Timestamp != 0
			},
		},
		{
			name: "attachment without extra info",
			slack: &Slack{
				Channel: "/general",
			},
			message: "Test message",
			want: func(attachment objects.Attachment) bool {
				return strings.Contains(attachment.Text, "Test message") &&
					attachment.Footer != "" &&
					attachment.Timestamp != 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attachment := tt.slack.BuildAttachment(tt.message)

			if !tt.want(attachment) {
				t.Errorf("BuildAttachment() failed validation for %s", tt.name)
				t.Logf("Attachment text: %s", attachment.Text)
				t.Logf("Attachment title: %s", attachment.Title)
			}

			// Check that color is set
			if attachment.Color == "" {
				t.Error("BuildAttachment() Color should not be empty")
			}
		})
	}
}

func TestSlack_genColor(t *testing.T) {
	slack := &Slack{}
	color := slack.genColor()

	if color == "" {
		t.Error("genColor() returned empty string")
	}

	if !strings.HasPrefix(color, "#") {
		t.Errorf("genColor() = %v, should start with #", color)
	}

	if len(color) != 7 {
		t.Errorf("genColor() = %v, should be 7 characters long", color)
	}
}

func TestSlack_SetSender(t *testing.T) {
	slack := &Slack{}
	mockSender := &MockSlackSender{}

	slack.SetSender(mockSender)

	if slack.sender != mockSender {
		t.Error("SetSender() didn't set the sender correctly")
	}
}
