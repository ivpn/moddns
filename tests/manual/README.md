# Manual Tests

This directory contains integration tests that should be run manually rather than as part of the automated CI pipeline.

## Why Manual Tests?

Tests in this directory are excluded from automated runs because they:

1. **Take significant time** (2-5+ minutes per test)
2. **Require special configuration** (e.g., host network setup)
3. **Simulate destructive scenarios** (e.g., database failures)
4. **Test edge cases** that don't need to run on every commit

## Tests in This Directory

### `test_mongodb_failover.py`

MongoDB replica set failover integration tests.

**Prerequisites**:
1. Docker Compose running with MongoDB replica set
2. Host network configuration (run `../setup_mongodb_hosts.sh`)
3. Python venv with dependencies installed

**Run**:
```bash
# From tests/ directory
source venv/bin/activate
pytest -v -s -m mongodb_failover manual/test_mongodb_failover.py

# Or using Make:
make test_mongodb_failover
```

**What it tests**:
- Primary MongoDB node failure and automatic failover
- Secondary node failures
- Complete cluster failure and recovery
- Rapid failover cycling under load

**Duration**: ~40-180 seconds per test scenario

See `../docs/mongodb-failover-tests-README.md` for detailed documentation.

## Running Manual Tests

### Option 1: Run from Host (Recommended)
```bash
cd tests/
./setup_mongodb_hosts.sh  # One-time setup
source venv/bin/activate
pytest -v -s manual/
```

### Option 2: Run Specific Test
```bash
pytest -v -s manual/test_mongodb_failover.py::TestMongoDBFailover::test_primary_mongodb_failure
```

### Option 3: Run with Makefile
```bash
make test_mongodb_failover
```

## CI Integration

Manual tests are excluded from CI by:
- Being in a separate directory (`manual/` instead of `dns_tests/`)
- CI pipeline only runs `pytest dns_tests/`

To include manual tests in CI (for nightly builds or release testing):
```yaml
# .github/workflows/manual-tests.yml
- name: Run Manual Tests
  run: |
    cd tests/
    pytest -v manual/
```

## Adding New Manual Tests

When creating tests that should run manually:

1. Place test file in `tests/manual/` directory
2. Add comprehensive docstrings explaining:
   - Why it's manual
   - Prerequisites
   - Expected duration
   - What it tests
3. Update this README with test description
4. Add Makefile target if appropriate
5. Document in `tests/docs/` if complex

## Best Practices

- ✅ Add detailed logging/output to show progress
- ✅ Include cleanup logic (use fixtures with `yield`)
- ✅ Verify prerequisites before running test
- ✅ Make tests idempotent (can be run multiple times)
- ✅ Add timeouts to prevent hanging
- ❌ Don't make tests depend on each other
- ❌ Don't modify shared state without cleanup
