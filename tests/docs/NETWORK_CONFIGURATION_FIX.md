grep mongodb /etc/hosts
# MongoDB Failover Test - Network Connectivity (Revised)

## Background
An earlier workaround advised adding container hostnames to `/etc/hosts` pointing to `127.0.0.1` so that host‑executed pytest runs could resolve the replica set members. That approach appeared to work for simple connectivity but **fails for primary failover detection**.

## What Went Wrong With The /etc/hosts Approach
```
127.0.0.1 mongodb-primary mongodb-secondary1 mongodb-secondary2
```
All three replica set members were advertised internally (inside the Docker network) as:
```
mongodb-primary:27017
mongodb-secondary1:27017
mongodb-secondary2:27017
```
However on the host we exposed *distinct* ports:
```
27017 -> mongodb-primary (container 27017)
27018 -> mongodb-secondary1 (container 27017)
27019 -> mongodb-secondary2 (container 27017)
```
The /etc/hosts mapping cannot express per‑hostname port translations. When the driver (running on the host) tries to reach `mongodb-secondary1:27017` it still connects to `127.0.0.1:27017` (NOT `27018`). Therefore only the original primary was ever truly reachable at the canonical port. After stopping the primary the client sees connection failures and cannot reach a secondary to discover the new primary, even though an election succeeded internally.

## Correct Solution (Recommended)
Run the failover tests *inside the Docker network* so that the test client resolves and connects to the same addresses the services use. A dedicated test runner container has been added:

```
tests/docker-compose.test-runner.yml
tests/Dockerfile.test-runner
```

### Run
```bash
cd tests
docker compose -f docker-compose.yml -f docker-compose.test-runner.yml run --rm test-runner
```

This launches a lightweight Python container on the `dnsnetwork` bridge and executes:
```
pytest -v -s -m mongodb_failover manual/test_mongodb_failover.py
```

### Benefits
- Accurate replica set member reachability (all at :27017)
- No brittle host `/etc/hosts` hacks
- Consistent with how application containers see MongoDB
- Eliminates false timeouts during failover

## Updated Artifacts
| File | Change |
|------|--------|
| `tests/Dockerfile.test-runner` | New image for network‑scoped test execution |
| `tests/docker-compose.test-runner.yml` | Defines `test-runner` service (uses existing `dnsnetwork`) |
| `tests/manual/test_mongodb_failover.py` | Updated docstring/comments removing `/etc/hosts` instructions |
| `tests/Dockerfile` | Fixed cert path logic (dynamic `certifi` resolution) |

## Fallback / Local Debugging
If you still wish to run from the host for quick smoke checks you may connect directly to a *single* node via its published port (e.g. `mongodb://localhost:27017`). Be aware this will NOT validate failover.

## Verification Steps (Containerised)
```bash
docker compose -f docker-compose.yml up -d --build mongodb-primary mongodb-secondary1 mongodb-secondary2 mongodb-init dnsapi dnsproxy
docker compose -f docker-compose.yml -f docker-compose.test-runner.yml run --rm test-runner
```
Expected: Test logs include the original primary stopping, election messages, and successful detection of a new primary.

## Next Improvements
- Add an automated CI target invoking the test runner selectively (manual label / nightly)
- Capture replica set status snapshots before/after failover for richer diagnostics

---
This revision corrects the prior misconception that Docker's host port publishing could magically differentiate replica members behind identical hostname+port pairs on the host. Running inside the network aligns addressing semantics and resolves the failover visibility gap.
