# Proxy Filtering Behaviour — Cross-Phase Decision Table

This document defines the expected behaviour of the two-phase DNS filtering
pipeline (domain phase + IP phase). It is the **single source of truth** for
how filtering decisions interact across phases.

> **Rule**: Every PR that changes proxy filtering logic (`proxy/filter/`,
> `proxy/server/server.go:postResolve`, or aggregation in `proxy/filter/aggregate.go`)
> **must** update this document and the corresponding unit tests in
> `proxy/filter/ip_phase_independence_test.go`.

---

## Architecture Overview

```
Client query
    |
    v
HandleBefore        — extract profile, settings, upstream
    |
    v
RequestHandler
    |
    v
DomainFilter.Execute   ← domain phase (pre-resolve)
    |                      sub-filters: filterBlocklists (T100),
    |                                   filterCustomRules (T200),
    |                                   applyDefaultRule  (T0)
    |
    |--- Processed ---------> Resolve upstream
    |                              |
    |                              |--- cache miss --> ResponseHandler --> postResolve
    |                              |--- cache hit  --> postResolve (directly)
    |
    |--- Blocked -----------> postResolve (skip IP filter, respond 0.0.0.0)
                                   |
                                   v
                           IPFilter.Execute   ← IP phase (post-resolve)
                               sub-filters: filterServices    (T100),
                                            filterCustomRules (T200)
                                   |
                                   v
                           respond() — apply final FilterResult
```

### Key Design Decisions

1. **Phase independence**: The IP phase aggregates **only its own** sub-filter
   results. Domain-phase results in `PartialFilteringResults` are preserved for
   observability but do not influence the IP-phase decision.

2. **Domain block guard**: `postResolve` skips `IPFilter.Execute` entirely when
   the domain phase already blocked (`FilterResult.Status == StatusBlocked`).
   This prevents the IP phase (which returns `Processed` for nil responses)
   from overwriting the domain block.

3. **Aggregation rule** (`getFinalFilteringResult`): Any `Allow` present wins
   over any `Block`, regardless of tier. Tiers only determine which reasons are
   reported (highest tier wins within the same decision type).

---

## Sub-filters Reference

| Phase  | Sub-filter          | Possible Decisions   | Tier | Trigger                                        |
|--------|---------------------|----------------------|------|------------------------------------------------|
| Domain | `filterBlocklists`  | None, Block          | 100  | Domain found on a subscribed blocklist         |
| Domain | `filterCustomRules` | None, Allow, Block   | 200  | Domain matches a user-defined custom rule      |
| Domain | `applyDefaultRule`  | None                 | 0    | Always passthrough                             |
| IP     | `filterServices`    | None, Block          | 100  | Resolved IP's ASN matches a blocked service    |
| IP     | `filterCustomRules` | None, Allow, Block   | 200  | Resolved IP or ASN matches a user custom rule  |

**Tier precedence within a phase**: Custom rules (T200) > Blocklists/Services (T100) > Default (T0).
Within same tier: Allow always beats Block.

---

## Section A: Domain Processed — IP Phase Decides

These cases cover the normal flow where the domain phase allows the query
(status = Processed), the upstream resolves, and the IP phase makes the final
decision.

