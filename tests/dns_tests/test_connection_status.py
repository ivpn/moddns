"""
Backend E2E tests for DNS Connection Status Check feature.

This test suite validates the complete flow of the DNS connection check feature:
1. DNS query to dnscheck authoritative server
2. HTTP API request to retrieve cached data
3. Response validation and status determination
4. Frontend behavior simulation
"""

import asyncio
import random
import time
from typing import Dict, Any

import pytest
import requests
from dns.rdatatype import A
from dns.rdataclass import IN

import moddns.api_client as client
import moddns.api as api
import moddns.configuration as api_config

from libs.settings import get_settings
from libs.dns_lib import DNSLib


@pytest.mark.skip(reason="I did not manage to fully setup the test environment")
class TestDnsConnectionStatus:
    """Backend E2E tests for DNS connection status check feature."""

    def setup_class(self):
        """Setup the test class."""
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)
        self.dnscheck_domain = "test.moddns.net"  # This is a little hack: domain is the same as dnscheck docker container to ensure whole flow is correct

        # Will be populated with real profiles from create_account_and_login fixture
        self.account = None
        self.cookie = None
        self.test_profiles = []

    def setup_real_profiles(self, account, cookie):
        """Setup real profiles from the created account."""
        self.account = account
        self.cookie = cookie

        # Get the default profile that's created with the account
        with client.ApiClient(self.api_config) as api_client:
            profiles_api = api.ProfileApi(api_client)
            profiles_api.api_client.default_headers["Cookie"] = cookie

            # Get the existing profile details
            profile_id = account.profiles[0]
            profile_response = profiles_api.api_v1_profiles_id_get(profile_id)

            self.test_profiles = [
                {
                    "profile_id": profile_response.profile_id,
                    "name": profile_response.name,
                    "id": profile_response.id,
                }
            ]

            print(
                f"Using real profile: {profile_response.name} (ID: {profile_response.profile_id})"
            )

    def generate_random_id(self, length: int = 12) -> str:
        """Generate a random ID similar to nanoid."""
        alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
        return "".join(random.choice(alphabet) for _ in range(length))

    def create_subdomain(self, profile_id: str) -> str:
        """Create a unique subdomain for DNS check."""
        random_id = self.generate_random_id()
        return f"{random_id}-{profile_id}.{self.dnscheck_domain}"

    async def send_dns_query(self, subdomain: str, profile_id: str) -> bool:
        """
        Send DNS over HTTPS query using the configured profile.

        Args:
            subdomain: The subdomain to query
            profile_id: The profile ID to use for the DOH request

        Returns:
            bool: True if query successful, False otherwise
        """
        try:
            # Send DNS over HTTPS query using the DNSLib
            response = await self.dns_lib.send_doh_request(profile_id, subdomain, "A")
            # Verify we get an answer
            assert len(response.answer) > 0, "No DNS response received"

            # Get the IP address from the response
            ip_address = response.answer[0].to_text().split(" ")[-1]
            print(f"DNS over HTTPS query for {subdomain} returned IP: {ip_address}")

            return True

        except Exception as e:
            print(f"DNS over HTTPS query failed: {e}")
            return False

    def send_http_request(
        self, subdomain: str, origin: str = "http://localhost:5174"
    ) -> Dict[str, Any]:
        """
        Send HTTP request to dnscheck API.

        Args:
            subdomain: The subdomain to query
            origin: Origin header for CORS testing

        Returns:
            Dict containing response data and metadata
        """
        url = f"http://{subdomain}/"
        headers = {"Origin": origin}

        try:
            response = requests.get(url, headers=headers, timeout=10)

            return {
                "success": True,
                "status_code": response.status_code,
                "headers": dict(response.headers),
                "data": response.json() if response.status_code == 200 else None,
                "error": None,
            }

        except Exception as e:
            return {
                "success": False,
                "status_code": None,
                "headers": {},
                "data": None,
                "error": str(e),
            }

    def validate_cors_headers(self, headers: Dict[str, str]) -> bool:
        """Validate that proper CORS headers are present."""
        required_cors_header = "Access-Control-Allow-Origin"
        return required_cors_header in headers

    def validate_response_structure(self, data: Dict[str, Any]) -> bool:
        """Validate the structure of dnscheck response."""
        required_fields = ["status", "asn", "asn_organization", "ip", "profile_id"]

        if not data:
            return False

        for field in required_fields:
            if field not in data:
                print(f"Missing required field: {field}")
                return False

        return True

    def determine_connection_status(
        self, response_data: Dict[str, Any], expected_profile_id: str
    ) -> Dict[str, str]:
        """
        Determine connection status based on response data.
        Simulates the frontend logic.
        """
        if not response_data:
            return {
                "status": "error",
                "badge": "Error",
                "message": "Unable to check DNS status",
                "resolver": "",
            }

        if response_data.get("status") == "ok":
            detected_profile_id = response_data.get("profile_id", "")

            if detected_profile_id == expected_profile_id:
                return {
                    "status": "connected",
                    "badge": "Connected",
                    "message": "Good! This device is using modDNS.",
                    "resolver": "This device is currently using modDNS with this profile.",
                }
            elif detected_profile_id:
                return {
                    "status": "different_profile",
                    "badge": "Different Profile",
                    "message": "This device is using modDNS with another profile.",
                    "resolver": f"This device is currently using profile {detected_profile_id}.",
                }
            else:
                return {
                    "status": "connected_no_profile",
                    "badge": "Connected",
                    "message": "Good! This device is using modDNS.",
                    "resolver": "This device is currently using modDNS.",
                }
        else:
            asn_org = response_data.get("asn_organization", "Unknown")
            return {
                "status": "disconnected",
                "badge": "Disconnected",
                "message": "This device is not using modDNS.",
                "resolver": f'This device is currently using "{asn_org}" as DNS resolver.',
            }

    @pytest.mark.integration
    @pytest.mark.asyncio
    async def test_complete_dns_check_flow_with_profile(self, create_account_and_login):
        """Test complete DNS check flow with a specific profile."""
        account, cookie = create_account_and_login
        self.setup_real_profiles(account, cookie)

        profile = self.test_profiles[0]
        profile_id = profile["profile_id"]
        subdomain = self.create_subdomain(profile_id)

        print(f"\nTesting DNS check flow with real profile: {profile['name']}")
        print(f"Profile ID: {profile_id}")
        print(f"Generated subdomain: {subdomain}")

        # Step 1: Send DNS query to cache data
        # TODO: DNS query has to be redirected from proxy to dncheck container, I did not manage to set it up yet
        # Last thing I tried is config/sdns.conf and redirection using custom hosts file - it does not support wildcards though
        dns_success = await self.send_dns_query(subdomain, profile_id)
        assert dns_success, "DNS query should succeed"
        print("✓ DNS query successful")

        # Small delay to ensure cache is updated
        time.sleep(1)

        # Step 2: Send HTTP request
        http_response = self.send_http_request(subdomain)
        assert http_response[
            "success"
        ], f"HTTP request failed: {http_response['error']}"
        assert (
            http_response["status_code"] == 200
        ), f"Expected 200, got {http_response['status_code']}"
        print("✓ HTTP request successful")

        # Step 3: Validate CORS headers
        cors_valid = self.validate_cors_headers(http_response["headers"])
        assert cors_valid, "CORS headers should be present"
        print("✓ CORS headers validated")

        # Step 4: Validate response structure
        print(f"Response data: {http_response['data']}")
        response_valid = self.validate_response_structure(http_response["data"])
        assert response_valid, "Response structure should be valid"
        print("✓ Response structure validated")

        # Step 5: Test status determination logic
        status_info = self.determine_connection_status(
            http_response["data"], profile_id
        )
        print(
            f"✓ Status determined: {status_info['status']} - {status_info['message']}"
        )

        # Additional assertions based on expected behavior
        response_data = http_response["data"]
        assert response_data["status"] == "ok", "DNS check should return 'ok' status"

        # Verify the profile ID in response matches what we sent
        print(f"Expected profile ID: {profile_id}")
        print(f"Received profile ID: {response_data.get('profile_id', 'None')}")

    @pytest.mark.integration
    @pytest.mark.asyncio
    async def test_dns_check_without_profile(self, create_account_and_login):
        """Test DNS check flow without profile ID (empty string)."""
        account, cookie = create_account_and_login
        self.setup_real_profiles(account, cookie)

        subdomain = self.create_subdomain("")  # Empty profile ID

        print(f"\nTesting DNS check flow without profile ID")
        print(f"Generated subdomain: {subdomain}")

        # Step 1: Send DNS query
        dns_success = await self.send_dns_query(subdomain, "")
        assert dns_success, "DNS query should succeed even without profile ID"

        time.sleep(1)

        # Step 2: Send HTTP request
        http_response = self.send_http_request(subdomain)
        assert http_response[
            "success"
        ], f"HTTP request failed: {http_response['error']}"
        assert (
            http_response["status_code"] == 200
        ), f"Expected 200, got {http_response['status_code']}"

        # Step 3: Validate response
        response_valid = self.validate_response_structure(http_response["data"])
        assert response_valid, "Response structure should be valid"

        # Step 4: Check status determination
        status_info = self.determine_connection_status(http_response["data"], "")
        print(
            f"✓ Status determined: {status_info['status']} - {status_info['message']}"
        )

    @pytest.mark.integration
    @pytest.mark.asyncio
    async def test_different_profile_detection(self, create_account_and_login):
        """Test detection when device uses different profile than expected."""
        account, cookie = create_account_and_login
        self.setup_real_profiles(account, cookie)

        # Create a second profile for testing different profile detection
        with client.ApiClient(self.api_config) as api_client:
            profiles_api = api.ProfileApi(api_client)
            profiles_api.api_client.default_headers["Cookie"] = cookie

            # Create a new profile
            from moddns import RequestsCreateProfileBody

            new_profile_body = RequestsCreateProfileBody(
                name="Test Profile 2 for Different Detection"
            )
            new_profile_response = profiles_api.api_v1_profiles_post(
                body=new_profile_body
            )

            # Add the new profile to our test profiles
            self.test_profiles.append(
                {
                    "profile_id": new_profile_response.profile_id,
                    "name": new_profile_response.name,
                    "id": new_profile_response.id,
                }
            )

        # Use profile 1 in subdomain, but expect profile 2
        actual_profile = self.test_profiles[0]["profile_id"]
        expected_profile = self.test_profiles[1]["profile_id"]

        subdomain = self.create_subdomain(actual_profile)

        print(f"\nTesting different profile detection")
        print(f"Subdomain profile: {actual_profile} ({self.test_profiles[0]['name']})")
        print(f"Expected profile: {expected_profile} ({self.test_profiles[1]['name']})")

        # Complete flow
        dns_success = await self.send_dns_query(subdomain, actual_profile)
        assert dns_success, "DNS query should succeed"

        time.sleep(1)

        http_response = self.send_http_request(subdomain)
        assert http_response["success"], "HTTP request should succeed"

        # Status should indicate different profile
        status_info = self.determine_connection_status(
            http_response["data"], expected_profile
        )

        if http_response["data"].get("status") == "ok" and http_response["data"].get(
            "profile_id"
        ):
            assert (
                status_info["status"] == "different_profile"
            ), "Should detect different profile"
            print(f"✓ Different profile correctly detected: {status_info['message']}")
        else:
            print(
                f"Response indicates no valid DNS config or different behavior: {http_response['data']}"
            )

    @pytest.mark.integration
    @pytest.mark.asyncio
    async def test_cors_headers_validation(self, create_account_and_login):
        """Test CORS headers with different origins."""
        account, cookie = create_account_and_login
        self.setup_real_profiles(account, cookie)

        profile_id = self.test_profiles[0]["profile_id"]
        subdomain = self.create_subdomain(profile_id)

        origins_to_test = [
            "http://localhost:5173",
            "http://localhost:5174",
            "https://app.moddns.dev",
            "null",  # For file:// protocol
        ]

        # Send DNS query first
        dns_success = await self.send_dns_query(subdomain, profile_id)
        assert dns_success, "DNS query should succeed"
        time.sleep(1)

        for origin in origins_to_test:
            print(f"\nTesting CORS with origin: {origin}")

            http_response = self.send_http_request(subdomain, origin)
            assert http_response["success"], f"HTTP request failed for origin {origin}"

            cors_valid = self.validate_cors_headers(http_response["headers"])
            assert cors_valid, f"CORS headers should be present for origin {origin}"

            # Check if Access-Control-Allow-Origin is set correctly
            cors_header = http_response["headers"].get("Access-Control-Allow-Origin")
            assert (
                cors_header is not None
            ), "Access-Control-Allow-Origin header should be present"
            print(f"✓ CORS header: {cors_header}")

    @pytest.mark.integration
    @pytest.mark.asyncio
    async def test_http_request_without_dns_query(self, create_account_and_login):
        """Test HTTP request without prior DNS query (should fail or return error)."""
        account, cookie = create_account_and_login
        self.setup_real_profiles(account, cookie)

        profile_id = self.test_profiles[0]["profile_id"]
        subdomain = self.create_subdomain(profile_id)

        print(f"\nTesting HTTP request without prior DNS query")
        print(f"Subdomain: {subdomain}")

        # Skip DNS query, go directly to HTTP request
        http_response = self.send_http_request(subdomain)

        # This should either fail or return an error response
        if http_response["success"]:
            if http_response["status_code"] == 500:
                print("✓ Correctly returned 500 error when no cached data available")
            elif http_response["status_code"] == 200:
                # If it returns 200, check if data indicates no cached info
                data = http_response["data"]
                if not data or data.get("status") != "ok":
                    print("✓ Returned 200 but indicates no valid cached data")
                else:
                    pytest.fail("Should not return valid data without prior DNS query")
        else:
            print(f"✓ HTTP request failed as expected: {http_response['error']}")

    @pytest.mark.integration
    @pytest.mark.asyncio
    async def test_multiple_concurrent_requests(self, create_account_and_login):
        """Test multiple concurrent DNS check requests."""
        account, cookie = create_account_and_login
        self.setup_real_profiles(account, cookie)

        print(f"\nTesting multiple concurrent requests")

        async def single_dns_check(profile_id: str) -> Dict[str, Any]:
            """Perform a single DNS check."""
            subdomain = self.create_subdomain(profile_id)

            # DNS query
            dns_success = await self.send_dns_query(subdomain, profile_id)
            if not dns_success:
                return {"success": False, "error": "DNS query failed"}

            time.sleep(1)

            # HTTP request
            http_response = self.send_http_request(subdomain)
            return http_response

        # Run multiple concurrent requests using the real profile
        profile_id = self.test_profiles[0]["profile_id"]

        # Create tasks for concurrent execution
        tasks = [single_dns_check(profile_id) for _ in range(3)]
        results = await asyncio.gather(*tasks)

        # Validate all requests succeeded
        for i, result in enumerate(results):
            assert result["success"], f"Request {i} should succeed"
            assert result["status_code"] == 200, f"Request {i} should return 200"
            print(f"✓ Concurrent request {i+1} successful")

    @pytest.mark.integration
    @pytest.mark.asyncio
    async def test_performance_timing(self, create_account_and_login):
        """Test the performance of DNS check operations."""
        account, cookie = create_account_and_login
        self.setup_real_profiles(account, cookie)

        profile_id = self.test_profiles[0]["profile_id"]
        subdomain = self.create_subdomain(profile_id)

        print(
            f"\nTesting performance timing with real profile: {self.test_profiles[0]['name']}"
        )

        # Measure DNS query time
        start_time = time.time()
        dns_success = await self.send_dns_query(subdomain, profile_id)
        dns_time = time.time() - start_time

        assert dns_success, "DNS query should succeed"
        print(f"✓ DNS query time: {dns_time:.3f} seconds")

        time.sleep(1)

        # Measure HTTP request time
        start_time = time.time()
        http_response = self.send_http_request(subdomain)
        http_time = time.time() - start_time

        assert http_response["success"], "HTTP request should succeed"
        print(f"✓ HTTP request time: {http_time:.3f} seconds")

        # Total time should be reasonable (less than 10 seconds)
        total_time = dns_time + http_time + 1  # +1 for sleep
        assert (
            total_time < 10
        ), f"Total time {total_time:.3f}s should be under 10 seconds"
        print(f"✓ Total time: {total_time:.3f} seconds")

    @pytest.mark.integration
    @pytest.mark.asyncio
    async def test_frontend_simulation(self, create_account_and_login):
        """Simulate the complete frontend behavior including periodic checks."""
        account, cookie = create_account_and_login
        self.setup_real_profiles(account, cookie)

        profile = self.test_profiles[0]
        profile_id = profile["profile_id"]

        print(
            f"\nSimulating frontend periodic DNS checks with real profile: {profile['name']}"
        )

        # Simulate 3 periodic checks (like the 5-second interval in frontend)
        check_results = []

        for i in range(3):
            print(f"  Check {i+1}/3...")

            subdomain = self.create_subdomain(profile_id)

            # DNS query
            dns_success = await self.send_dns_query(subdomain, profile_id)
            assert dns_success, f"DNS query {i+1} should succeed"

            time.sleep(1)

            # HTTP request
            http_response = self.send_http_request(subdomain)
            assert http_response["success"], f"HTTP request {i+1} should succeed"

            # Status determination
            status_info = self.determine_connection_status(
                http_response["data"], profile_id
            )
            check_results.append(status_info)

            print(f"    Result: {status_info['badge']} - {status_info['message']}")

            # Small delay between checks (simulating frontend interval)
            if i < 2:  # Don't sleep after last check
                time.sleep(2)

        # All checks should be consistent
        first_status = check_results[0]["status"]
        for result in check_results[1:]:
            assert (
                result["status"] == first_status
            ), "All checks should return consistent status"

        print(f"✓ All {len(check_results)} periodic checks consistent")
