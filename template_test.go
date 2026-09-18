package mailer

import (
	"errors"
	"strings"
	"testing"
)

func TestRenderTemplateWelcome(t *testing.T) {
	data := struct {
		Name string
	}{
		Name: "Alice",
	}

	html, text, err := RenderTemplate("welcome", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "Hi,") || !strings.Contains(html, "Alice") {
		t.Errorf("expected HTML to contain 'Hi,' and 'Alice', got: %s", html)
	}

	if !strings.Contains(text, "Hi, Alice") {
		t.Errorf("expected text to contain 'Hi, Alice', got: %s", text)
	}
}

func TestRenderTemplateMagicLink(t *testing.T) {
	data := map[string]string{
		"MagicLink": "https://example.com/magic?token=xyz",
	}

	html, text, err := RenderTemplate("magic-link", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "https://example.com/magic?token=xyz") {
		t.Errorf("expected HTML to contain magic link URL, got: %s", html)
	}

	if !strings.Contains(text, "Sign in to Facile") {
		t.Errorf("expected text to contain title, got: %s", text)
	}
}

func TestRenderTemplatePasswordReset(t *testing.T) {
	data := map[string]string{
		"ResetLink": "https://example.com/reset?token=123",
	}

	html, text, err := RenderTemplate("password-reset", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "Reset your password") {
		t.Errorf("expected HTML to contain title, got: %s", html)
	}

	if !strings.Contains(text, "https://example.com/reset?token=123") {
		t.Errorf("expected text to contain reset URL, got: %s", text)
	}
}

func TestRenderTemplateNotFound(t *testing.T) {
	_, _, err := RenderTemplate("nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent template, got nil")
	}

	if !errors.Is(err, ErrTemplateNotFound) {
		t.Errorf("expected ErrTemplateNotFound, got: %v", err)
	}
}

func TestRenderMessage(t *testing.T) {
	msg, err := RenderMessage([]string{"test@example.com"}, "Welcome aboard", "welcome", map[string]string{"Name": "Bob"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.Subject != "Welcome aboard" {
		t.Errorf("expected subject 'Welcome aboard', got: %s", msg.Subject)
	}

	if len(msg.To) != 1 || msg.To[0] != "test@example.com" {
		t.Errorf("expected To to be ['test@example.com'], got: %v", msg.To)
	}

	if !strings.Contains(msg.HTML, "Bob") {
		t.Errorf("expected HTML to contain 'Bob', got: %s", msg.HTML)
	}
}
