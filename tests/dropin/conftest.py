"""Pytest fixtures for the drop-in e2e suite (#365).

Two Misskey TS instances (A, B) run against their own Postgres / Redis pair
and federate via self-signed TLS. Phase 13-1 only verifies that this
baseline federates correctly; Phase 13-2 will add an overlay that swaps
instance A's backend with mk-go.
"""

from __future__ import annotations

import os
import sys

import pytest

# `tests/federation/common/` is mounted at /tests/federation_common in the
# test-runner (see docker-compose.dropin.yml). Reuse `conftest_base.py`
# since mk-go's federation suite already maintains a
# Misskey-compatible API client there.
_COMMON_DIR = "/tests/federation_common"
if os.path.isdir(_COMMON_DIR) and _COMMON_DIR not in sys.path:
    sys.path.insert(0, _COMMON_DIR)
else:
    # Local/host runs (no docker mount) fall back to the repo path so the
    # suite can be imported by linters or type-checkers.
    _LOCAL_COMMON = os.path.abspath(
        os.path.join(os.path.dirname(__file__), "..", "federation", "common")
    )
    if _LOCAL_COMMON not in sys.path:
        sys.path.insert(0, _LOCAL_COMMON)

from conftest_base import MisskeyLikeClient, wait_for_health  # noqa: E402

A_URL = os.environ.get("A_URL", "https://a")
B_URL = os.environ.get("B_URL", "https://b")
A_DOMAIN = os.environ.get("A_DOMAIN", "a")
B_DOMAIN = os.environ.get("B_DOMAIN", "b")


@pytest.fixture(scope="session", autouse=True)
def wait_for_instances() -> None:
    """Block until both TS instances can accept API requests."""
    wait_for_health(A_URL, "/api/ping", method="POST")
    wait_for_health(B_URL, "/api/ping", method="POST")


@pytest.fixture(scope="session")
def instance_a() -> MisskeyLikeClient:
    return MisskeyLikeClient(A_URL, A_DOMAIN)


@pytest.fixture(scope="session")
def instance_b() -> MisskeyLikeClient:
    return MisskeyLikeClient(B_URL, B_DOMAIN)


@pytest.fixture(scope="session")
def alice(instance_a: MisskeyLikeClient) -> dict:
    """Root user on instance A."""
    return instance_a.create_admin("alice", "password1234")


@pytest.fixture(scope="session")
def bob(instance_b: MisskeyLikeClient) -> dict:
    """Root user on instance B."""
    return instance_b.create_admin("bob", "password1234")
