<!--
modDNS announcements feed — a single file, fetched and served by the modDNS API.
Each record is a YAML frontmatter block (--- fenced) followed by a Markdown body;
records are split on the --- fences. Use *** for in-body horizontal rules.
See README.md on this branch for the full format and publishing guide.
-->

---
id: 2026-07-query-logs-dnscrypt-stamps-rebinding
category: feature
severity: info
title: Query logs improvements, dnscrypt-proxy support, DNS rebinding protection
published_at: 2026-07-30
---
Latest additions:

- [Query logs](https://moddns.net/query-logs) now consolidate duplicate entries, and each card expands to show query details, block reasons, and a "no answer" label when a domain cannot be resolved.
- dnscrypt support: reach modDNS from a dnscrypt-proxy client over DoH. See the [Linux setup guide](https://moddns.net/setup).
- DNS Stamps: generate a stamp to configure network devices that support the format. See the [Routers setup guide](https://moddns.net/setup).
- DNS rebinding protection blocks public domains that resolve to private IP addresses. Enable it in the [Blocklists Security tab](https://moddns.net/blocklists).

---
id: 2026-07-locations-ipv6-export-import
category: feature
severity: info
title: New PoP locations, IPv6, profile export/import, custom rules improvements
published_at: 2026-07-09
---
New in modDNS:

- Added two new PoP locations, New York (nys1.dns.moddns.net) and Los Angeles (lax1.dns.moddns.net), to improve latency for users in the US.
- modDNS now supports IPv6. See the [setup guides](https://moddns.net/setup) for details.
- Export your profile configuration from [Account preferences](https://moddns.net/account-preferences) and re-import it later.
- Custom rules can now be grouped, with notes and reordering.

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