| #  | Domain BL (T100) | Domain CR (T200) | Domain Result | IP SVC (T100) | IP CR (T200) | Final     | Rationale                                                              |
|----|------------------|-------------------|---------------|---------------|--------------|-----------|------------------------------------------------------------------------|
| 1  | —                | —                 | Processed     | —             | —            | Processed | No rules matched anywhere — passthrough                                |
| 2  | —                | —                 | Processed     | Block         | —            | Blocked   | Services block, no domain allow to interfere                           |
| 3  | —                | —                 | Processed     | —             | Block        | Blocked   | IP custom block, no domain allow to interfere                          |
| 4  | —                | —                 | Processed     | Block         | Block        | Blocked   | Both IP sub-filters block                                              |
| 5  | —                | —                 | Processed     | —             | Allow        | Processed | IP custom allow, nothing to conflict with                              |
| 6  | —                | —                 | Processed     | Block         | Allow        | Processed | IP custom allow (T200) overrides services block (T100) within IP phase |
| 7  | —                | Allow             | Processed     | —             | —            | Processed | Domain allow, IP has no opinion                                        |
| 8  | —                | Allow             | Processed     | Block         | —            | Blocked   | Domain allow does NOT leak into IP phase — services block takes effect |
| 9  | —                | Allow             | Processed     | —             | Block        | Blocked   | Domain allow does NOT leak — IP custom block takes effect              |
| 10 | —                | Allow             | Processed     | Block         | Block        | Blocked   | Domain allow does NOT leak — both IP blocks take effect                |
| 11 | —                | Allow             | Processed     | —             | Allow        | Processed | Both phases allow — no conflict                                        |
| 12 | —                | Allow             | Processed     | Block         | Allow        | Processed | IP custom allow (T200) overrides services (T100) within IP phase       |
| 13 | Block            | Allow             | Processed     | —             | —            | Processed | Domain custom (T200) overrides blocklist (T100); IP no opinion         |
| 14 | Block            | Allow             | Processed     | Block         | —            | Blocked   | Domain allow does NOT leak — services block takes effect               |
| 15 | Block            | Allow             | Processed     | —             | Block        | Blocked   | Domain allow does NOT leak — IP custom block takes effect              |
| 16 | Block            | Allow             | Processed     | Block         | Block        | Blocked   | Domain allow does NOT leak — both IP blocks take effect                |
| 17 | Block            | Allow             | Processed     | —             | Allow        | Processed | IP custom allow, domain results irrelevant                             |
| 18 | Block            | Allow             | Processed     | Block         | Allow        | Processed | IP custom allow (T200) overrides services (T100) within IP phase       |

---

## Section B: Domain Blocked — IP Phase Skipped

When the domain phase blocks, no upstream resolution occurs (`dctx.Res` is nil).
The `postResolve` method guards against running the IP filter in this state.

| #  | Domain BL (T100) | Domain CR (T200) | Domain Result | IP Phase     | Final   | Rationale                                                       |
|----|------------------|-------------------|---------------|--------------|---------|-----------------------------------------------------------------|
| 19 | Block            | —                 | Blocked       | Skipped      | Blocked | postResolve guard: domain block preserved, 0.0.0.0 returned    |
| 20 | —                | Block             | Blocked       | Skipped      | Blocked | postResolve guard: domain block preserved                       |
| 21 | Block            | Block             | Blocked       | Skipped      | Blocked | postResolve guard: domain block preserved                       |

**Without the guard**: Both IP sub-filters return `DecisionNone` for nil `Res`
(`filterServices:34`, `filterCustomRules:168`). `getFinalFilteringResult([None, None])`
returns `Processed`, which would overwrite the domain block.

---

## Section C: Cache Hit vs Cache Miss

The code path differs but the behaviour is identical:

| #  | Path       | postResolve triggered via                                    | Behaviour           |
|----|------------|--------------------------------------------------------------|---------------------|
| 22 | Cache miss | `ResponseHandler` (called by vendor after upstream reply)    | Same as Section A   |
| 23 | Cache hit  | `RequestHandler` (detects `dctx.CachedUpstreamAddr != ""`)  | Same as Section A   |

---

## Test Coverage

All scenarios are covered by unit tests in
`proxy/filter/ip_phase_independence_test.go`:

| Test Function                                     | Table Rows  | What It Verifies                                            |
|---------------------------------------------------|-------------|-------------------------------------------------------------|
| `TestIPFilter_PhaseIndependence`                  | #1 — #18    | IP phase aggregates only its own results                    |
| `TestIPFilter_NilResponse_ReturnsProcessed`       | #19 — #21   | IP phase returns Processed for nil Res (guard is essential) |
| `TestIPFilter_PhaseIndependence_PartialResultsGrow` | —         | Domain + IP results accumulate in PartialFilteringResults   |
| `TestIPFilter_NilResponse_SubFiltersReturnNone`   | —           | Both sub-filters return None for nil Res individually       |
| `TestIPFilter_DnsCtxWithAddr`                     | —           | Execute works with Addr set (real-request simulation)       |
