package notifier

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"

	"gopkg.in/mail.v2"
)

// MailDialer interface for dependency injection and testing.
type MailDialer interface {
	DialAndSend(m ...*mail.Message) error
}

// Mail struct.
type Mail struct {
	Host     string     `schema:"host"`
	Port     int        `schema:"port"`
	Username string     `schema:"username"`
	Password string     `schema:"password"`
	From     string     `schema:"from"`
	To       string     `schema:"to"`
	Subject  string     `schema:"subject"`
	TLS      bool       `schema:"tls"`
	dialer   MailDialer // for testing
	logger   *slog.Logger
}

// NewMail creates a new Mail notifier.
func NewMail(schema string, logger *slog.Logger) (*Mail, error) {
	// Add scheme to make it a valid URL for parsing
	u, err := url.Parse("smtp://" + schema)
	if err != nil {
		return nil, err
	}

	m := &Mail{
		Host:    u.Hostname(),
		Subject: "Dewy Notification",
		TLS:     true,
	}

	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil {
			return nil, fmt.Errorf("invalid port: %s", u.Port())
		}
		m.Port = port
	} else {
		m.Port = 587 // default SMTP port with STARTTLS
	}

	if err := decoder.Decode(m, u.Query()); err != nil {
		return nil, err
	}

	// Get credentials from environment variables if not provided in URL
	if m.Username == "" {
		m.Username = os.Getenv("MAIL_USERNAME")
	}
	if m.Password == "" {
		m.Password = os.Getenv("MAIL_PASSWORD")
	}
	if m.From == "" {
		m.From = os.Getenv("MAIL_FROM")
	}

	// Extract To address from URL path if provided
	if u.Path != "" && u.Path != "/" {
		m.To = strings.TrimPrefix(u.Path, "/")
	}

	// Set from to username if not specified
	if m.From == "" && m.Username != "" {
		m.From = m.Username
	}

	// Validate required fields
	if m.Host == "" {
		return nil, fmt.Errorf("mail host is required")
	}
	if m.Username == "" {
		return nil, fmt.Errorf("mail username is required")
	}
	if m.Password == "" {
		return nil, fmt.Errorf("mail password is required")
	}
	if m.From == "" {
		return nil, fmt.Errorf("mail from address is required")
	}
	if m.To == "" {
		return nil, fmt.Errorf("mail to address is required")
	}

	m.logger = logger
	return m, nil
}

// SetDialer sets the mail dialer for testing purposes.
func (m *Mail) SetDialer(dialer MailDialer) {
	m.dialer = dialer
}

// Send sends an email notification.
func (m *Mail) Send(ctx context.Context, message string) {
	msg := mail.NewMessage()
	msg.SetHeader("From", m.From)
	msg.SetHeader("To", m.To)
	msg.SetHeader("Subject", m.Subject)

	body := m.formatMessage(message)
	msg.SetBody("text/plain", body)

	var dialer MailDialer
	if m.dialer != nil {
		dialer = m.dialer
	} else {
		d := mail.NewDialer(m.Host, m.Port, m.Username, m.Password)
		if m.TLS {
			d.TLSConfig = &tls.Config{
				ServerName:         m.Host,
				InsecureSkipVerify: false,
			}
		} else {
			d.TLSConfig = &tls.Config{
				ServerName:         m.Host,
				InsecureSkipVerify: true,
			}
		}
		dialer = d
	}

	if err := dialer.DialAndSend(msg); err != nil {
		m.logger.Error("Mail send failure", slog.String("error", err.Error()))
	}
}

func (m *Mail) formatMessage(message string) string {
	return fmt.Sprintf(`%s

Host: %s
User: %s
Working dir: %s

--
Dewy: https://github.com/linyows/dewy`, message, hostname(), username(), cwd())
}
