<!--
modDNS announcements feed — a single file, fetched and served by the modDNS API.
Each record is a YAML frontmatter block (--- fenced) followed by a Markdown body;
records are split on the --- fences. Use *** for in-body horizontal rules.
See README.md on this branch for the full format and publishing guide.
-->

---
id: 2026-06-blocklists-services-dnssec
category: feature
severity: info
title: Blocklists, services, and DNSSEC updates
published_at: 2026-06-18
---
New additions:

- [Security tab](https://github.com/ivpn/moddns/pull/162) in Blocklists: two malware and phishing lists and a Newly Registered Domain setting.
- Four general blocklists, including 1Hosts, someonewhocares, and Peter Lowe.
- Seven Services options, including Twitter/X, TikTok, and Reddit.
- Fixed earlier DNSSEC validation issues.

---
id: 2026-06-news-feed-and-faster-web-app
category: feature
severity: info
title: News feed and a faster web app
published_at: 2026-06-12
---
We will use this newly added [announcements feed](https://github.com/ivpn/moddns/pull/153) for service updates, infrastructure additions such as new endpoints, and maintenance notifications.

We also improved the web app's [responsiveness](https://github.com/ivpn/moddns/issues/111). Navigation should be smoother across tabs, toggles, and dialogs.
