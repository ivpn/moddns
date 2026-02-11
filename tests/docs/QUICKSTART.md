# MongoDB Failover Tests - Quick Start Guide

## 🚀 Quick Start (5 minutes)

### 1. Install Dependencies
```bash
cd tests/
pip install -r requirements.txt
```

### 2. Start Test Environment
```bash
docker-compose up -d
```

### 3. Wait for Initialization (~60 seconds)
```bash
# Watch the init process
docker logs -f mongodb-init

# Wait for: "MongoDB replica set initialization complete!"
# Then Ctrl+C to exit log view
```

### 4. Validate Setup
```bash
./validate_mongodb_setup.sh
```

Expected output:
```
✓ Docker daemon is running
✓ mongodb-primary is running
✓ mongodb-secondary1 is running
✓ mongodb-secondary2 is running
✓ Replica set is initialized
...
```

### 5. Run Tests

**Run all failover tests** (~5-7 minutes):
```bash
make test_mongodb_failover
```

**Run just one test** (~1 minute):
```bash
pytest -v -s manual/test_mongodb_failover.py::TestMongoDBFailover::test_primary_mongodb_failure
```

## 📊 What to Expect

### Test Output Format
```
==================================================
TEST: Primary MongoDB Node Failure
==================================================

1️⃣ Verifying initial cluster state...
   Initial primary: mongodb-primary

2️⃣ Verifying initial account creation...
   Account ID: 507f1f77bcf86cd799439011
   Profile ID: abc123...

...

✅ PRIMARY FAILOVER TEST PASSED
==================================================
```

### Status Indicators
- ✓ = Step completed successfully
- ⟳ = Operation in progress
- ⚠ = Warning (usually expected)
- ✗ = Error (unexpected)

## 🎯 Test Scenarios

| Test | Duration | What It Tests |
|------|----------|--------------|
| `test_primary_mongodb_failure` | ~60s | Automatic failover when primary fails |
| `test_secondary_mongodb_failure` | ~30s | Operations with degraded cluster |
| `test_complete_mongodb_failure_and_recovery` | ~90s | Complete outage and recovery |
| `test_rapid_mongodb_failover_cycling` | ~180s | Multiple consecutive failovers |

## 🐛 Troubleshooting

### "No primary node found"
```bash
# Restart the initialization
docker restart mongodb-init
docker logs -f mongodb-init
```

### "Container not healthy"
```bash
# Check container health
docker ps
docker inspect mongodb-primary | grep -A 20 Health

# Restart if needed
docker-compose restart mongodb-primary
```

### "pymongo not installed"
```bash
pip install -r requirements.txt
```

### Tests hang or timeout
```bash
# Check system resources
docker stats

# Restart environment
docker-compose down
docker-compose up -d
sleep 60  # Wait for initialization
```

## 🔄 Common Commands

### Restart Everything
```bash
docker-compose down
docker-compose up -d
sleep 60  # Wait for replica set initialization
```

### Clean Restart (Remove Data)
```bash
docker-compose down -v  # ⚠️ Destroys all data!
docker-compose up -d
sleep 60
```

### View Logs
```bash
# MongoDB logs
docker logs mongodb-primary
docker logs mongodb-init

# Service logs
docker logs dnsapi
docker logs dnsproxy
```

### Check Replica Set Status
```bash
docker exec mongodb-primary mongosh --eval "rs.status()"
```

## 📈 Performance Tips

### Run Tests Faster
Skip the slow tests during development:
```bash
# Run only fast tests
pytest -v -m "not mongodb_failover" dns_tests/

# Or run specific quick test
pytest -v dns_tests/test_basic.py
```

### Parallel Testing
These tests cannot run in parallel (they modify cluster state).
Run sequentially only.

## ✅ Success Criteria

All tests pass when you see:
```
==================================================
✅ PRIMARY FAILOVER TEST PASSED
==================================================
...
==================================================
✅ SECONDARY FAILURE TEST PASSED  
==================================================
...
==================================================
✅ COMPLETE FAILURE AND RECOVERY TEST PASSED
==================================================
...
==================================================
✅ RAPID FAILOVER CYCLING TEST PASSED
==================================================

========== 4 passed in 300.00s ==========
```

## 🎉 That's It!

Your MongoDB failover tests are ready. If you see all tests passing, the modDNS services successfully handle MongoDB failures and automatic failover.

## 📚 More Information

- **Detailed Guide**: `tests/docs/mongodb-failover-tests-README.md`
- **Design Document**: `tests/docs/mongodb-failover-test-design.md`
- **Implementation Summary**: `tests/docs/mongodb-failover-implementation-summary.md`

## 💡 Tips

1. First time running? Expect 10-15 minutes for full suite
2. Subsequent runs are faster (~5-7 minutes)
3. Run validation script before each test session
4. Keep Docker Desktop/daemon running
5. Ensure 4GB+ free RAM for smooth operation

## 🆘 Need Help?

Check the logs:
```bash
docker-compose logs | grep -i error
```

Or review the full documentation in `tests/docs/`.
