package notify

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"gopkg.in/mail.v2"
)

// MockDialer implements MailDialer interface for testing
type MockDialer struct {
	DialAndSendFunc func(m ...*mail.Message) error
	Messages        []*mail.Message
}

func (md *MockDialer) DialAndSend(m ...*mail.Message) error {
	md.Messages = append(md.Messages, m...)
	if md.DialAndSendFunc != nil {
		return md.DialAndSendFunc(m...)
	}
	return nil
}

func TestNewMail(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		envVars map[string]string
		want    *Mail
		wantErr bool
	}{
		{
			name:   "valid URL with all parameters",
			schema: "smtp.gmail.com:587/recipient@example.com?username=sender@gmail.com&password=secret&from=sender@gmail.com&subject=Test+Subject&tls=true",
			want: &Mail{
				Host:     "smtp.gmail.com",
				Port:     587,
				Username: "sender@gmail.com",
				Password: "secret",
				From:     "sender@gmail.com",
				To:       "recipient@example.com",
				Subject:  "Test Subject",
				TLS:      true,
			},
			wantErr: false,
		},
		{
			name:   "valid URL with default port",
			schema: "smtp.gmail.com/recipient@example.com?username=sender@gmail.com&password=secret&from=sender@gmail.com",
			want: &Mail{
				Host:     "smtp.gmail.com",
				Port:     587,
				Username: "sender@gmail.com",
				Password: "secret",
				From:     "sender@gmail.com",
				To:       "recipient@example.com",
				Subject:  "Dewy Notification",
				TLS:      true,
			},
			wantErr: false,
		},
		{
			name:   "URL with environment variables",
			schema: "smtp.example.com:25/recipient@example.com",
			envVars: map[string]string{
				"MAIL_USERNAME": "env_user@example.com",
				"MAIL_PASSWORD": "env_password",
				"MAIL_FROM":     "env_from@example.com",
			},
			want: &Mail{
				Host:     "smtp.example.com",
				Port:     25,
				Username: "env_user@example.com",
				Password: "env_password",
				From:     "env_from@example.com",
				To:       "recipient@example.com",
				Subject:  "Dewy Notification",
				TLS:      true,
			},
			wantErr: false,
		},
		{
			name:   "from defaults to username",
			schema: "smtp.example.com:587/recipient@example.com?password=secret&username=sender@example.com",
			want: &Mail{
				Host:     "smtp.example.com",
				Port:     587,
				Username: "sender@example.com",
				Password: "secret",
				From:     "sender@example.com", // should default to username
				To:       "recipient@example.com",
				Subject:  "Dewy Notification",
				TLS:      true,
			},
			wantErr: false,
		},
		{
			name:    "missing host",
			schema:  ":587/recipient@example.com?username=user&password=pass&from=from@example.com",
			wantErr: true,
		},
		{
			name:    "missing username and from",
			schema:  "smtp.example.com:587/recipient@example.com?password=pass",
			wantErr: true,
		},
		{
			name:    "missing password",
			schema:  "smtp.example.com:587/recipient@example.com?username=user&from=from@example.com",
			wantErr: true,
		},
		{
			name:   "missing from but has username (should use username as from)",
			schema: "smtp.example.com:587/recipient@example.com?username=user@example.com&password=pass",
			want: &Mail{
				Host:     "smtp.example.com",
				Port:     587,
				Username: "user@example.com",
				Password: "pass",
				From:     "user@example.com", // should default to username
				To:       "recipient@example.com",
				Subject:  "Dewy Notification",
				TLS:      true,
			},
			wantErr: false,
		},
		{
			name:    "missing to",
			schema:  "smtp.example.com:587/?username=user&password=pass&from=from@example.com",
			wantErr: true,
		},
		{
			name:    "invalid port",
			schema:  "smtp.example.com:invalid/recipient@example.com?username=user&password=pass&from=from@example.com",
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
				os.Setenv(key, value)
			}

			// Clean up environment variables after test
			defer func() {
				for key := range tt.envVars {
					if original, exists := originalEnv[key]; exists {
						os.Setenv(key, original)
					} else {
						os.Unsetenv(key)
					}
				}
			}()

			got, err := NewMail(tt.schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if got.Host != tt.want.Host {
				t.Errorf("NewMail() Host = %v, want %v", got.Host, tt.want.Host)
			}
			if got.Port != tt.want.Port {
				t.Errorf("NewMail() Port = %v, want %v", got.Port, tt.want.Port)
			}
			if got.Username != tt.want.Username {
				t.Errorf("NewMail() Username = %v, want %v", got.Username, tt.want.Username)
			}
			if got.Password != tt.want.Password {
				t.Errorf("NewMail() Password = %v, want %v", got.Password, tt.want.Password)
			}
			if got.From != tt.want.From {
				t.Errorf("NewMail() From = %v, want %v", got.From, tt.want.From)
			}
			if got.To != tt.want.To {
				t.Errorf("NewMail() To = %v, want %v", got.To, tt.want.To)
			}
			if got.Subject != tt.want.Subject {
				t.Errorf("NewMail() Subject = %v, want %v", got.Subject, tt.want.Subject)
			}
			if got.TLS != tt.want.TLS {
				t.Errorf("NewMail() TLS = %v, want %v", got.TLS, tt.want.TLS)
			}
		})
	}
}

