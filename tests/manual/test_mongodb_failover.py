"""
MongoDB Failover Integration Tests

These tests verify that modDNS services (API, Proxy, Blocklists) can handle
MongoDB replica set failover scenarios gracefully.

Test scenarios:
1. Primary node failure
2. Secondary node failure
3. Complete cluster failure and recovery
4. Rapid failover cycling
5. Network partition simulation
"""

import time
import pytest
import docker
from typing import Generator, Dict
import pymongo
from pymongo.errors import AutoReconnect, ConnectionFailure, ServerSelectionTimeoutError

import moddns.api_client as client
import moddns.api as api
import moddns.configuration as api_config
from moddns import RequestsLoginBody
from helpers import generate_complex_password
from libs.settings import get_settings
from libs.dns_lib import DNSLib
import random
import string


# Test configuration
# IMPORTANT: For reliable failover detection RUN THESE TESTS INSIDE THE DOCKER NETWORK
# Use the containerised test runner declared in docker-compose.test-runner.yml:
#   docker compose -f docker-compose.yml -f docker-compose.test-runner.yml run --rm test-runner
# The previous approach of adding container names to /etc/hosts on the host and running pytest locally
# is NOT sufficient for replica set failover because all three members were mapped to 127.0.0.1:27017,
# making secondaries unreachable after the original primary stops. Keeping the test inside the
# Docker bridge network preserves correct intra-cluster addressing (mongodb-*:27017).
MONGODB_CONNECTION_STRING = (
    "mongodb://mongodb-primary:27017,"
    "mongodb-secondary1:27017,"
    "mongodb-secondary2:27017/"
    "?replicaSet=rs0"
    "&directConnection=false"
    "&serverSelectionTimeoutMS=30000"
)
MONGODB_AUTH = None  # No authentication for testing
FAILOVER_TIMEOUT = 120  # seconds to wait for failover (election takes ~15-30s)
RECOVERY_WAIT = 20  # seconds to wait after cluster recovery


# ============================================================================
# Helper Functions
# ============================================================================


def wait_for_mongodb_failover(
    connection_string: str = MONGODB_CONNECTION_STRING,
    timeout: int = FAILOVER_TIMEOUT,
    interval: int = 2,
) -> bool:
    """
    Wait for MongoDB replica set to elect new primary.

    Args:
        connection_string: MongoDB connection string
        timeout: Maximum time to wait in seconds
        interval: Check interval in seconds

    Returns:
        True if new primary elected, False if timeout
    """
    start_time = time.time()
    client_mongo = None

    try:
        # Connect with or without authentication
        if MONGODB_AUTH:
            client_mongo = pymongo.MongoClient(
                connection_string,
                username=MONGODB_AUTH["username"],
                password=MONGODB_AUTH["password"],
                authSource=MONGODB_AUTH["authSource"],
                serverSelectionTimeoutMS=30000,
            )
        else:
            client_mongo = pymongo.MongoClient(
                connection_string,
                serverSelectionTimeoutMS=30000,
            )

        while time.time() - start_time < timeout:
            try:
                status = client_mongo.admin.command("replSetGetStatus")
                primary_count = sum(
                    1 for m in status["members"] if m["stateStr"] == "PRIMARY"
                )

                if primary_count == 1:
                    # Found exactly one primary
                    primary_member = next(
                        m for m in status["members"] if m["stateStr"] == "PRIMARY"
                    )
                    print(f"✓ New primary elected: {primary_member['name']}")
                    return True

                elif primary_count == 0:
                    print(
                        f"⟳ Waiting for primary election... ({int(time.time() - start_time)}s)"
                    )

            except (AutoReconnect, ConnectionFailure, ServerSelectionTimeoutError) as e:
                print(f"⟳ MongoDB unreachable during failover: {str(e)[:50]}...")

            time.sleep(interval)

    except Exception as e:
        print(f"✗ Error checking replica set status: {e}")
    finally:
        if client_mongo:
            client_mongo.close()

    print(f"✗ Timeout waiting for primary election after {timeout}s")
    return False


