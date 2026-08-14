# facteur

Transactional mailer for the Facile Suite (Go) — confirmations, notifications,
resets. Package `mailer`, `net/smtp` stdlib only, zero dependencies.

The TypeScript sibling is [facteur-ts](https://github.com/FacileStudio/facteur-ts)
(`@facile/facteur`). Both implementations speak the same `SMTP_*` environment
convention, so ops documents one set of variables.

Transport and message shaping only. Templates and send policy (log-and-continue
vs fail-closed) live in the app, not here. Courrier's mail module stays in
Courrier: it sends through each user's own IMAP/SMTP credentials, which is a
product feature, not shared plumbing.

## Environment

| Variable | Meaning |
|---|---|
| `SMTP_HOST` | SMTP server hostname. Absent ⇒ mailer refuses to send |
| `SMTP_PORT` | Port (default 587; 465 gets implicit TLS, otherwise STARTTLS) |
| `SMTP_USER` | Username (optional; empty = no auth) |
| `SMTP_PASS` | Password |
| `SMTP_FROM` | From address, e.g. `noreply@facile.studio` |
| `SMTP_FROM_NAME` | Display name |

## Install

Go consumers pin the pseudo-version of a commit — there are no semver tags.

```sh
go get github.com/FacileStudio/facteur@latest
```

Before the repo is pushed / for local iteration, use a replace directive:

```
require github.com/FacileStudio/facteur v0.0.0
replace github.com/FacileStudio/facteur => ../facteur
```

## Usage

```go
import "github.com/FacileStudio/facteur"

m := mailer.New(mailer.FromEnv(os.Getenv))

err := m.Send(ctx, mailer.Message{
    To:      []string{"someone@example.com"},
    Subject: "Your confirmation code",
    HTML:    "<p>Code: <strong>482913</strong></p>",
})
if err != nil {
    slog.Error("failed to send confirmation", slog.Any("error", err))
}
```

Sending with no `SMTP_HOST` returns `mailer.ErrNotConfigured` — the app decides
whether that is fatal or a log-and-continue. Tests use the in-memory seam:

```go
m := mailer.NewMemory()
_ = m.Send(ctx, mailer.Message{To: []string{"a@example.com"}, Subject: "hi", HTML: "<p>hi</p>"})
got := m.Messages() // assert on what would have been sent
```

An empty `Text` is filled from the HTML (tags stripped) so every message
carries a plain-text part. The message is a multipart/alternative MIME body
with RFC 2047-encoded headers, Message-ID and Date.

## Development

```sh
go vet ./... && go test ./...
```
