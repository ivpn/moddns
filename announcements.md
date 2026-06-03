<!--
LOCAL DEV fixture for the Announcements feature. Not committed to the app.
Serve over HTTP and point ANNOUNCEMENTS_URL at it (see instructions).
Use *** for in-body horizontal rules; a bare --- delimits records.
-->

---
id: dev-missing-title
category: incident
severity: critical
published_at: 2026-05-20

---
Missing title field  should be rejected by parser.

---
id: dev-invalid-severity
category: news
severity: high
title: Invalid severity test
published_at: 2026-05-20

---
Severity is invalid (must be info | warning | critical).

---
id: dev-invalid-category
category: breaking-news
severity: info
title: Invalid category test
published_at: 2026-05-20

---
This should fail because category is not in the allowed set.

---
id: dev-future-001
category: news
severity: info
title: Future announcement (should be hidden)
published_at: 2099-01-01

---
This should NOT appear yet.

---
id: dev-expired-001
category: maintenance
severity: warning
title: Expired announcement (should disappear)
published_at: 2025-01-01
expires_at: 2025-02-01
---
This should be hidden because it's expired.

---
id: dev-maintenance-001
category: maintenance
severity: warning
title: Scheduled infrastructure maintenance
published_at: 2026-05-20
pinned: true
link: https://ivpn.net

---
We will perform routine maintenance on core DNS services.

- Expected minor latency during the window
- No downtime expected

---
id: dev-feature-001
category: feature
severity: info
title: New dashboard rollout
published_at: 2026-05-18

---
We are rolling out a new analytics dashboard for modDNS users.

- Improved latency charts
- Better zone visibility

---
id: dev-incident
category: incident
severity: critical
title: Testing news feed
published_at: 2026-06-02
pinned: true
link: https://status.moddns.net

---
Testing news feed update for modDNS staging environment

- Test
- Test

---
id: dev-security
category: security
severity: warning
title: Rotate API credentials
published_at: 2026-05-27
---
As a precaution we recommend rotating any long-lived API credentials.

---
id: dev-maintenance
category: maintenance
severity: warning
title: Scheduled maintenance ??? EU resolvers
published_at: 2026-05-26
expires_at: 2026-06-10
link: https://status.moddns.net
---
Maintenance on **28 May, 22:00???23:00 UTC**. Brief failovers expected, no action required.

---
id: dev-feature
category: feature
severity: info
title: Three new blocklists available
published_at: 2026-05-25
link: https://moddns.net/blog/new-blocklists
---
Enable AdGuard Tracking, OISD Big, and HaGeZi Pro from your profile's Blocklists tab.

---
id: dev-news
category: news
severity: info
title: modDNS is out of beta
published_at: 2026-05-24
---
Thanks for being an early user. Read what changed and what's next.

---
id: dev-policy
category: policy
severity: info
title: Updated Terms of Service
published_at: 2026-05-20
---
Our Terms of Service have been updated. Continued use constitutes acceptance.

---
id: dev-expired
category: maintenance
severity: warning
title: (should be hidden) past maintenance
published_at: 2026-05-01
expires_at: 2026-05-10
---
This entry is expired and must NOT appear in the list.

---
id: dev-future
category: news
severity: info
title: (should be hidden) future post
published_at: 2026-12-01
---
This entry is scheduled for the future and must NOT appear yet.
