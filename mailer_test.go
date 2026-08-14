package mailer

import (
	"context"
	"net/mail"
	"strings"
	"testing"
)

func TestMemoryMailerRecords(t *testing.T) {
	m := NewMemory()
	msg := Message{To: []string{"a@example.com"}, Subject: "hello", HTML: "<p>hi</p>"}
	if err := m.Send(context.Background(), msg); err != nil {
		t.Fatalf("send: %v", err)
	}
	got := m.Messages()
	if len(got) != 1 {
		t.Fatalf("recorded %d messages, want 1", len(got))
	}
	if got[0].Subject != "hello" {
		t.Errorf("subject = %q, want hello", got[0].Subject)
	}
}

func TestErrNotConfigured(t *testing.T) {
	m := New(Config{})
	err := m.Send(context.Background(), Message{To: []string{"a@example.com"}})
	if err != ErrNotConfigured {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
}

func TestNormalizeFillsTextFromHTML(t *testing.T) {
	msg := normalizeMessage(Message{HTML: "<p>hello <b>world</b></p><p>line two</p>"})
	want := "hello world line two"
	if msg.Text != want {
		t.Errorf("text = %q, want %q", msg.Text, want)
	}
}

func TestNormalizeKeepsExplicitText(t *testing.T) {
	msg := normalizeMessage(Message{HTML: "<p>hi</p>", Text: "plain"})
	if msg.Text != "plain" {
		t.Errorf("text = %q, want plain", msg.Text)
	}
}

func TestBuildBodyIsMultipartAlternative(t *testing.T) {
	from := mustParseAddress(t, "Facteur <noreply@facile.studio>")
	to := mustParseAddress(t, "someone@example.com")
	body, err := buildBody(
		Message{To: []string{"someone@example.com"}, Subject: "héllo wörld", HTML: "<p>hi</p>", Text: "hi"},
		from, []*mail.Address{to},
	)
	if err != nil {
		t.Fatalf("buildBody: %v", err)
	}
	raw := string(body)
	for _, want := range []string{"multipart/alternative", "text/plain", "text/html", "=?utf-8?q?"} {
		if !strings.Contains(raw, want) {
			t.Errorf("body missing %q:\n%s", want, raw)
		}
	}
	if strings.Contains(raw, from.Address+"<") || strings.Contains(raw, to.Address+"<") {
		t.Errorf("header injection: addresses appear raw in headers:\n%s", raw)
	}
}

func TestFromEnvDefaults(t *testing.T) {
	env := map[string]string{
		"SMTP_HOST": "smtp.example.com", "SMTP_USER": "u",
		"SMTP_PASS": "p", "SMTP_FROM": "a@example.com",
	}
	cfg := FromEnv(func(k string) string { return env[k] })
	if cfg.Host != "smtp.example.com" || cfg.Port != 587 || cfg.Username != "u" ||
		cfg.Password != "p" || cfg.FromEmail != "a@example.com" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestFromEnvInvalidPortFallsBack(t *testing.T) {
	env := map[string]string{"SMTP_PORT": "notaport"}
	cfg := FromEnv(func(k string) string { return env[k] })
	if cfg.Port != 587 {
		t.Errorf("port = %d, want 587", cfg.Port)
	}
}

func mustParseAddress(t *testing.T, raw string) *mail.Address {
	t.Helper()
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return addr
}
