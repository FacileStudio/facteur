# facteur

Transactional mailer for the Facile Suite (Go): confirmations, notifications,
resets. `net/smtp` stdlib only, zero dependencies.

Sibling repo [facteur-ts](https://github.com/FacileStudio/facteur-ts) is the
TypeScript implementation (`@facile/facteur`). One `SMTP_*` env convention
across both.

## Tech Stack

- Go 1.24+, `net/smtp`, zero dependencies. Module `github.com/FacileStudio/facteur`,
  package `mailer` (import `mailer "github.com/FacileStudio/facteur"`).

## Layout

```
facteur/
  go.mod           # module github.com/FacileStudio/facteur
  mailer.go        # Config, FromEnv, Message, Sender, Mailer, MemoryMailer
  smtp.go          # smtpSender: dial, TLS/STARTTLS, auth, envelope
  message.go       # MIME body: normalize, stripHTML, headers, Message-ID
  mailer_test.go
  .github/workflows/filet.yml   # style gate + antenne alerts
  README.md
  CLAUDE.md
```

## Conventions

- **Env vars**: `SMTP_HOST`, `SMTP_PORT` (default 587), `SMTP_USER`, `SMTP_PASS`,
  `SMTP_FROM`, `SMTP_FROM_NAME`. Identical names in facteur-ts — one ops doc.
- **No host configured ⇒ refuse, don't swallow**: `ErrNotConfigured`. The app
  decides log-and-continue vs fail-closed.
- **Test seam**: `mailer.NewMemory()` records messages instead of sending.
  Tests assert on what would have been sent, no SMTP needed.
- **Text part always present**: empty `Text` is auto-filled by stripping HTML.
- **Scope**: transport + message shaping only. No templates, no queue, no
  per-owner config (Plume's `SmtpConfig` rows stay in Plume).

## Key Commands

```bash
go vet ./... && go test ./...
```

## Gotchas

- Port 465 = implicit TLS; any other port = plain + STARTTLS when offered.
  Plain auth (AUTH LOGIN) only when `SMTP_USER` is set.
- Package is `mailer`, not `facteur` — `import mailer "github.com/FacileStudio/facteur"`.
- Courrier's mail module is intentionally NOT built on this — it sends through
  per-user IMAP/SMTP credentials (an email client, not a transactional sender).
