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

Open a pull request **targeting this `announcements` branch**. On merge, the API
picks up the change within its reload interval (~5 minutes) — no app redeploy,
no release cycle.

> Merging here publishes to **every** modDNS user. This branch is protected;
> changes require review.

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

## Validation

The API rejects the **entire** file (and keeps serving the last known-good
version) if any record:

- is missing a required field (`id`, `category`, `severity`, `title`,
  `published_at`),
- uses an unknown `category` or `severity`,
- reuses an `id` already used by another record, or
- has malformed / unterminated frontmatter.

Bad edit can never take the announcements feed down — it just won't go
live until the file is valid.
