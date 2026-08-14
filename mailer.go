// Package mailer sends transactional email (confirmations, notifications,
// resets) from a Facile app over SMTP, and records messages in memory when
// used as a test seam. It owns transport and message shaping only: templates
// and send policy live in the app.
package mailer

import (
	"context"
	"errors"
	"strconv"
	"sync"
)

// ErrNotConfigured is returned by Send when the mailer has no SMTP host. The
// app decides whether that is fatal or a log-and-continue.
var ErrNotConfigured = errors.New("mailer: SMTP not configured")

// Config is the SMTP connection settings for one mailer. It mirrors the
// suite-wide SMTP_* environment convention (see FromEnv) so ops documents one
// set of variables for both the Go and TypeScript implementations.
type Config struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromName  string
	FromEmail string
}

// FromEnv builds a Config from the suite-wide SMTP_* variables. The default
// port is 587 when SMTP_PORT is absent or invalid.
func FromEnv(getenv func(string) string) Config {
	port, err := strconv.Atoi(getenv("SMTP_PORT"))
	if err != nil || port == 0 {
		port = 587
	}
	return Config{
		Host:      getenv("SMTP_HOST"),
		Port:      port,
		Username:  getenv("SMTP_USER"),
		Password:  getenv("SMTP_PASS"),
		FromName:  getenv("SMTP_FROM_NAME"),
		FromEmail: getenv("SMTP_FROM"),
	}
}

// Message is one outgoing email.
type Message struct {
	To      []string
	Subject string
	HTML    string
	Text    string
}

// Sender is the transport seam. The zero configuration needed to plug in a
// different sink (a queue, a provider API) is why the app-facing type is the
// interface, not the SMTP struct.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// Mailer sends messages through its Sender. New returns one with a real SMTP
// transport, NewMemory with an in-memory recorder for tests.
type Mailer struct {
	sender Sender
}

// New returns a Mailer that sends over SMTP. Sending without a configured
// host returns ErrNotConfigured rather than failing silently.
func New(cfg Config) *Mailer {
	return &Mailer{sender: &smtpSender{cfg: cfg}}
}

// Send delivers a message through the configured Sender.
func (m *Mailer) Send(ctx context.Context, msg Message) error {
	return m.sender.Send(ctx, msg)
}

// MemoryMailer records every message instead of sending. Tests assert on
// Messages. NewMemory returns one.
type MemoryMailer struct {
	*Mailer
	mem *memorySender
}

// NewMemory returns a mailer that records every message instead of sending.
func NewMemory() *MemoryMailer {
	mem := &memorySender{}
	return &MemoryMailer{Mailer: &Mailer{sender: mem}, mem: mem}
}

// Messages returns the recorded messages, in send order.
func (m *MemoryMailer) Messages() []Message {
	return m.mem.messages()
}

type memorySender struct {
	mu   sync.Mutex
	sent []Message
}

func (m *memorySender) Send(_ context.Context, msg Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	return nil
}

func (m *memorySender) messages() []Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Message, len(m.sent))
	copy(out, m.sent)
	return out
}
