# modDNS Announcements (content branch)

This is an **orphan branch** that holds the user-facing announcements for
modDNS. It is a content-only channel, intentionally decoupled from the
application's gitflow: nothing here is application code, and this branch is
never merged into `develop`/`main`.

## How it works

- All announcements live in a single file: [`announcements.md`](announcements.md).
- The modDNS API fetches that file over HTTP (`ANNOUNCEMENTS_URL`), parses it,
  and serves it from the public `GET /api/v1/announcements` endpoint.
- The web app renders it at `/announcements`, visible to **logged-in and
  logged-out** users.

## Publishing

Open a pull request **targeting this `announcements` branch**. Every PR is
automatically validated by CI (it runs the same parser the API uses, so the
check matches exactly what will be accepted at runtime) — a malformed file or an
invalid record fails the check and blocks merge. On merge, the API picks up the
change within its reload interval (~5 minutes) — no app redeploy, no release
cycle.

> Merging here publishes to **every** modDNS user. This branch is protected;
> changes require review and a passing validation check.

## File format

One file, multiple records. Each record is a YAML *frontmatter* block (fenced by
`---`) followed by a Markdown body. Records are separated by the `---` fences.

```markdown
---
id: 2026-05-eu-maintenance        # required — stable, unique slug
category: maintenance             # required — see Categories below
severity: warning                 # required — info | warning | critical
title: Scheduled maintenance      # required
published_at: 2026-05-26          # required — YYYY-MM-DD or RFC3339 (hidden until this time)
expires_at: 2026-06-10            # optional — hidden after this time
pinned: false                     # optional — sorts to the top
link: https://status.moddns.net   # optional — must be an http(s) URL
---
Body in **Markdown** — lists, bold, and links are supported.

- Raw HTML is ignored (not rendered)
- Use *** for a horizontal rule (a bare --- starts a new record)
```

The body is optional — a record with only a title (and no body) is valid.

### Categories

`news` · `feature` · `maintenance` · `incident` · `security` · `policy`

(Category sets the badge/icon shown on the page.)

### Severity

`info` · `warning` · `critical` — drives how prominently the app surfaces an
item (e.g. an unread item shows a quiet dot in the nav; a `critical` one shows a
red dot).

### Display rules

- Sorted **pinned first, then newest** `published_at`.
- A record with a future `published_at` stays hidden until then; one past its
  `expires_at` is hidden.

#### Nav unread indicator

The Announcements nav item shows a dot to flag unread items:

- **A dot appears** whenever at least one announcement is *unread* — i.e.
  published after the user last opened the Announcements page (if they have
  never opened it, every published item is unread). Opening the page clears the
  dot.
- **Red dot** — at least one unread item has `critical` severity.
- **Brand-colour dot** — there are unread items but none are critical
  (`info` or `warning`).

Severity only changes the dot's **colour** (critical → red, otherwise brand
colour); the dot's **presence** is driven by unread state, not severity. A
`warning` item shows the same brand-colour dot as an `info` item — only
`critical` escalates to red.

### Size limit

The API reads at most **1 MB** of the feed; any content beyond that is silently
dropped at runtime, so trailing announcements would simply disappear. Keep the
file well under the cap by pruning expired entries — the validation CLI warns
(without failing) once the file passes **80%** of the limit.

## Validation

The API rejects the **entire** file (and keeps serving the last known-good
version) if any record:

- is missing a required field (`id`, `category`, `severity`, `title`,
  `published_at`),
- uses an unknown `category` or `severity`,
- reuses an `id` already used by another record,
- has a `link` that is not an absolute `http(s)` URL, or
- has malformed / unterminated frontmatter.

Bad edit can never take the announcements feed down — it just won't go
live until the file is valid. The same safety net covers upstream hiccups: if a
refresh fails (network error, non-200, timeout) the API keeps serving the last
good copy, and a load failure at startup never blocks the API — it just retries
on the next reload.

These exact checks also run in CI on every pull request (see
`.github/workflows/validate-announcements.yml`), so an invalid file is caught
before merge rather than silently failing to publish.
