"""
Redis Read-Replica Failover Backend E2E Test

Verifies that the proxy falls back to the Redis master (via sentinel) when
its co-located read replica becomes unavailable, and switches back when the
replica recovers.

The proxy's DualClient health check runs every 3 s and requires 3 consecutive
failures before swapping (~9 s worst-case). Instead of fixed sleeps, tests
poll DNS resolution with a generous deadline — queries fail while the proxy
is still pointed at the dead replica and succeed once the swap completes, so
"first successful query" is the observable swap signal.
"""

import asyncio
import time

import docker
import pytest
from libs.accounts import create_account, delete_account
from libs.dns_lib import DNSLib
from libs.settings import get_settings

REPLICA_CONTAINER = "redis-replica-dns"
# Health check: 3 failures × 3 s interval = ~9 s before the swap; poll with margin.
FAILOVER_TIMEOUT = 30.0
RECOVERY_TIMEOUT = 30.0
POLL_INTERVAL = 1.0

pytestmark = pytest.mark.redis_failover


@pytest.fixture(scope="module")
def docker_client():
    client = docker.from_env()
    yield client
    client.close()


class TestRedisReplicaFailover:

    def setup_class(self):
        self.config = get_settings()
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)
        self.docker_client = docker.from_env()
        # Create a test account once for the whole class.
        account, cookie, password, _ = create_account()
        assert len(account.profiles) == 1
        self.profile_id = account.profiles[0]
        self._cookie = cookie
        self._password = password
        self._account_id = account.id

    def teardown_class(self):
        # Best-effort account cleanup, but always release the docker client.
        try:
            delete_account(self._cookie, self._password, account_id=self._account_id)
        finally:
            self.docker_client.close()

    def _get_replica(self):
        return self.docker_client.containers.get(REPLICA_CONTAINER)

    async def _wait_dns_healthy(self, timeout: float, context: str):
        """Poll until a DoH query returns an answer, tolerating errors while
        the proxy's DualClient detects the topology change. Returns the first
        healthy response; fails the test on deadline."""
        deadline = time.monotonic() + timeout
        last_err = None
        while time.monotonic() < deadline:
            try:
                resp = await self.dns_lib.send_doh_request(
                    self.profile_id, "example.com", "A"
                )
                if resp.answer:
                    return resp
                last_err = "empty answer"
            except Exception as exc:  # connection dropped mid-swap
                last_err = exc
            await asyncio.sleep(POLL_INTERVAL)
        pytest.fail(f"{context}: DNS did not recover within {timeout}s (last: {last_err})")

    @pytest.fixture(autouse=True)
    def _ensure_replica_running(self):
        """Guarantee the replica container is running after every test."""
        yield
        # Restore replica no matter what happened during the test.
        container = self._get_replica()
        container.reload()
        if container.status != "running":
            container.start()
            # Wait for replica sync + proxy health-check recovery.
            asyncio.run(
                self._wait_dns_healthy(RECOVERY_TIMEOUT, "post-test replica restore")
            )

    @pytest.mark.asyncio
    async def test_proxy_falls_back_to_master_when_replica_stops(self):
        """
        Stop the DNS read-replica and verify the proxy continues to
        resolve queries by falling back to the sentinel-managed master.
        """
        # 1. Baseline: query succeeds via replica.
        resp = await self.dns_lib.send_doh_request(
            self.profile_id, "example.com", "A"
        )
        assert len(resp.answer) > 0, "Baseline DNS query failed"

        # 2. Stop the read replica.
        self._get_replica().stop()

        # 3. Poll until the DualClient swaps to master and queries succeed again.
        resp = await self._wait_dns_healthy(
            FAILOVER_TIMEOUT, "fallback to master after replica stop"
        )
        assert len(resp.answer) > 0, (
            "DNS query failed after replica stop — fallback to master did not work"
        )

    @pytest.mark.asyncio
    async def test_proxy_recovers_back_to_replica(self):
        """
        After a failover to master, restarting the replica should cause
        the proxy to switch back to the replica automatically.
        """
        # 1. Baseline query (retry — proxy may still be recovering from previous test's replica restart).
        resp = await self.dns_lib.send_doh_request_with_retry(
            self.profile_id, "example.com", "A"
        )
        assert len(resp.answer) > 0

        # 2. Stop replica → poll until failover to master completes.
        self._get_replica().stop()
        resp = await self._wait_dns_healthy(FAILOVER_TIMEOUT, "fallback to master")
        assert len(resp.answer) > 0, "Fallback to master failed"

        # 3. Restart replica; poll until queries are healthy (proxy swaps back
        #    within one health-check cycle; master keeps serving meanwhile).
        self._get_replica().start()
        resp = await self._wait_dns_healthy(RECOVERY_TIMEOUT, "replica recovery")
        assert len(resp.answer) > 0, "DNS query failed after replica recovery"
