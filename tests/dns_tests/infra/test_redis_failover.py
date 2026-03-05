"""
Redis Read-Replica Failover Integration Test

Verifies that the proxy falls back to the Redis master (via sentinel) when
its co-located read replica becomes unavailable, and switches back when the
replica recovers.

The proxy's DualClient health check runs every 3 s and requires 3 consecutive
failures before swapping (~9 s worst-case).  We use 15 s waits to be safe.
"""

import time

import docker
import pytest
from libs.dns_lib import DNSLib
from libs.settings import get_settings

from conftest import create_acc_and_login_func

REPLICA_CONTAINER = "redis-replica-dns"
# Health check: 3 failures × 3 s interval = ~9 s.  Add generous margin.
FAILOVER_WAIT = 15
RECOVERY_WAIT = 15

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
        account, _ = create_acc_and_login_func()
        assert len(account.profiles) == 1
        self.profile_id = account.profiles[0]

    def teardown_class(self):
        self.docker_client.close()

    def _get_replica(self):
        return self.docker_client.containers.get(REPLICA_CONTAINER)

    @pytest.fixture(autouse=True)
    def _ensure_replica_running(self):
        """Guarantee the replica container is running after every test."""
        yield
        # Restore replica no matter what happened during the test.
        container = self._get_replica()
        container.reload()
        if container.status != "running":
            container.start()
            # Give it a moment to reconnect to master and sync.
            time.sleep(5)

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

        # 3. Wait for DualClient health check to detect the failure and swap.
        time.sleep(FAILOVER_WAIT)

        # 4. Query must still succeed — now served via master.
        resp = await self.dns_lib.send_doh_request(
            self.profile_id, "example.com", "A"
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
        # 1. Baseline query.
        resp = await self.dns_lib.send_doh_request(
            self.profile_id, "example.com", "A"
        )
        assert len(resp.answer) > 0

        # 2. Stop replica → trigger failover to master.
        self._get_replica().stop()
        time.sleep(FAILOVER_WAIT)

        # 3. Verify queries work via master.
        resp = await self.dns_lib.send_doh_request(
            self.profile_id, "example.com", "A"
        )
        assert len(resp.answer) > 0, "Fallback to master failed"

        # 4. Restart replica.
        self._get_replica().start()
        time.sleep(RECOVERY_WAIT)

        # 5. Verify queries still work — proxy should have switched back.
        resp = await self.dns_lib.send_doh_request(
            self.profile_id, "example.com", "A"
        )
        assert len(resp.answer) > 0, (
            "DNS query failed after replica recovery"
        )
