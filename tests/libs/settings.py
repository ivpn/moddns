import pytest

from functools import lru_cache
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Config class holds the configuration for the tests."""
    DNS_API_ADDR: str = "http://localhost:3000"
    DOH_ENDPOINT: str = "https://moddns.dev/dns-query/"


@lru_cache()
def get_settings() -> Settings:
    """Gets the application settings."""
    return Settings()
