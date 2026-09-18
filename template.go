package mailer

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	htmltmpl "html/template"
	texttmpl "text/template"
)

//go:embed templates/*.html templates/*.txt
var defaultTemplateFS embed.FS

var ErrTemplateNotFound = errors.New("mailer: template not found")

// Renderer executes HTML and plaintext email templates.
type Renderer struct {
	htmlTemplates map[string]*htmltmpl.Template
	textTemplates map[string]*texttmpl.Template
}

// NewRenderer creates a Renderer with embedded default templates.
func NewRenderer() (*Renderer, error) {
	return NewRendererFromFS(defaultTemplateFS, "templates")
}

// NewRendererFromFS creates a Renderer from an embed.FS and directory.
func NewRendererFromFS(fs embed.FS, dir string) (*Renderer, error) {
	r := &Renderer{
		htmlTemplates: make(map[string]*htmltmpl.Template),
		textTemplates: make(map[string]*texttmpl.Template),
	}

	for _, name := range []string{"welcome", "magic-link", "password-reset"} {
		htmlPattern := fmt.Sprintf("%s/%s.html", dir, name)
		htmlTmpl, err := htmltmpl.ParseFS(fs, htmlPattern)
		if err != nil {
			return nil, fmt.Errorf("mailer: parse html template %q: %w", name, err)
		}
		r.htmlTemplates[name] = htmlTmpl

		textPattern := fmt.Sprintf("%s/%s.txt", dir, name)
		textTmpl, err := texttmpl.ParseFS(fs, textPattern)
		if err != nil {
			return nil, fmt.Errorf("mailer: parse text template %q: %w", name, err)
		}
		r.textTemplates[name] = textTmpl
	}

	return r, nil
}

// Render executes both the HTML and plaintext versions of a named template with data.
func (r *Renderer) Render(name string, data any) (string, string, error) {
	htmlTmpl, ok := r.htmlTemplates[name]
	if !ok {
		return "", "", fmt.Errorf("%w: %s", ErrTemplateNotFound, name)
	}

	textTmpl, ok := r.textTemplates[name]
	if !ok {
		return "", "", fmt.Errorf("%w: %s", ErrTemplateNotFound, name)
	}

	var htmlBuf bytes.Buffer
	if err := htmlTmpl.Execute(&htmlBuf, data); err != nil {
		return "", "", fmt.Errorf("mailer: render html template %q: %w", name, err)
	}

	var textBuf bytes.Buffer
	if err := textTmpl.Execute(&textBuf, data); err != nil {
		return "", "", fmt.Errorf("mailer: render text template %q: %w", name, err)
	}

	return htmlBuf.String(), textBuf.String(), nil
}

// RenderTemplate renders the embedded named template into HTML and text strings.
func RenderTemplate(name string, data any) (string, string, error) {
	r, err := NewRenderer()
	if err != nil {
		return "", "", err
	}
	return r.Render(name, data)
}

// RenderMessage builds a Message by rendering the named template with data.
func RenderMessage(to []string, subject string, templateName string, data any) (Message, error) {
	html, text, err := RenderTemplate(templateName, data)
	if err != nil {
		return Message{}, err
	}
	return Message{
		To:      to,
		Subject: subject,
		HTML:    html,
		Text:    text,
	}, nil
}