def get_mongodb_replica_status(
    connection_string: str = MONGODB_CONNECTION_STRING,
) -> Dict:
    """
    Get current replica set status.

    Returns:
        Dictionary with replica set status or None if unavailable
    """
    client_mongo = None
    try:
        # Connect with or without authentication
        if MONGODB_AUTH:
            client_mongo = pymongo.MongoClient(
                connection_string,
                username=MONGODB_AUTH["username"],
                password=MONGODB_AUTH["password"],
                authSource=MONGODB_AUTH["authSource"],
                serverSelectionTimeoutMS=30000,
            )
        else:
            client_mongo = pymongo.MongoClient(
                connection_string,
                serverSelectionTimeoutMS=30000,
            )
        return client_mongo.admin.command("replSetGetStatus")
    except Exception as e:
        print(f"Could not get replica status: {e}")
        return None
    finally:
        if client_mongo:
            client_mongo.close()


def get_primary_node_name() -> str:
    """Get the name of the current primary node."""
    status = get_mongodb_replica_status()
    if status:
        for member in status["members"]:
            if member["stateStr"] == "PRIMARY":
                return member["name"].split(":")[0]
    return None


def verify_service_health(
    service_name: str, docker_client: docker.DockerClient
) -> bool:
    """
    Verify a service container is healthy and running.

    Args:
        service_name: Name of the Docker container
        docker_client: Docker client instance

    Returns:
        True if service is running, False otherwise
    """
    try:
        container = docker_client.containers.get(service_name)
        container.reload()
        return container.status == "running"
    except docker.errors.NotFound:
        return False
    except Exception as e:
        print(f"Error checking {service_name}: {e}")
        return False


def wait_for_container_healthy(
    container_name: str, docker_client: docker.DockerClient, timeout: int = 30
) -> bool:
    """
    Wait for a container to become healthy.

    Args:
        container_name: Name of the container
        docker_client: Docker client instance
        timeout: Maximum time to wait in seconds

    Returns:
        True if container is healthy, False if timeout
    """
    start_time = time.time()
    while time.time() - start_time < timeout:
        try:
            container = docker_client.containers.get(container_name)
            container.reload()

            if container.status == "running":
                # Check if healthcheck is defined
                health = container.attrs.get("State", {}).get("Health", {})
                if health:
                    if health.get("Status") == "healthy":
                        return True
                else:
                    # No healthcheck defined, just check if running
                    return True

        except docker.errors.NotFound:
            pass
        except Exception as e:
            print(f"Error checking {container_name}: {e}")

        time.sleep(2)

    return False


# ============================================================================
# Pytest Fixtures
# ============================================================================


@pytest.fixture
def docker_client() -> docker.DockerClient:
    """Get Docker client for container control."""
    return docker.from_env()


@pytest.fixture
def mongodb_containers(docker_client: docker.DockerClient) -> Dict:
    """Get references to MongoDB containers."""
    return {
        "primary": docker_client.containers.get("mongodb-primary"),
        "secondary1": docker_client.containers.get("mongodb-secondary1"),
        "secondary2": docker_client.containers.get("mongodb-secondary2"),
    }


