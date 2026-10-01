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
    # Credentials match tests/docker-compose.yml (MONGO_INITDB_ROOT_*).
    MONGO_URI: str = "mongodb://admin:admin@localhost:27017/?authSource=admin"
    MONGO_DB: str = "dns"
    MOCK_PREAUTH_URL: str = "http://localhost:8080"
    # Must match API_PSK in config/api.env — the PSK middleware fails closed
    # when the secret is unset, so an empty value no longer passes.
    API_PSK: str = "e2e-test-psk"
    # Third-party resolver clients from docker-compose.yml (test_adguard_home.py,
    # test_pihole.py), reached on their published host ports like everything else.
    ADGUARD_HOME_API_ADDR: str = "http://localhost:5391"
    ADGUARD_HOME_DNS_ADDR: str = "localhost:5390"
    PIHOLE_DNS_ADDR: str = "localhost:5392"
    # Fixed compose address of the proxy (docker-compose.yml). A client *container*
    # dials this; the API's stamps encode the host loopback (config/api.env), so
    # stamps handed to a container are re-addressed to it (libs/stamps.py).
    PROXY_NETWORK_ADDR: str = "10.5.0.100"
    # knot on the compose network: the plain resolver a client container uses to
    # bootstrap the proxy's hostnames (config/knot.config.yaml).
    BOOTSTRAP_DNS_ADDR: str = "10.5.0.6"


@lru_cache()
def get_settings() -> Settings:
    """Gets the application settings."""
    return Settings()
