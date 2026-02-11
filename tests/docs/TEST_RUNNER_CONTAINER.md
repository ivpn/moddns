# Test Runner Container

A dedicated container (`test-runner`) executes MongoDB failover integration tests *inside* the Docker bridge network. This avoids hostname/port translation issues that occur when running pytest directly on the host.

## Why
Running tests on the host relied on a `/etc/hosts` hack that mapped:
```
127.0.0.1 mongodb-primary mongodb-secondary1 mongodb-secondary2
```
Because all replica set members advertise port 27017 internally, the host-based approach could only truly reach the published primary (`27017`). After stopping the primary, the driver could not contact the secondaries (which were actually published on 27018/27019), so failover detection timed out despite a successful election.

## How It Works
The `test-runner` service joins the existing `dnsnetwork` so container name resolution and intra-replica connectivity match the application services. The pytest execution then correctly discovers the stepped-up primary.

## Files
- `tests/docker-compose.test-runner.yml` – service definition
- `tests/Dockerfile.test-runner` – slim Python image with dependencies
- `tests/manual/test_mongodb_failover.py` – updated guidance recommending containerized execution

## Usage
```bash
cd tests
# Bring up dependencies first (or rely on implicit depends_on in compose run)
docker compose -f docker-compose.yml up -d mongodb-primary mongodb-secondary1 mongodb-secondary2 mongodb-init dnsapi dnsproxy
# Run failover test
docker compose -f docker-compose.yml -f docker-compose.test-runner.yml run --rm test-runner
```

To run *all* marked failover tests (if more are enabled later):
```bash
docker compose -f docker-compose.yml -f docker-compose.test-runner.yml run --rm test-runner pytest -m mongodb_failover -v -s
```

## Custom Invocations
Interactive shell for ad‑hoc debugging:
```bash
docker compose -f docker-compose.yml -f docker-compose.test-runner.yml run --rm test-runner bash
```

Inside the shell you can run:
```bash
mongosh 'mongodb://mongodb-primary:27017/?replicaSet=rs0' --eval 'rs.status()'
```

## Extending
- Add a Makefile target, e.g. `make test-failover` executing the above compose run.
- Integrate nightly CI pipeline: run only on schedule or when labels (e.g. `needs-failover-test`) are present.

## Troubleshooting
| Symptom | Likely Cause | Fix |
|---------|--------------|-----|
| Timeout waiting for primary | Election still in progress or node not healthy | Check MongoDB logs with `docker logs mongodb-secondary1` |
| Connection refused immediately | Container not started or network mismatch | Ensure `mongodb-init` finished and container is on `dnsnetwork` |
| Replica set has no PRIMARY | Election parameters misconfigured | Inspect `init-replica-set.sh` and member priorities |

## Next Ideas
- Capture rs.status() snapshots pre/post failover into artifact logs
- Add structured JSON logging around election detection to simplify parsing
- Parametrize test to simulate repeated failovers automatically

---
This containerized approach restores accurate visibility into replica set topology and solves the false negative failover detection caused by host-only execution.
