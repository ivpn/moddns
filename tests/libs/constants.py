"""Shared deterministic test constants — single source of truth.

Historically two different ``TEST_DOMAIN`` constants existed (``example.com``
in conftest = blocklisted, ``test.com`` in profile_helpers = resolvable) with
opposite meanings. Import from this module and use the explicit names below;
never redefine these in test files.
"""

# The blocklist seeded by fixtures and enabled on new profiles by default.
TEST_BLOCKLIST_ID = "hagezi_threat_intelligence_feeds_full"

# Inserted into TEST_BLOCKLIST_ID by the ensure_test_blocklisted fixture, so it
# is BLOCKED for profiles with the default blocklist enabled. Resolvable upstream.
BLOCKLISTED_DOMAIN = "example.com"
# Intentionally NOT inserted into the blocklist; used to validate inherited
# subdomain blocking.
BLOCKLISTED_SUBDOMAIN = f"sub.{BLOCKLISTED_DOMAIN}"

# Pinned in config/testhosts.txt (and mirrored in config/knot.config.yaml) —
# resolves deterministically to RESOLVABLE_TEST_IP and is in NO blocklist.
RESOLVABLE_TEST_DOMAIN = "test.com"
RESOLVABLE_TEST_IP = "104.18.74.230"  # AS13335 (Cloudflare, not in catalog)
