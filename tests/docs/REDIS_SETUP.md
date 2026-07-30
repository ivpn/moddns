# Redis Sentinel Test Topology (Multi-User ACL)

Current backend E2E test topology provides a minimal high-availability Redis deployment with explicit ACL users for clearer separation of application vs. replication/failover concerns.

## Services
- `cache`: Primary Redis (master) on port 6379 (`tests/redis/master.conf`)
- `redis-replica`: Single replica following master (`tests/redis/replica.conf`)
- `sentinel1`, `sentinel2`, `sentinel3`: Three Sentinel instances (quorum = 2) sharing the same base config (`tests/redis/sentinel.conf`)

## ACL & Credential Model
We intentionally avoid a global `requirepass` and instead enable only explicit ACL users:

| User | Purpose | Password | Defined In |
|------|---------|----------|------------|
| `appcache` | Direct client connections (API, Proxy, Blocklists services) | `appCacheTestPass123` | master / replica / sentinel |
| `failovercache` | Replica replication auth & Sentinel->master auth; also usable by clients after a failover | `failoverCacheTestPass456` | master / replica / sentinel |

Default user remains disabled (no accidental broad access). Both users currently have `~* +@all` for simplicity; you may scope commands later (see Hardening section).

## Environment Variables (service `.env` files under `tests/config`)
```
CACHE_MASTER_NAME=mymaster
CACHE_ADDRESSES=sentinel1:26379,sentinel2:26379,sentinel3:26379
CACHE_ADDRESS=cache:6379               # Direct single-node address (still used for non-failover paths)
CACHE_USERNAME=appcache
CACHE_PASSWORD=appCacheTestPass123
CACHE_FAILOVER_USERNAME=failovercache
CACHE_FAILOVER_PASSWORD=failoverCacheTestPass456
```
Failover mode is activated when both `CACHE_MASTER_NAME` and `CACHE_ADDRESSES` are set; otherwise clients may fall back to simple `CACHE_ADDRESS` usage.

## Master Configuration (`tests/redis/master.conf`)
Key lines:
```
user appcache on >appCacheTestPass123 ~* +@all
user failovercache on >failoverCacheTestPass456 ~* +@all
save ""
appendonly no
```
Persistence is disabled for test speed (AOF off, no RDB schedules). Enable if persistence behavior needs test coverage.

## Replica Configuration (`tests/redis/replica.conf`)
```
replicaof 10.5.0.10 6379
masteruser failovercache
masterauth failoverCacheTestPass456
user appcache on >appCacheTestPass123 ~* +@all
user failovercache on >failoverCacheTestPass456 ~* +@all
save ""
appendonly no
```
Replica authenticates to master using the dedicated `failovercache` ACL user (avoids needing a global password / default user).

## Sentinel Configuration (`tests/redis/sentinel.conf`)
```
user appcache on >appCacheTestPass123 ~* +@all
user failovercache on >failoverCacheTestPass456 ~* +@all

sentinel monitor mymaster 10.5.0.10 6379 2
sentinel auth-user mymaster failovercache
sentinel auth-pass mymaster failoverCacheTestPass456
sentinel down-after-milliseconds mymaster 5000
sentinel failover-timeout mymaster 60000
sentinel parallel-syncs mymaster 1
```
Quorum (2) means a majority of Sentinels must agree master is down before failover.

## Using in Tests
Most services auto-detect failover when the failover env vars are populated. Ensure containers for `sentinel1..3`, `cache`, and `redis-replica` are started (see `tests/docker-compose.yml`).

Typical sequence (conceptual):
1. Start stack (compose up).
2. App services connect using `appcache` creds and sentinel address list.
3. Sentinel triggers promotion if `cache` is stopped; replica becomes new master.
4. Clients reconnect transparently (go-redis uses sentinel discovery + failovercache where configured).

## Fallback / Single-Node Mode
Unset `CACHE_MASTER_NAME` or `CACHE_ADDRESSES` to operate purely against `CACHE_ADDRESS` (no Sentinel). Useful for faster, non-HA test runs.

## Hardening Ideas (Future)
1. Narrow `failovercache` permissions: grant only replication-related commands (`+psync +replconf +ping +auth +info +multi +exec +role`).
2. Add a dedicated sentinel user separate from replication if you want audit separation.
3. Rotate passwords during test runs to ensure dynamic credential handling is robust.
4. Add chaos test: kill master container and assert API continues functioning within a timeout.

## Troubleshooting
| Symptom | Likely Cause | Fix |
|---------|--------------|-----|
| Replica log: `AUTH <password> called without any password configured` | Using `masterauth` while default user disabled & missing `masteruser` | Add `masteruser failovercache` with matching ACL user/password |
| Clients see `NOAUTH Authentication required` | Env vars mismatch or connecting before container ready | Verify env values & add health wait | 
| Sentinel not failing over | Quorum not reached / insufficient Sentinels running | Ensure at least 2 Sentinels are healthy |

## Rationale for Multi-User ACL
Separating `appcache` and `failovercache` allows future least-privilege constraints, independent rotation, and clearer audit trails during simulated failover events.

---
Last updated: (auto) reflect current repository state.
