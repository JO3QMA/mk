"""Fixtures for mk-go ↔ Misskey federation tests."""

from __future__ import annotations

import os
import sys

import pytest

# `common/` is mounted alongside this directory inside the test-runner
# container (see docker-compose.federation.misskey.yml). Add it to sys.path
# so we can import the shared helpers.
_COMMON_DIR = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "common")
if _COMMON_DIR not in sys.path:
    sys.path.insert(0, _COMMON_DIR)

from conftest_base import MisskeyLikeClient, wait_for_health  # noqa: E402

MKGO_URL = os.environ.get("MKGO_URL", "https://mkgo")
MISSKEY_URL = os.environ.get("MISSKEY_URL", "https://misskey")
MKGO_DOMAIN = os.environ.get("MKGO_DOMAIN", "mkgo")
MISSKEY_DOMAIN = os.environ.get("MISSKEY_DOMAIN", "misskey")


@pytest.fixture(scope="session", autouse=True)
def wait_for_instances() -> None:
    """Block until both instances can accept API requests."""
    wait_for_health(MKGO_URL, "/healthz")
    wait_for_health(MISSKEY_URL, "/api/ping", method="POST")


@pytest.fixture(scope="session")
def mkgo() -> MisskeyLikeClient:
    return MisskeyLikeClient(MKGO_URL, MKGO_DOMAIN)


@pytest.fixture(scope="session")
def misskey() -> MisskeyLikeClient:
    return MisskeyLikeClient(MISSKEY_URL, MISSKEY_DOMAIN)


@pytest.fixture(scope="session")
def alice(mkgo: MisskeyLikeClient) -> dict:
    """First user on mk-go (= root)."""
    return mkgo.create_admin("alice", "password1234")


@pytest.fixture(scope="session")
def bob(misskey: MisskeyLikeClient) -> dict:
    """First user on Misskey (= root)."""
    return misskey.create_admin("bob", "password1234")
