package mailer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"time"
)

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
	to, err := parseRecipients(msg.To)
	if err != nil {
		return err
	}
	body, err := buildBody(msg, from, to)
	if err != nil {
		return err
	}
	client, err := connect(ctx, cfg)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := handshake(client, cfg); err != nil {
		return err
	}
	return deliver(client, from, to, body)
}

func parseRecipients(raw []string) ([]*mail.Address, error) {
	if len(raw) == 0 {
		return nil, errors.New("mailer: no recipients")
	}
	to := make([]*mail.Address, 0, len(raw))
	for _, recipient := range raw {
		addr, err := mail.ParseAddress(recipient)
		if err != nil {
			return nil, fmt.Errorf("mailer: invalid recipient %q: %w", recipient, err)
		}
		to = append(to, addr)
	}
	return to, nil
}

// connect dials the server and wraps the connection in an SMTP client.
func connect(ctx context.Context, cfg Config) (*smtp.Client, error) {
	conn, err := dial(ctx, cfg)
	if err != nil {
		return nil, err
	}
	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("mailer: smtp handshake: %w", err)
	}
	return client, nil
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

// handshake upgrades the connection to TLS when the server offers STARTTLS
// and authenticates when credentials are configured.
func handshake(client *smtp.Client, cfg Config) error {
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
	return nil
}

// deliver writes the envelope: MAIL FROM, one RCPT TO per recipient, the
// message body, then quits.
func deliver(client *smtp.Client, from *mail.Address, to []*mail.Address, body []byte) error {
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
