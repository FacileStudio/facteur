package mailer

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"regexp"
	"strings"
	"time"
)

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
	h := messageHeaders(msg, from, to, mw.Boundary())
	for key, values := range h {
		for _, value := range values {
			fmt.Fprintf(&buf, "%s: %s\r\n", key, value)
		}
	}
	fmt.Fprint(&buf, "\r\n")

	if err := writePart(mw, "text/plain; charset=\"utf-8\"", msg.Text); err != nil {
		return nil, err
	}
	if err := writePart(mw, "text/html; charset=\"utf-8\"", msg.HTML); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("mailer: close multipart: %w", err)
	}
	return buf.Bytes(), nil
}

func messageHeaders(msg Message, from *mail.Address, to []*mail.Address, boundary string) textproto.MIMEHeader {
	h := textproto.MIMEHeader{}
	h.Set("From", from.String())
	h.Set("To", joinAddresses(to))
	h.Set("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	h.Set("Date", time.Now().UTC().Format(time.RFC1123Z))
	h.Set("Message-ID", newMessageID(from.Address))
	h.Set("MIME-Version", "1.0")
	h.Set("Content-Type", "multipart/alternative; boundary="+boundary)
	return h
}

func writePart(mw *multipart.Writer, contentType, body string) error {
	part, err := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {contentType}})
	if err != nil {
		return fmt.Errorf("mailer: %s part: %w", contentType, err)
	}
	if _, err := part.Write([]byte(body)); err != nil {
		return fmt.Errorf("mailer: write %s part: %w", contentType, err)
	}
	return nil
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
