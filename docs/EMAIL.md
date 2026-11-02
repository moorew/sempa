# Email → tasks

There are four ways to turn email into tasks. Most people only need one.

| Way | Best for | Set up in |
|---|---|---|
| **Gmail / Fastmail import** | Starring mail you already have | Settings → Integrations |
| **Task inbox** | A dedicated address you forward mail to | Settings → Integrations → Task inbox |
| **Cloudflare webhook** | Your own domain, routed through Cloudflare Email Routing | `install.sh` or `EMAIL_FORWARD_TOKEN` |
| **Built-in SMTP receiver** | Advanced: a mail server you run that can relay to Sempa | `SMTP_PORT` + `SMTP_RECIPIENT_TOKEN` |

For every route, Sempa reads the subject and the message text (up to 25 MB per
message), drops attachments, and keeps only `http(s)` links. Integrations and
their settings can only be changed by an admin.

## Cloudflare webhook

`install.sh` asks **Enable email-to-task?** and generates a token for this. To
set it up by hand, put a long random value in `.env.local`:

```env
EMAIL_FORWARD_TOKEN=<random secret>
```

Then point a Cloudflare Email Worker at `https://<your-sempa>/api/v1/tasks/from-email`
with the header `Authorization: Bearer <token>`. To limit who can create tasks,
restrict the senders in your Cloudflare Email Routing rule; Sempa accepts any
message the worker posts with the right token.

## Built-in SMTP receiver

Since the November 2026 security release the receiver is **off by default**. It
used to listen on port 2525 on every network interface and accept mail from
anyone, so anything on your LAN could create tasks.

Plain SMTP has no login here, and the sender address is whatever the
connecting machine claims, so a sender allowlist alone proves nothing. The
receiver therefore requires a **secret recipient token**: mail is accepted only
when the recipient address carries it.

To turn it on, add to `.env.local` and restart (`docker compose up -d`):

```env
SMTP_PORT=2525
SMTP_RECIPIENT_TOKEN=<random secret, e.g. from: openssl rand -hex 16>
# Optional extra filter. Not authentication on its own.
SMTP_ALLOWED_SENDERS=you@example.com,@yourcompany.com
```

Then send or relay mail to either form of the address:

```text
tasks+<token>@<your-sempa-host>
<token>@<your-sempa-host>
```

The part after `@` doesn't matter to Sempa; only the token does. Mail to any
other recipient is refused with `550 no such mailbox`. If `SMTP_PORT` is set but
the token is missing, the receiver doesn't start and the server logs
`smtp disabled: … SMTP_RECIPIENT_TOKEN is empty`.

The receiver listens on every interface of the Sempa host (in the shipped
compose file that is the host's network). Keep the port off the internet with a
firewall, and treat the token like a password; changing it means updating
whatever relays mail to Sempa.

### Upgrading from an earlier version

If you used the built-in receiver before (mail sent to `tasks@<host>:2525`),
it stops after upgrading until you set `SMTP_PORT` and `SMTP_RECIPIENT_TOKEN` as
above and update your relay to the new address. Nothing else changes.
