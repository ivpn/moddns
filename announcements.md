<!--
LOCAL DEV fixture for the Announcements feature. Not committed to the app.
Serve over HTTP and point ANNOUNCEMENTS_URL at it (see instructions).
Use *** for in-body horizontal rules; a bare --- delimits records.
-->

---
id: dev-maint-006
category: feature
severity: info
title: DNS announcements
published_at: 2026-06-04
---

- First announcement 

---
id: dev-maint-005
category: maintenance
severity: warning
title: Core DNS maintenance window
published_at: 2026-06-03
pinned: true
expires_at: 2026-06-04
---
Short maintenance window scheduled to improve system reliability.
- Emojis: 🚀 ⚠️ ✅
- Accents: naïve, façade, São Paulo
- Non-Latin: こんにちは, 안녕하세요, Привет
- Markdown: **bold**, *italic*, `code`

---
id: dev-feature-005
category: feature
severity: info
title: Introduced smarter caching strategy
published_at: 2026-05-01
---
Caching improvements reduce average response time across regions.

---
id: dev-incident-005
category: incident
severity: critical
title: Brief outage in APAC region (resolved)
published_at: 2026-05-03
---
A short outage affected APAC traffic. Service has been fully restored.

---
id: dev-security-005
category: security
severity: critical
title: Emergency security patch deployed
published_at: 2026-05-04
---
Security update deployed with no service interruption.

---
id: dev-news-005
category: news
severity: info
title: System health improvements completed
published_at: 2026-05-05
---
General system health improvements rolled out successfully.

---
id: dev-policy-005
category: policy
severity: warning
title: Updated acceptable usage policy
published_at: 2026-05-06
---
Clarifications added to API usage limits and abuse prevention rules.

---
id: dev-news-004
category: news
severity: info
title: Monthly platform update released
published_at: 2026-05-01
pinned: true
---
This update includes minor performance improvements and UI refinements.

---
id: dev-policy-004
category: policy
severity: warning
title: Updated API usage guidelines
published_at: 2026-05-03
---
We have updated API usage guidelines to improve fairness and stability.

Please review updated documentation before scaling usage.

---
id: dev-security-004
category: security
severity: critical
title: Security patch applied to resolver layer
published_at: 2026-05-04
---
A security patch has been applied to the DNS resolver layer.

No user action is required.

---
id: dev-feature-003
category: feature
severity: info
title: Introduced query caching layer
published_at: 2026-05-09
pinned: true
---
We have introduced a new caching layer to improve DNS lookup performance.

Expected benefits:
- Reduced response time
- Lower backend load

---
id: dev-feature-004
category: feature
severity: info
title: New API rate limit headers
published_at: 2026-05-07
---
API responses now include clearer rate-limit headers for better observability.

No breaking changes introduced.

---
id: dev-incident-001
category: incident
severity: critical
title: Temporary DNS resolution degradation (resolved)
published_at: 2026-05-12
pinned: true
---
We experienced a brief degradation in DNS resolution affecting some regions.

Status: RESOLVED

Root cause: upstream routing instability

---
id: dev-incident-002
category: incident
severity: warning
title: Partial latency spike in EU region
published_at: 2026-05-11
---
We observed elevated latency in the EU region for a short period.

The issue self-resolved without intervention.

---
id: dev-maint-001
category: maintenance
severity: warning
title: Scheduled DNS node upgrades
published_at: 2026-05-10
pinned: true
---
We will upgrade several DNS nodes to improve global latency.

- No downtime expected
- Short-lived latency fluctuations possible

---
id: dev-maint-002
category: maintenance
severity: info
title: Database optimization rollout
published_at: 2026-05-08
---
Background database optimizations have been deployed successfully.

Users may notice slightly faster query resolution times.

---
id: dev-maintenance-002
category: maintenance
severity: warning
title: Scheduled DNS infrastructure maintenance
published_at: 2026-06-03
pinned: true
link: https://status.moddns.net
---
We will be performing scheduled maintenance on core DNS infrastructure.

During this window:
- Brief latency spikes may occur
- DNS resolution remains available throughout
- No customer action is required

---
id: dev-feature-002
category: feature
severity: info
title: New analytics dashboard now available
published_at: 2026-06-03
---
We are introducing a redesigned analytics dashboard for modDNS.

Improvements include:
- Faster load times
- Improved query visibility
- Cleaner zone overview layout

---
id: dev-news-001
category: news
severity: info
title: Platform stability improvements deployed
published_at: 2026-05-15
---
We have deployed several backend optimizations to improve overall stability and response times.

These updates are fully transparent and require no user action.

---
id: dev-future-001
category: feature
severity: info
title: Upcoming API enhancements (preview)
published_at: 2099-01-01
---
This announcement is scheduled for the future and should remain hidden until its publish date.

It describes upcoming improvements to the modDNS API, including expanded filtering options and improved rate-limit 
handling.

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
published_at: 2026-06-03
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
