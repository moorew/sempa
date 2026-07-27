# Backups

Sempa builds a zip of the database (plus attachments) and pushes it to one or
more destinations. Configure it in **Settings → Backup**.

Destinations: **Google Drive**, **S3-compatible**, **WebDAV / Nextcloud**, and a
**local folder**. More than one can be enabled; each gets its own copy and its
own retention.

---

## The Google Drive 7-day expiry (read this before debugging)

**Symptom.** Backups run fine for a week, then every scheduled run fails with:

```
backup scheduler: run failed  err="Google authorization expired — please reconnect the account"
```

Reconnecting fixes it — for exactly seven more days.

**This is not a bug in Sempa.** The OAuth request is already correct: it sends
`access_type=offline` and `prompt=consent` (`internal/integrations/gmail/oauth.go`),
which is what earns a long-lived refresh token.

**The cause** is the Google Cloud OAuth consent screen being left in **"Testing"**
publishing status. Google expires refresh tokens issued by a Testing app after
**7 days**, unconditionally. No amount of retrying or re-requesting works around
it.

**The fix** — once, permanently:

1. Google Cloud Console → **APIs & Services → OAuth consent screen**
2. Publishing status: **Testing** → **Publish app** ("In production")
3. Reconnect Drive in Sempa (Settings → Backup → Connect Google Drive)

Refresh tokens stop expiring on a clock from that point on.

**About the "unverified app" warning.** A published-but-unverified app shows a
"Google hasn't verified this app" interstitial on the consent screen; click
**Advanced → Go to Sempa (unsafe)** to continue. Verification only removes that
warning and lifts the 100-user cap — neither matters for a single-user
self-hosted install, and it does **not** affect token lifetime.

**Scope.** Backups use `drive.file`, the least-privileged Drive scope: Sempa can
only ever see files it created itself, never the rest of your Drive.

### Avoiding Google entirely

If you'd rather not depend on Google at all, enable an **S3** or
**WebDAV / Nextcloud** destination instead — or alongside Drive, so one
provider failing never leaves you with zero backups.

---

## Knowing when backups break

A backup that has silently stopped is the worst kind of failure: you find out
when you need it. Sempa surfaces it in three places:

- **In-app banner** — appears app-wide when the last run failed, with a
  **Reconnect** action when the cause is an expired token. Dismissing it lasts
  for the session only; it returns until backups actually succeed again.
- **Settings → Backup** — per-destination status and recent run history.
- **Server log** — `backup scheduler: run failed`, with the underlying error.

The banner is driven by `GET /api/v1/backup/health`, a pure database read of the
last recorded run. It deliberately does **not** probe Google: that call
(`GET /api/v1/backup/drive`) makes a live token request and is far too expensive
to poll. The last failure's reason is stored as `backup_settings.last_error_code`
(`reauth_required` for an expired token) so the UI can act on it without
string-matching an error message.

Reconnecting Drive clears the stale failure state, so the banner goes away as
soon as you've done what it asked rather than waiting for the next nightly run.
Use **Run now** in Settings → Backup if you want immediate confirmation.

---

## Restoring

Settings → Backup → **Restore** takes a backup zip and replaces the current
database. If the backup was encrypted you'll need its passphrase — it is never
stored inside the backup itself.

Restores rewrite the whole database. Take a fresh backup first.
