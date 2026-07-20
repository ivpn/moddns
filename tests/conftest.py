import asyncio
import os
import pytest
from datetime import datetime
from pathlib import Path
import shutil
from typing import Iterator
import redis

from dns.rdatatype import A

from retry import retry
from testcontainers.compose import DockerCompose

import moddns.api_client as client
import moddns.api as api
import moddns.configuration as api_config

# Re-exported so existing `from conftest import …` sites keep working.
from libs.accounts import (  # noqa: F401
    create_account,
    create_temp_subscription,
    delete_account,
)
from libs.constants import BLOCKLISTED_DOMAIN, TEST_BLOCKLIST_ID
from libs.dns_lib import DNSLib, is_resolved
from libs.session import ProfileSession
from libs.settings import get_settings


@pytest.fixture
def ensure_test_blocklisted():
    """Insert a deterministic test domain into the target blocklist for the duration of a test.
    The subdomain is intentionally not added; proxy logic should still block it when subdomain rule applies.
    """
    cfg = get_settings()
    r = redis.Redis(host=cfg.REDIS_HOST, port=cfg.REDIS_PORT, db=0)
    key = f"blocklist:{TEST_BLOCKLIST_ID}"
    r.sadd(key, BLOCKLISTED_DOMAIN)
    try:
        yield
    finally:
        r.srem(key, BLOCKLISTED_DOMAIN)


@pytest.fixture
def ensure_domain_blocklisted():
    """Insert an arbitrary domain into the test blocklist.

    Usage: ``@pytest.mark.parametrize`` the ``blocklist_domain`` parameter,
    then request this fixture.  The domain is removed on teardown.
    """
    _inserted = []
    cfg = get_settings()
    r = redis.Redis(host=cfg.REDIS_HOST, port=cfg.REDIS_PORT, db=0)
    key = f"blocklist:{TEST_BLOCKLIST_ID}"

    def _insert(domain: str):
        r.sadd(key, domain)
        _inserted.append(domain)

    yield _insert

    for d in _inserted:
        r.srem(key, d)


@pytest.fixture(scope="class")
def user() -> Iterator[ProfileSession]:
    """Class-scoped logged-in test user (ProfileSession facade).

    Tests needing isolation create per-test profiles via ``user.new_profile()``
    — the account itself is shared across the class for speed and deleted on
    teardown.
    """
    session = ProfileSession.create()
    yield session
    session.cleanup()


# Deprecated: migrate to the `user` fixture. Kept while old-style tests remain.
@pytest.fixture(scope="class")
def create_account_and_login():
    """
    Pytest fixture to create a new account, log in, and return the account object with session cookie.
    Cleans up by deleting the account (and all its profiles) after the test class is completed.
    """
    account, cookie, password, _ = create_account()
    yield account, cookie
    delete_account(cookie, password, account_id=account.id)


@pytest.fixture(scope="session")
def redis_client():
    """Session-scoped Redis client for fixtures/tests that seed blocklist sets."""
    cfg = get_settings()
    return redis.Redis(host=cfg.REDIS_HOST, port=cfg.REDIS_PORT, db=0)


def create_acc_and_login_func():
    """Deprecated wrapper over libs.accounts.create_account.

    Returns (account, cookie, password). New code should use the `user`
    fixture (ProfileSession) or libs.accounts.create_account directly.
    """
    account, cookie, password, _ = create_account()
    return account, cookie, password


@pytest.fixture(scope="session", autouse=True)
def ensure_blocklists_configured(start_compose):
    """
    Autouse fixture that runs once per test session to ensure blocklists are
    configured and the DNS stack is ready to serve queries.
    Fails the test run early if no blocklists are found.
    Uses retry with exponential backoff to handle temporary unavailability.

    Depends on ``start_compose`` explicitly so the containers are guaranteed
    to be up before the first API call, regardless of autouse ordering.
    """
    acc, cookie, password = create_acc_and_login_func()
    config = get_settings()
    api_conf = api_config.Configuration(host=config.DNS_API_ADDR)

    @retry(tries=5, delay=2, backoff=2, exceptions=(AssertionError, Exception))
    def check_blocklists():
        with client.ApiClient(api_conf) as api_client:
            bi = api.BlocklistsApi(api_client)
            bi.api_client.default_headers["Cookie"] = cookie
            resp = bi.api_v1_blocklists_get_with_http_info()
            assert (
                resp.status_code == 200
            ), f"Failed to get blocklists info with status code: {resp.status_code}"
            assert (
                len(resp.data) > 0
            ), "No blocklists found in the system. Please configure at least one blocklist before running tests."
            # Check if the TIF blocklist is present
            found = False
            for blocklist in resp.data:
                if blocklist.blocklist_id == "hagezi_threat_intelligence_feeds_full":
                    found = True
                    break

            assert (
                found
            ), "Threat Intelligence Feeds blocklist is not enabled. Please enable it before running tests."

    check_blocklists()

    # DNS-stack readiness gate. The proxy image is FROM scratch (no shell), so
    # it cannot declare a compose healthcheck and testcontainers' wait=True
    # only gates the API. One successfully resolved query through the full
    # chain (proxy TLS → replica Redis profile lookup → recursor, using the
    # testhosts-pinned test.com) proves the stack is ready before any test runs.
    dns_lib = DNSLib(config.DOH_ENDPOINT)
    resp = asyncio.run(
        dns_lib.wait_until(
            acc.profiles[0], "test.com", A, is_resolved, timeout=60.0, interval=1.0
        )
    )
    assert is_resolved(resp), (
        "DNS stack not ready: proxy did not resolve pinned domain test.com within 60s"
    )

    yield

    delete_account(cookie, password, account_id=acc.id)


@pytest.fixture(scope="session")  # autouse=True
def start_compose():
    with DockerCompose("./", build=True, wait=True) as compose:
        yield compose


@pytest.fixture(scope="session", autouse=True)
def docker_logs(start_compose, request):
    """Fixture to save Docker container logs after test suite execution."""
    yield

    # Define logs directory (can be configured through pytest.ini or environment variable)
    logs_dir = os.getenv("DOCKER_LOGS_DIR", "docker_logs")

    # Get compose instance from the existing fixture
    compose = request.getfixturevalue("start_compose")

    # Save logs for all containers
    save_container_logs(compose, logs_dir)


def save_container_logs(compose: DockerCompose, output_dir: str) -> None:
    """Save logs from all containers in the docker-compose setup."""
    containers = compose.get_containers()

    # remove directory if it exists
    if os.path.exists(output_dir):
        shutil.rmtree(output_dir)

    # Create logs directory
    logs_dir = Path(output_dir)
    logs_dir.mkdir(parents=True, exist_ok=True)

    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    for container in containers:
        container_name = container.Name
        try:
            stdout, stderr = compose.get_logs(container_name)

            # Create log file with timestamp
            log_file_stdout = logs_dir / f"{container_name}_{timestamp}.stdout.log"
            log_file_stderr = logs_dir / f"{container_name}_{timestamp}.stderr.log"

            # Write logs to file
            with open(log_file_stdout, "wb") as f:
                f.write(stdout.encode())

            with open(log_file_stderr, "wb") as f:
                f.write(stderr.encode())

        except Exception as e:
            print(f"Failed to save logs for container {container_name}: {str(e)}")