func TestMail_Send(t *testing.T) {
	tests := []struct {
		name     string
		mail     *Mail
		message  string
		mockFunc func(m ...*mail.Message) error
		wantErr  bool
	}{
		{
			name: "successful send",
			mail: &Mail{
				Host:     "smtp.example.com",
				Port:     587,
				Username: "user@example.com",
				Password: "password",
				From:     "from@example.com",
				To:       "to@example.com",
				Subject:  "Test Subject",
				TLS:      true,
			},
			message: "Test message",
			mockFunc: func(m ...*mail.Message) error {
				return nil
			},
			wantErr: false,
		},
		{
			name: "send with error (should not panic)",
			mail: &Mail{
				Host:     "smtp.example.com",
				Port:     587,
				Username: "user@example.com",
				Password: "password",
				From:     "from@example.com",
				To:       "to@example.com",
				Subject:  "Test Subject",
				TLS:      false,
			},
			message: "Test message",
			mockFunc: func(m ...*mail.Message) error {
				return errors.New("connection failed")
			},
			wantErr: false, // Send method logs errors but doesn't return them
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDialer := &MockDialer{
				DialAndSendFunc: tt.mockFunc,
			}
			tt.mail.SetDialer(mockDialer)

			ctx := context.Background()
			tt.mail.Send(ctx, tt.message)

			// Verify that message was passed to mock dialer
			if len(mockDialer.Messages) != 1 {
				t.Errorf("Expected 1 message to be sent, got %d", len(mockDialer.Messages))
				return
			}

			msg := mockDialer.Messages[0]
			if from := msg.GetHeader("From"); len(from) == 0 || from[0] != tt.mail.From {
				t.Errorf("Message From = %v, want %v", from, tt.mail.From)
			}
			if to := msg.GetHeader("To"); len(to) == 0 || to[0] != tt.mail.To {
				t.Errorf("Message To = %v, want %v", to, tt.mail.To)
			}
			if subject := msg.GetHeader("Subject"); len(subject) == 0 || subject[0] != tt.mail.Subject {
				t.Errorf("Message Subject = %v, want %v", subject, tt.mail.Subject)
			}
		})
	}
}

func TestMail_formatMessage(t *testing.T) {
	mail := &Mail{}
	message := "Test notification"
	
	formatted := mail.formatMessage(message)
	
	if formatted == "" {
		t.Error("formatMessage() returned empty string")
	}
	
	// Check that the original message is included
	if !strings.Contains(formatted, message) {
		t.Errorf("formatMessage() doesn't contain original message: %v", formatted)
	}
	
	// Check that system info is included
	if !strings.Contains(formatted, "Host:") {
		t.Error("formatMessage() doesn't contain Host information")
	}
	if !strings.Contains(formatted, "User:") {
		t.Error("formatMessage() doesn't contain User information")
	}
	if !strings.Contains(formatted, "Working dir:") {
		t.Error("formatMessage() doesn't contain Working dir information")
	}
	if !strings.Contains(formatted, "Dewy: https://github.com/linyows/dewy") {
		t.Error("formatMessage() doesn't contain signature")
	}
}

func TestMail_SetDialer(t *testing.T) {
	mail := &Mail{}
	mockDialer := &MockDialer{}
	
	mail.SetDialer(mockDialer)
	
	if mail.dialer != mockDialer {
		t.Error("SetDialer() didn't set the dialer correctly")
	}
}