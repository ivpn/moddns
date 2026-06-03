# Real blocklist fixtures

Frozen, **truncated** snapshots of real upstream blocklists, one per distinct
extractor / format variant. Used by `TestRealBlocklists` (offline, deterministic)
to verify the full parse pipeline (`NewExtractor → ExtractMetadata → Convert →
scanValidatedDomains`) against real-world content.

Each file = the upstream **header block** (verbatim) + a **random ~10k sample** of
the body (or the whole body if smaller). Regenerate / rotate the sample with:

```
REFRESH_FIXTURES=1 go test ./service -run TestRefreshFixtures -count=1
```

These are small excerpts retained for testing only; each upstream list is under
its own license (see the source repos).

| File | blocklist_id | Extractor | Source |
|---|---|---|---|
| adguard.txt | adguard_dns_filter | AdGuard | https://adguardteam.github.io/AdGuardSDNSFilter/Filters/filter.txt |
| hagezi_domains.txt | hagezi_multi_light | Hagezi | https://github.com/hagezi/dns-blocklists (domains/light.txt) |
| hagezi_wildcard.txt | hagezi_gambling | Hagezi | https://github.com/hagezi/dns-blocklists (wildcard/gambling-onlydomains.txt) |
| oisd.txt | oisd_small | OISD | https://small.oisd.nl/domainswild2 |
| steven_black.txt | steven_black_ads_malware | StevenBlack | https://github.com/StevenBlack/hosts |
| blp.txt | blp_gambling | Domains | https://github.com/blocklistproject/Lists (alt-version/gambling-nl.txt) |
| blp_fakenews.txt | blp_fakenews | Domains | https://github.com/marktron/fakenews (hosts format) |
| ut1.txt | ut1_gaming | Domains | https://github.com/olbat/ut1-blacklists (games/domains) |
| shadowwhisperer.txt | shadowwhisperer_dating | Domains | https://github.com/ShadowWhisperer/BlockLists (RAW/Dating) |