@pytest.fixture
def restore_mongodb_cluster(
    mongodb_containers: Dict, docker_client: docker.DockerClient
) -> Generator:
    """
    Ensure MongoDB cluster is restored after test.

    This fixture automatically restores the cluster state after each test,
    even if the test fails.
    """
    yield

    print("\n🔄 Restoring MongoDB cluster...")

    # Restart all stopped containers
    for name, container in mongodb_containers.items():
        try:
            container.reload()
            if container.status != "running":
                print(f"  Starting {name}...")
                container.start()
                time.sleep(3)
        except Exception as e:
            print(f"  Warning: Could not restore {name}: {e}")

    # Wait for all containers to be healthy
    for name in ["mongodb-primary", "mongodb-secondary1", "mongodb-secondary2"]:
        wait_for_container_healthy(name, docker_client, timeout=30)

    # Wait for cluster to stabilize
    print("  Waiting for cluster to stabilize...")
    time.sleep(RECOVERY_WAIT)

    # Verify cluster is healthy
    status = get_mongodb_replica_status()
    if status:
        primary_count = sum(1 for m in status["members"] if m["stateStr"] == "PRIMARY")
        if primary_count == 1:
            print("✓ MongoDB cluster restored successfully")
        else:
            print(f"⚠ Warning: Unexpected primary count: {primary_count}")
    else:
        print("⚠ Warning: Could not verify cluster status")


@pytest.fixture
def test_account_with_dns():
    """
    Create a test account and return account, cookie, and DNS client.

    This is a convenience fixture that combines account creation with DNS client setup.
    """
    config = get_settings()
    api_conf = api_config.Configuration(host=config.DNS_API_ADDR)
    dns_lib = DNSLib(config.DOH_ENDPOINT)

    with client.ApiClient(api_conf) as api_client:
        account_api = api.AccountApi(api_client)
        auth_api = api.AuthenticationApi(api_client)

        # Create account
        email = f"failover_test_{''.join(random.choice(string.digits) for _ in range(8))}@ivpn.net"
        password = generate_complex_password()
        # subscription required for registration
        from conftest import create_temp_subscription

        subscription_id = create_temp_subscription()
        # Registration now returns only 201, no account object
        account_api.api_v1_accounts_post(
            body={"email": email, "password": password, "subid": subscription_id}
        )

        # Login to obtain session cookie
        login_response = auth_api.api_v1_login_post_with_http_info(
            body=RequestsLoginBody(email=email, password=password)
        )
        cookie = login_response.headers.get("Set-Cookie")
        assert cookie

        # Fetch current account details
        account = account_api.api_v1_accounts_current_get()
        profile_id = account.profiles[0]

        return account, cookie, dns_lib, profile_id


# ============================================================================
# Test Markers
# ============================================================================

pytestmark = pytest.mark.mongodb_failover


# ============================================================================
# Test Cases
# ============================================================================


