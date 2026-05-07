import os
import pytest
from datetime import datetime
from pathlib import Path
import shutil
import random
import string
import uuid
from datetime import timedelta, timezone
import os as _os
import redis

from retry import retry
from testcontainers.compose import DockerCompose

import hashlib
import base64
import requests as http_requests

import moddns.api_client as client
import moddns.api as api
import moddns.configuration as api_config
from moddns import RequestsLoginBody
from moddns.api.pa_session_api import PASessionApi
from moddns.models.requests_pa_session_req import RequestsPASessionReq
from moddns.models.requests_rotate_pa_session_req import RequestsRotatePASessionReq

from helpers import generate_complex_password
from libs.settings import get_settings

# Shared deterministic blocklist test constants
TEST_BLOCKLIST_ID = "hagezi_threat_intelligence_feeds_full"
TEST_DOMAIN = "example.com"  # parent only inserted, existing domain so it's resolvable
TEST_SUBDOMAIN = (
    f"sub.{TEST_DOMAIN}"  # not inserted; used to validate inherited blocking
)


@pytest.fixture
def ensure_test_blocklisted():
    """Insert a deterministic test domain into the target blocklist for the duration of a test.
    The subdomain is intentionally not added; proxy logic should still block it when subdomain rule applies.
    """
    r = redis.Redis(host="localhost", port=6379, db=0)
    key = f"blocklist:{TEST_BLOCKLIST_ID}"
    r.sadd(key, TEST_DOMAIN)
    try:
        yield
    finally:
        r.srem(key, TEST_DOMAIN)


@pytest.fixture
def ensure_domain_blocklisted():
    """Insert an arbitrary domain into the test blocklist.

    Usage: ``@pytest.mark.parametrize`` the ``blocklist_domain`` parameter,
    then request this fixture.  The domain is removed on teardown.
    """
    _inserted = []
    r = redis.Redis(host="localhost", port=6379, db=0)
    key = f"blocklist:{TEST_BLOCKLIST_ID}"

    def _insert(domain: str):
        r.sadd(key, domain)
        _inserted.append(domain)

    yield _insert

    for d in _inserted:
        r.srem(key, d)


# TODO: class scope can be troublesome, investigate usage and change if necessary
@pytest.fixture(scope="class")
def create_account_and_login():
    """
    Pytest fixture to create a new account, log in, and return the account object with session cookie.
    Cleans up by deleting the account after the test class is completed.
    """
    account, cookie = create_acc_and_login_func()
    yield account, cookie

    # TODO: Cleanup: delete the account after the test
    # this has to be done together with scope change
    # try:
    #     config = get_settings()
    #     api_conf = api_config.Configuration(host=config.DNS_API_ADDR)
    #     with client.ApiClient(api_conf) as api_client:
    #         account_api = api.AccountApi(api_client)
    #         account_api.api_client.default_headers["Cookie"] = cookie
    # TODO: get deletion code before
    #         resp = account_api.api_v1_accounts_current_delete_with_http_info()
    #         assert (
    #             resp.status_code == 204
    #         ), f"Account deletion failed with status code: {resp.status_code}"
    # except Exception as e:
    #     # Log the error but don't fail the test due to cleanup issues
    #     print(f"Warning: Failed to delete test account {account.id}: {str(e)}")


