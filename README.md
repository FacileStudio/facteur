# facteur

The transactional mailer for the [Facile Suite](https://facile.studio) — the SMTP plumbing
every app that sends confirmations, notifications or resets needs, and none of them should be
re-deciding.

The Go family once carried its own mail code where it carried any at all: Plume's per-owner
`modules/smtp` is the only real sender in the suite, and the other seven Go apps have no
app-initiated transactional mail — so the day an app needs a "you've been invited" or a reset link, it would have copy-pasted a
fourth or fifth variant of `net/smtp` ceremony. `facteur` is the single sender, plus the
in-memory test seam none of the copies had. The TypeScript sibling is
[facteur-ts](https://github.com/FacileStudio/facteur-ts); both speak the same `SMTP_*`
environment convention, so ops documents one set of variables.

Transport and message shaping only. Templates and send policy (log-and-continue vs fail-closed)
live in the app, not here. Courrier's mail module stays in Courrier: it sends through each
user's own IMAP/SMTP credentials, which is a product feature, not shared plumbing.

## What it does

- Sends transactional email over SMTP with the standard library only — zero dependencies
- Speaks the suite's `SMTP_*` environment convention, identical names to facteur-ts
- Builds a multipart/alternative message with a plain-text part derived from the HTML when you
  do not supply one, so every email carries both
- Encodes subjects RFC 2047, stamps Message-ID and Date, and connects with a 10-second
  context-aware timeout
- Uses implicit TLS on port 465, STARTTLS on other ports when offered, and plain auth only when
  a username is set
- Refuses to send when SMTP is not configured — `ErrNotConfigured` — and leaves the policy
  (fatal or log-and-continue) to the app
- Gives tests an in-memory mailer that records what would have been sent, no SMTP server needed

## Stack

| Layer | Tech |
|---|---|
| Runtime | Go 1.24, `net/smtp` stdlib — nothing else |

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

For local iteration, use a replace directive:

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

Sending with no `SMTP_HOST` returns `mailer.ErrNotConfigured` — the app decides whether that is
fatal or a log-and-continue. Tests use the in-memory seam:

```go
m := mailer.NewMemory()
_ = m.Send(ctx, mailer.Message{To: []string{"a@example.com"}, Subject: "hi", HTML: "<p>hi</p>"})
got := m.Messages() // assert on what would have been sent
```

## Development

```sh
go vet ./... && go test ./...
```