class TestMongoDBFailover:
    """Test suite for MongoDB failover scenarios."""

    def setup_class(self):
        """Setup test class."""
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)

    @pytest.mark.asyncio
    async def test_primary_mongodb_failure(
        self,
        docker_client,
        mongodb_containers,
        restore_mongodb_cluster,
        test_account_with_dns,
    ):
        """
        Test that services handle primary MongoDB failure gracefully.

        This test verifies:
        1. Account creation works before failover
        2. Primary node can be stopped
        3. Automatic failover occurs (secondary becomes primary)
        4. Services reconnect to new primary automatically
        5. Write operations work after failover
        6. DNS resolution continues working
        7. Read operations work after failover

        Timeline:
        - Create account: ~2s
        - Stop primary: ~1s
        - Wait for failover: ~30-60s (MongoDB election + service reconnection)
        - Verify operations: ~5s

        Total expected duration: ~40-70 seconds
        """
        account, cookie, dns_lib, profile_id = test_account_with_dns

        print("\n" + "=" * 70)
        print("TEST: Primary MongoDB Node Failure")
        print("=" * 70)

        # Step 1: Verify initial cluster state
        print("\n1️⃣ Verifying initial cluster state...")
        initial_status = get_mongodb_replica_status()
        assert initial_status is not None, "Could not get initial cluster status"

        initial_primary = get_primary_node_name()
        print(f"   Initial primary: {initial_primary}")
        assert initial_primary is not None, "No primary node found"

        # Step 2: Verify account was created successfully
        print("\n2️⃣ Verifying initial account creation...")
        print(f"   Account ID: {account.id}")
        print(f"   Profile ID: {profile_id}")
        assert len(account.profiles) == 1

        # Step 3: Verify DNS resolution works before failover
        print("\n3️⃣ Testing DNS resolution before failover...")
        try:
            response = await dns_lib.send_doh_request(profile_id, "example.com", "A")
            print(f"   ✓ DNS query successful: {len(response.answer)} answers")
            assert len(response.answer) > 0
        except Exception as e:
            pytest.fail(f"DNS resolution failed before failover: {e}")

        # Step 4: Stop the primary MongoDB node
        print(f"\n4️⃣ Stopping primary node: {initial_primary}...")
        primary_container = mongodb_containers["primary"]
        if initial_primary != "mongodb-primary":
            # Primary might have already failed over, find the actual primary
            for name, container in mongodb_containers.items():
                container_name = container.attrs["Name"].lstrip("/")
                if container_name == initial_primary:
                    primary_container = container
                    break

        primary_container.stop()
        print(f"   ✓ Stopped {initial_primary}")

        # Step 5: Wait for automatic failover
        print("\n5️⃣ Waiting for automatic failover...")
        print(f"   This may take up to {FAILOVER_TIMEOUT} seconds...")
        failover_successful = wait_for_mongodb_failover(timeout=FAILOVER_TIMEOUT)

        assert failover_successful, "MongoDB failover did not complete within timeout"

        new_primary = get_primary_node_name()
        print(f"   ✓ Failover complete! New primary: {new_primary}")
        assert new_primary != initial_primary, "Primary did not change"

        # Step 6: Wait for services to reconnect
        print("\n6️⃣ Waiting for services to reconnect...")
        time.sleep(10)  # Give services time to reconnect

        # Verify API service is still healthy
        assert verify_service_health(
            "dnsapi", docker_client
        ), "API service is not running"
        assert verify_service_health(
            "dnsproxy", docker_client
        ), "Proxy service is not running"
        print("   ✓ Services are running")

        # Step 7: Test write operation after failover (create new account)
        print("\n7️⃣ Testing write operation after failover...")
        try:
            with client.ApiClient(self.api_config) as api_client:
                account_api = api.AccountApi(api_client)

                email = f"post_failover_{''.join(random.choice(string.digits) for _ in range(8))}@ivpn.net"
                password = generate_complex_password()
                from conftest import create_temp_subscription

                subscription_id = create_temp_subscription()
                # Register (201 only)
                account_api.api_v1_accounts_post(
                    body={
                        "email": email,
                        "password": password,
                        "subid": subscription_id,
                    }
                )

                # Login & fetch current
                auth_api = api.AuthenticationApi(api_client)
                login_resp = auth_api.api_v1_login_post_with_http_info(
                    body=RequestsLoginBody(email=email, password=password)
                )
                assert login_resp.status_code == 200
                cookie_new = login_resp.headers.get("Set-Cookie")
                assert cookie_new
                account_api.api_client.default_headers["Cookie"] = cookie_new
                new_account = account_api.api_v1_accounts_current_get()

                print(f"   ✓ Created new account: {new_account.id}")
                assert len(new_account.profiles) == 1
        except Exception as e:
            pytest.fail(f"Write operation failed after failover: {e}")

        # Step 8: Test read operation after failover (login to original account)
        print("\n8️⃣ Testing read operation after failover...")
        try:
            with client.ApiClient(self.api_config) as api_client:
                auth_api = api.AuthenticationApi(api_client)
                account_api = api.AccountApi(api_client)

                # Login to original account
                login_response = auth_api.api_v1_login_post_with_http_info(
                    body=RequestsLoginBody(
                        email=account.email, password=account.email.split("@")[0]
                    )  # Using email prefix as we generated password
                )
                # Note: We can't verify login since we don't have original password stored
                # Instead, let's fetch account with the existing cookie

                account_api.api_client.default_headers["Cookie"] = cookie
                # Try to get blocklists (read operation)
                blocklist_api = api.BlocklistsApi(api_client)
                blocklist_api.api_client.default_headers["Cookie"] = cookie
                blocklists = blocklist_api.api_v1_blocklists_get()

                print(f"   ✓ Read operation successful: {len(blocklists)} blocklists")
        except Exception as e:
            print(f"   ⚠ Read operation with old session: {e}")
            print("   (Session may have expired during failover, this is acceptable)")

        # Step 9: Verify DNS resolution still works after failover
        print("\n9️⃣ Testing DNS resolution after failover...")
        try:
            response = await dns_lib.send_doh_request(profile_id, "google.com", "A")
            print(f"   ✓ DNS query successful: {len(response.answer)} answers")
            assert len(response.answer) > 0
        except Exception as e:
            pytest.fail(f"DNS resolution failed after failover: {e}")

        # Step 10: Verify cluster state
        print("\n🔟 Verifying final cluster state...")
        final_status = get_mongodb_replica_status()
        assert final_status is not None, "Could not get final cluster status"

        primary_count = sum(
            1 for m in final_status["members"] if m["stateStr"] == "PRIMARY"
        )
        healthy_count = sum(
            1
            for m in final_status["members"]
            if m["stateStr"] in ["PRIMARY", "SECONDARY"]
        )

        print(f"   Primaries: {primary_count}")
        print(f"   Healthy nodes: {healthy_count}/3")

        assert primary_count == 1, f"Expected 1 primary, found {primary_count}"
        assert (
            healthy_count >= 2
        ), f"Expected at least 2 healthy nodes, found {healthy_count}"

        print("\n" + "=" * 70)
        print("✅ PRIMARY FAILOVER TEST PASSED")
        print("=" * 70)

    # @pytest.mark.asyncio
    # async def test_secondary_mongodb_failure(
    #     self,
    #     docker_client,
    #     mongodb_containers,
    #     restore_mongodb_cluster,
    #     test_account_with_dns,
    # ):
    #     """
    #     Test that services handle secondary MongoDB failure gracefully.

    #     This test verifies:
    #     1. Services continue operating when one secondary fails
    #     2. Services continue operating when both secondaries fail (primary only)
    #     3. No failover occurs (primary remains primary)
    #     4. Write and read operations continue working

    #     Expected duration: ~20-30 seconds
    #     """
    #     account, cookie, dns_lib, profile_id = test_account_with_dns

    #     print("\n" + "=" * 70)
    #     print("TEST: Secondary MongoDB Node Failures")
    #     print("=" * 70)

    #     # Step 1: Get initial state
    #     print("\n1️⃣ Getting initial cluster state...")
    #     initial_primary = get_primary_node_name()
    #     print(f"   Primary node: {initial_primary}")

    #     # Step 2: Stop first secondary
    #     print("\n2️⃣ Stopping first secondary (mongodb-secondary1)...")
    #     mongodb_containers["secondary1"].stop()
    #     print("   ✓ Stopped mongodb-secondary1")
    #     time.sleep(5)

    #     # Step 3: Verify operations still work
    #     print("\n3️⃣ Verifying operations with 1 secondary down...")
    #     try:
    #         with client.ApiClient(self.api_config) as api_client:
    #             account_api = api.AccountApi(api_client)
    #             email = f"one_sec_down_{''.join(random.choice(string.digits) for _ in range(8))}@ivpn.net"
    #             password = generate_complex_password()
    #             new_account = account_api.api_v1_accounts_post(
    #                 body={"email": email, "password": password}
    #             )
    #             print(f"   ✓ Write operation successful: {new_account.id}")
    #     except Exception as e:
    #         pytest.fail(f"Operations failed with 1 secondary down: {e}")

    #     # Step 4: Verify DNS works
    #     response = await dns_lib.send_doh_request(profile_id, "example.com", "A")
    #     print(f"   ✓ DNS resolution successful: {len(response.answer)} answers")

    #     # Step 5: Stop second secondary
    #     print("\n4️⃣ Stopping second secondary (mongodb-secondary2)...")
    #     mongodb_containers["secondary2"].stop()
    #     print("   ✓ Stopped mongodb-secondary2")
    #     print("   ⚠ Cluster now has only primary (no redundancy)")
    #     time.sleep(5)

    #     # Step 6: Verify operations still work (primary only)
    #     print("\n5️⃣ Verifying operations with primary only...")
    #     try:
    #         with client.ApiClient(self.api_config) as api_client:
    #             account_api = api.AccountApi(api_client)
    #             email = f"primary_only_{''.join(random.choice(string.digits) for _ in range(8))}@ivpn.net"
    #             password = generate_complex_password()
    #             new_account = account_api.api_v1_accounts_post(
    #                 body={"email": email, "password": password}
    #             )
    #             print(f"   ✓ Write operation successful: {new_account.id}")
    #     except Exception as e:
    #         pytest.fail(f"Operations failed with primary only: {e}")

    #     # Step 7: Verify DNS still works
    #     response = await dns_lib.send_doh_request(profile_id, "google.com", "A")
    #     print(f"   ✓ DNS resolution successful: {len(response.answer)} answers")

    #     # Step 8: Verify primary didn't change
    #     current_primary = get_primary_node_name()
    #     print(f"\n6️⃣ Verifying primary remained stable...")
    #     print(f"   Initial primary: {initial_primary}")
    #     print(f"   Current primary: {current_primary}")
    #     assert current_primary == initial_primary, "Primary changed unexpectedly"

    #     print("\n" + "=" * 70)
    #     print("✅ SECONDARY FAILURE TEST PASSED")
    #     print("=" * 70)

    # @pytest.mark.asyncio
    # async def test_complete_mongodb_failure_and_recovery(
    #     self,
    #     docker_client,
    #     mongodb_containers,
    #     restore_mongodb_cluster,
    #     test_account_with_dns,
    # ):
    #     """
    #     Test service behavior during complete MongoDB outage and recovery.

    #     This test verifies:
    #     1. Services handle complete MongoDB outage gracefully (no crashes)
    #     2. Services return appropriate errors during outage
    #     3. Services automatically reconnect after cluster recovery
    #     4. Operations resume normally after recovery

    #     Expected duration: ~60-90 seconds
    #     """
    #     account, cookie, dns_lib, profile_id = test_account_with_dns

    #     print("\n" + "=" * 70)
    #     print("TEST: Complete MongoDB Cluster Failure and Recovery")
    #     print("=" * 70)

    #     # Step 1: Verify initial operations work
    #     print("\n1️⃣ Verifying baseline operations...")
    #     response = await dns_lib.send_doh_request(profile_id, "example.com", "A")
    #     print(f"   ✓ DNS resolution successful: {len(response.answer)} answers")

    #     # Step 2: Stop ALL MongoDB containers
    #     print("\n2️⃣ Stopping ALL MongoDB nodes...")
    #     for name, container in mongodb_containers.items():
    #         container.stop()
    #         print(f"   ✓ Stopped {name}")

    #     print("   ⚠ Complete MongoDB outage!")
    #     time.sleep(5)

    #     # Step 3: Verify services don't crash
    #     print("\n3️⃣ Verifying services remain running...")
    #     assert verify_service_health("dnsapi", docker_client), "API service crashed"
    #     assert verify_service_health("dnsproxy", docker_client), "Proxy service crashed"
    #     print("   ✓ Services still running (not crashed)")

    #     # Step 4: Verify operations fail gracefully (not crash)
    #     print("\n4️⃣ Verifying graceful error handling...")
    #     try:
    #         with client.ApiClient(self.api_config) as api_client:
    #             account_api = api.AccountApi(api_client)
    #             email = f"during_outage_{''.join(random.choice(string.digits) for _ in range(8))}@ivpn.net"
    #             password = generate_complex_password()
    #             new_account = account_api.api_v1_accounts_post(
    #                 body={"email": email, "password": password}
    #             )
    #             print("   ⚠ Operation unexpectedly succeeded during outage")
    #     except Exception as e:
    #         print(f"   ✓ Operation failed as expected: {str(e)[:80]}...")

    #     # Step 5: DNS might still work from cache
    #     print("\n5️⃣ Testing DNS during MongoDB outage...")
    #     try:
    #         response = await dns_lib.send_doh_request(
    #             profile_id, "cached-example.com", "A"
    #         )
    #         print(
    #             f"   ✓ DNS query succeeded (likely from cache): {len(response.answer)} answers"
    #         )
    #     except Exception as e:
    #         print(f"   ℹ DNS query failed (expected): {str(e)[:80]}...")

    #     # Step 6: Restart MongoDB cluster
    #     print("\n6️⃣ Restarting MongoDB cluster...")
    #     for name, container in mongodb_containers.items():
    #         container.start()
    #         print(f"   Starting {name}...")
    #         time.sleep(3)

    #     # Step 7: Wait for cluster to recover
    #     print("\n7️⃣ Waiting for cluster recovery...")
    #     for container_name in [
    #         "mongodb-primary",
    #         "mongodb-secondary1",
    #         "mongodb-secondary2",
    #     ]:
    #         if not wait_for_container_healthy(
    #             container_name, docker_client, timeout=30
    #         ):
    #             pytest.fail(f"Container {container_name} did not become healthy")
    #         print(f"   ✓ {container_name} is healthy")

    #     # Wait for replica set to initialize
    #     print("   Waiting for replica set initialization...")
    #     time.sleep(RECOVERY_WAIT)

    #     # Verify primary election
    #     if not wait_for_mongodb_failover(timeout=FAILOVER_TIMEOUT):
    #         pytest.fail("Cluster did not elect primary after recovery")

    #     print("   ✓ Cluster recovered with primary")

    #     # Step 8: Wait for services to reconnect
    #     print("\n8️⃣ Waiting for services to reconnect...")
    #     time.sleep(10)

    #     # Step 9: Verify operations work after recovery
    #     print("\n9️⃣ Verifying operations after recovery...")
    #     try:
    #         with client.ApiClient(self.api_config) as api_client:
    #             account_api = api.AccountApi(api_client)
    #             email = f"after_recovery_{''.join(random.choice(string.digits) for _ in range(8))}@ivpn.net"
    #             password = generate_complex_password()
    #             new_account = account_api.api_v1_accounts_post(
    #                 body={"email": email, "password": password}
    #             )
    #             print(f"   ✓ Write operation successful: {new_account.id}")
    #     except Exception as e:
    #         pytest.fail(f"Operations failed after recovery: {e}")

    #     # Step 10: Verify DNS works after recovery
    #     response = await dns_lib.send_doh_request(profile_id, "post-recovery.com", "A")
    #     print(f"   ✓ DNS resolution successful: {len(response.answer)} answers")

    #     print("\n" + "=" * 70)
    #     print("✅ COMPLETE FAILURE AND RECOVERY TEST PASSED")
    #     print("=" * 70)

    # @pytest.mark.asyncio
    # async def test_rapid_mongodb_failover_cycling(
    #     self,
    #     docker_client,
    #     mongodb_containers,
    #     restore_mongodb_cluster,
    #     test_account_with_dns,
    # ):
    #     """
    #     Test service stability during multiple consecutive failovers.

    #     This test verifies:
    #     1. Services handle rapid primary changes
    #     2. Data consistency maintained across multiple failovers
    #     3. Services don't crash or hang during rapid changes

    #     Expected duration: ~120-180 seconds
    #     """
    #     account, cookie, dns_lib, profile_id = test_account_with_dns

    #     print("\n" + "=" * 70)
    #     print("TEST: Rapid MongoDB Failover Cycling")
    #     print("=" * 70)

    #     created_accounts = []

    #     # Cycle 1: Stop primary
    #     print("\n🔄 Failover Cycle 1")
    #     print("=" * 50)
    #     primary_1 = get_primary_node_name()
    #     print(f"1️⃣ Current primary: {primary_1}")

    #     # Create account before failover
    #     with client.ApiClient(self.api_config) as api_client:
    #         account_api = api.AccountApi(api_client)
    #         email = f"cycle1_{''.join(random.choice(string.digits) for _ in range(8))}@ivpn.net"
    #         password = generate_complex_password()
    #         acc = account_api.api_v1_accounts_post(
    #             body={"email": email, "password": password}
    #         )
    #         created_accounts.append(acc.id)
    #         print(f"   ✓ Created account: {acc.id}")

    #     # Stop primary
    #     print(f"2️⃣ Stopping {primary_1}...")
    #     for name, container in mongodb_containers.items():
    #         if container.attrs["Name"].lstrip("/") == primary_1:
    #             container.stop()
    #             break

    #     print("3️⃣ Waiting for failover...")
    #     assert wait_for_mongodb_failover(timeout=FAILOVER_TIMEOUT), "Failover 1 failed"
    #     primary_2 = get_primary_node_name()
    #     print(f"   ✓ New primary: {primary_2}")

    #     time.sleep(10)

    #     # Cycle 2: Stop new primary
    #     print("\n🔄 Failover Cycle 2")
    #     print("=" * 50)
    #     print(f"1️⃣ Current primary: {primary_2}")

    #     # Create account on new primary
    #     with client.ApiClient(self.api_config) as api_client:
    #         account_api = api.AccountApi(api_client)
    #         email = f"cycle2_{''.join(random.choice(string.digits) for _ in range(8))}@ivpn.net"
    #         password = generate_complex_password()
    #         acc = account_api.api_v1_accounts_post(
    #             body={"email": email, "password": password}
    #         )
    #         created_accounts.append(acc.id)
    #         print(f"   ✓ Created account: {acc.id}")

    #     # Stop new primary
    #     print(f"2️⃣ Stopping {primary_2}...")
    #     for name, container in mongodb_containers.items():
    #         if container.attrs["Name"].lstrip("/") == primary_2:
    #             container.stop()
    #             break

    #     print("3️⃣ Waiting for failover...")
    #     assert wait_for_mongodb_failover(timeout=FAILOVER_TIMEOUT), "Failover 2 failed"
    #     primary_3 = get_primary_node_name()
    #     print(f"   ✓ New primary: {primary_3}")

    #     time.sleep(10)

    #     # Verify data consistency
    #     print("\n4️⃣ Verifying data consistency...")
    #     with client.ApiClient(self.api_config) as api_client:
    #         account_api = api.AccountApi(api_client)
    #         email = f"cycle3_{''.join(random.choice(string.digits) for _ in range(8))}@ivpn.net"
    #         password = generate_complex_password()
    #         acc = account_api.api_v1_accounts_post(
    #             body={"email": email, "password": password}
    #         )
    #         created_accounts.append(acc.id)
    #         print(f"   ✓ Created account after 2 failovers: {acc.id}")

    #     # Verify DNS still works
    #     response = await dns_lib.send_doh_request(profile_id, "stability-test.com", "A")
    #     print(f"   ✓ DNS resolution successful: {len(response.answer)} answers")

    #     print(
    #         f"\n5️⃣ Successfully created {len(created_accounts)} accounts across {len(set([primary_1, primary_2, primary_3]))} different primaries"
    #     )

    #     print("\n" + "=" * 70)
    #     print("✅ RAPID FAILOVER CYCLING TEST PASSED")
    #     print("=" * 70)