def create_temp_subscription(validity_days: int = 30) -> tuple[str, str]:
    """Provision a pre-auth session (PASession) for the ZLA signup flow.

    Flow:
      1. Generate a random token and compute its SHA256 hash
      2. Create a preauth entry in the mock preauth service
      3. Call POST /api/v1/pasession/add with PSK to cache the PASession
      4. Call PUT /api/v1/pasession/rotate to get a rotated session cookie
      5. Return (subscription_id, pa_session_cookie)
    """
    config = get_settings()

    subscription_id = str(uuid.uuid4())
    session_id = str(uuid.uuid4())
    preauth_id = str(uuid.uuid4())
    token = str(uuid.uuid4())  # random token

    active_until_dt = datetime.utcnow().replace(tzinfo=timezone.utc) + timedelta(
        days=validity_days
    )
    active_until = active_until_dt.isoformat().replace("+00:00", "Z")

    # Compute token hash (SHA256, base64-encoded) matching what the API validates
    token_hash = base64.b64encode(hashlib.sha256(token.encode()).digest()).decode()

    # 1. Create preauth entry in mock preauth service
    mock_preauth_url = _os.getenv("MOCK_PREAUTH_URL", "http://localhost:8080")
    http_requests.post(
        f"{mock_preauth_url}/entry",
        json={
            "id": preauth_id,
            "token_hash": token_hash,
            "is_active": True,
            "active_until": active_until,
            "tier": "Tier 2",
        },
    ).raise_for_status()

    # 2. Add PASession via API (PSK-protected endpoint)
    api_conf = api_config.Configuration(host=config.DNS_API_ADDR)
    psk = ""  # empty PSK works if no PSK is set in API .env

    with client.ApiClient(api_conf) as api_client:
        pa_api = PASessionApi(api_client)
        pa_api.api_client.default_headers["Authorization"] = f"Bearer {psk}"
        body = RequestsPASessionReq(id=session_id, preauth_id=preauth_id, token=token)
        resp = pa_api.api_v1_pasession_add_post(body=body)
        assert (
            resp.get("message") == "pre-auth session added"
        ), f"Unexpected PASession add response: {resp}"

    # 3. Rotate PASession to get cookie
    with client.ApiClient(api_conf) as api_client:
        pa_api = PASessionApi(api_client)
        rotate_body = RequestsRotatePASessionReq(sessionid=session_id)
        rotate_resp = pa_api.api_v1_pasession_rotate_put_with_http_info(
            body=rotate_body
        )
        assert rotate_resp.status_code == 200, (
            f"PASession rotation failed: {rotate_resp.status_code}"
        )
        pa_cookie = rotate_resp.headers.get("Set-Cookie", "")
        assert "pa_session=" in pa_cookie, (
            f"No pa_session cookie in rotation response: {pa_cookie}"
        )

    return subscription_id, pa_cookie


def create_acc_and_login_func():
    """Create a new account, log in, fetch current account and return (account, cookie).
    Flow:
        1. create temp subscription cache key
        2. register account (201 expected)
        3. login to obtain session cookie
        4. GET /accounts/current to retrieve full account object
    """
    config = get_settings()
    api_conf = api_config.Configuration(host=config.DNS_API_ADDR)
    with client.ApiClient(api_conf) as api_client:
        account_api = api.AccountApi(api_client)
        auth_api = api.AuthenticationApi(api_client)

        # Create a new account with a random email
        email = (
            f"test{''.join(random.choice(string.digits) for _ in range(5))}@ivpn.net"
        )
        password = generate_complex_password()

        # Prepare PASession for ZLA signup flow
        subscription_id, pa_cookie = create_temp_subscription()

        # Set pa_session cookie for registration
        account_api.api_client.default_headers["Cookie"] = pa_cookie
        reg_resp = account_api.api_v1_accounts_post_with_http_info(
            body={"email": email, "password": password, "subid": subscription_id}
        )
        assert (
            reg_resp.status_code == 201
        ), f"Registration failed with status code: {reg_resp.status_code}"
        # registration success is 201; full account not returned anymore
        # Log in to the account
        login_response = auth_api.api_v1_login_post_with_http_info(
            body=RequestsLoginBody(email=email, password=password)
        )
        assert (
            login_response.status_code == 200
        ), f"Login failed with status code: {login_response.status_code}"
        cookie = login_response.headers.get("Set-Cookie")
        assert cookie, "No session cookie returned after login"

        # Fetch current account data using cookie
        account_api.api_client.default_headers["Cookie"] = cookie
        account = account_api.api_v1_accounts_current_get()
        assert len(account.profiles) == 1
        return account, cookie


@pytest.fixture(scope="session", autouse=True)
def ensure_blocklists_configured():
    """
    Autouse fixture that runs once per test session to ensure blocklists are configured.
    Fails the test run early if no blocklists are found.
    Uses retry with exponential backoff to handle temporary unavailability.
    """
    acc, cookie = create_acc_and_login_func()
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
        try:
            # Get container name and logs
            container_name = container.Name
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
