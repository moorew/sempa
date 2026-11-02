# Files: what Sempa accepts and how it handles them

Sempa takes files in four places. This page lists what each accepts, the size
limits, and how a file is shown when you open it.

## Attachments (tasks, objectives, projects)

**Any file type is accepted, up to 500 MB per file.** Add files with **+ Add
file** or by dropping them on a task, objective or project. Files are stored on
your server (`ATTACHMENTS_DIR`, or `/data/attachments` in Docker) and are
included in backups.

What happens when you open an attachment depends on its type, because a file
shown inside the app runs on Sempa's own web address:

| Type | Opens as |
|---|---|
| Images: PNG, JPEG, GIF, WebP, BMP, ICO, TIFF | Shown in the browser |
| PDF | Shown in the browser's PDF viewer |
| Plain text, CSV, Markdown | Shown as text |
| Audio and video (`audio/*`, `video/*`) | Played in the browser |
| **Everything else** (including HTML, SVG, XML, JavaScript, Office documents, archives, executables, and anything unrecognised) | **Downloaded**, never shown in the app |

Notes:

- **SVG is always downloaded.** An SVG can contain script, so showing one inside
  the app could act with your login. Convert it to PNG if you want a preview.
- The type comes from the file's name and what the uploading device reports.
  Sempa never guesses ("sniffs") a different type, and anything it can't classify
  is treated as a download.
- Downloads that aren't shown inline are also served with a sandbox policy, so
  even a browser that ignored the download instruction couldn't run them.
- Who can open an attachment follows its task, objective or project: if you can
  see the item, you can see its files. Making the item private hides them too.
- Sempa doesn't scan files for malware. Treat attachments from other people like
  any email attachment.

## Link previews

When a task note contains a link, Sempa can show a preview card. The preview
image is fetched by your server and passed through, but **only plain image
formats** are used (PNG, JPEG, GIF, WebP, AVIF, BMP, ICO), up to 5 MB. SVG and
anything else is refused and the card falls back to the site's icon. Your server
only fetches public addresses for previews: never your local network, your
tailnet (Tailscale addresses), or the server itself.

## Backup restore

**Settings → Backup → Restore** accepts a Sempa backup bundle: a `.zip` (or
`.enc` for an encrypted bundle), up to about 560 MB. Restore is admin-only and
replaces all data. Bundles never contain login tokens, so after a restore
everyone signs in again on any device whose session predates it. See
[BACKUP.md](BACKUP.md).

## Email

Mail that becomes a task (forwarded, starred, or sent to the built-in
receiver) is read for its **subject and text only**, up to 25 MB per message.
**Email attachments are not saved**, and links found in the message are kept
only if they are ordinary `http(s)` web links. See [EMAIL.md](EMAIL.md).

## Sharing into Sempa (Android)

The Android share sheet accepts **text and links** (`text/plain`), which open
a new task to review before saving. Sharing a file or photo into Sempa isn't
supported; attach it from the task instead.
