// Package mailer sends transactional email (confirmations, notifications,
// resets) from a Facile app over SMTP, and records messages in memory when
// used as a test seam. It owns transport and message shaping only: templates
// and send policy live in the app.
package mailer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
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

// NewMemory returns a Mailer that records every message instead of sending.
// Tests assert on Messages.
func NewMemory() *Mailer {
	return &Mailer{sender: &memorySender{}}
}

// Send delivers a message through the configured Sender.
func (m *Mailer) Send(ctx context.Context, msg Message) error {
	return m.sender.Send(ctx, msg)
}

// Messages returns the recorded messages of a memory mailer. It panics if the
// mailer is not a memory mailer, which is a test wiring mistake, not a
// runtime path.
func (m *Mailer) Messages() []Message {
	mem, ok := m.sender.(*memorySender)
	if !ok {
		panic("mailer: Messages called on a non-memory mailer")
	}
	return mem.messages()
}

type smtpSender struct {
	cfg Config
}

func (s *smtpSender) Send(ctx context.Context, msg Message) error {
	cfg := s.cfg
	if cfg.Host == "" {
		return ErrNotConfigured
	}
	from, err := mail.ParseAddress(cfg.FromEmail)
	if err != nil {
		return fmt.Errorf("mailer: invalid from address %q: %w", cfg.FromEmail, err)
	}
	msg = normalizeMessage(msg)
	if len(msg.To) == 0 {
		return errors.New("mailer: no recipients")
	}
	to := make([]*mail.Address, 0, len(msg.To))
	for _, raw := range msg.To {
		addr, err := mail.ParseAddress(raw)
		if err != nil {
			return fmt.Errorf("mailer: invalid recipient %q: %w", raw, err)
		}
		to = append(to, addr)
	}

	body, err := buildBody(msg, from, to)
	if err != nil {
		return err
	}

	conn, err := dial(ctx, cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return fmt.Errorf("mailer: smtp handshake: %w", err)
	}
	defer client.Close()

	if cfg.Port != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
				return fmt.Errorf("mailer: starttls: %w", err)
			}
		}
	}

	if cfg.Username != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("mailer: authentication: %w", err)
		}
	}

	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("mailer: from %s: %w", from.Address, err)
	}
	for _, addr := range to {
		if err := client.Rcpt(addr.Address); err != nil {
			return fmt.Errorf("mailer: recipient %s: %w", addr.Address, err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("mailer: data: %w", err)
	}
	if _, err := writer.Write(body); err != nil {
		return fmt.Errorf("mailer: write body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("mailer: close body: %w", err)
	}
	return client.Quit()
}

// dial connects to the SMTP server. Port 465 gets implicit TLS; every other
// port starts plain and relies on STARTTLS negotiation later.
func dial(ctx context.Context, cfg Config) (net.Conn, error) {
	d := net.Dialer{Timeout: 10 * time.Second}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("mailer: connect %s: %w", addr, err)
	}
	if cfg.Port != 465 {
		return conn, nil
	}
	tconn := tls.Client(conn, &tls.Config{ServerName: cfg.Host})
	if err := tconn.HandshakeContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("mailer: tls handshake: %w", err)
	}
	return tconn, nil
}

// normalizeMessage fills an empty Text from the HTML so every email carries a
// plain-text part.
func normalizeMessage(msg Message) Message {
	if msg.Text == "" {
		msg.Text = stripHTML(msg.HTML)
	}
	return msg
}

var tagRe = regexp.MustCompile(`<[^>]*>`)

func stripHTML(s string) string {
	return strings.Join(strings.Fields(tagRe.ReplaceAllString(s, " ")), " ")
}

// buildBody renders the wire bytes: RFC 2047-encoded headers plus a
// multipart/alternative body with the text part first, per RFC 2046.
func buildBody(msg Message, from *mail.Address, to []*mail.Address) ([]byte, error) {
	var buf bytes.Buffer

	mw := multipart.NewWriter(&buf)
	h := textproto.MIMEHeader{}
	h.Set("From", from.String())
	h.Set("To", joinAddresses(to))
	h.Set("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	h.Set("Date", time.Now().UTC().Format(time.RFC1123Z))
	h.Set("Message-ID", newMessageID(from.Address))
	h.Set("MIME-Version", "1.0")
	h.Set("Content-Type", "multipart/alternative; boundary="+mw.Boundary())
	for key, values := range h {
		for _, value := range values {
			fmt.Fprintf(&buf, "%s: %s\r\n", key, value)
		}
	}
	fmt.Fprint(&buf, "\r\n")

	textPart, err := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain; charset=\"utf-8\""}})
	if err != nil {
		return nil, fmt.Errorf("mailer: text part: %w", err)
	}
	if _, err := textPart.Write([]byte(msg.Text)); err != nil {
		return nil, fmt.Errorf("mailer: write text part: %w", err)
	}

	htmlPart, err := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/html; charset=\"utf-8\""}})
	if err != nil {
		return nil, fmt.Errorf("mailer: html part: %w", err)
	}
	if _, err := htmlPart.Write([]byte(msg.HTML)); err != nil {
		return nil, fmt.Errorf("mailer: write html part: %w", err)
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("mailer: close multipart: %w", err)
	}
	return buf.Bytes(), nil
}

func joinAddresses(addrs []*mail.Address) string {
	parts := make([]string, 0, len(addrs))
	for _, a := range addrs {
		parts = append(parts, a.String())
	}
	return strings.Join(parts, ", ")
}

func newMessageID(domain string) string {
	at := strings.LastIndex(domain, "@")
	if at >= 0 {
		domain = domain[at+1:]
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("<%d@%s>", time.Now().UnixNano(), domain)
	}
	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(raw[:]), domain)
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
