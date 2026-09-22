from functools import lru_cache
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Config class holds the configuration for the tests.

    Every field is overridable via an identically-named environment variable.
    Defaults match the port mappings in ``tests/docker-compose.yml``.
    """
    DNS_API_ADDR: str = "http://localhost:3000"
    # dnscheck HTTP API; the probe hostname goes in the Host header because the
    # public check zone resolves to production, not to the test stack.
    DNSCHECK_API_ADDR: str = "http://localhost:30080"
    DOH_ENDPOINT: str = "https://moddns.dev/dns-query/"
    REDIS_HOST: str = "localhost"
    REDIS_PORT: int = 6379
    MOCK_PREAUTH_URL: str = "http://localhost:8080"
    # Must match API_PSK in config/api.env — the PSK middleware fails closed
    # when the secret is unset, so an empty value no longer passes.
    API_PSK: str = "e2e-test-psk"


@lru_cache()
def get_settings() -> Settings:
    """Gets the application settings."""
    return Settings()
