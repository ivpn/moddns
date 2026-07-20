from functools import lru_cache
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Config class holds the configuration for the tests.

    Every field is overridable via an identically-named environment variable.
    Defaults match the port mappings in ``tests/docker-compose.yml``.
    """
    DNS_API_ADDR: str = "http://localhost:3000"
    DOH_ENDPOINT: str = "https://moddns.dev/dns-query/"
    REDIS_HOST: str = "localhost"
    REDIS_PORT: int = 6379
    MOCK_PREAUTH_URL: str = "http://localhost:8080"


@lru_cache()
def get_settings() -> Settings:
    """Gets the application settings."""
    return Settings()
