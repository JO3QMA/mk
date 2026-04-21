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


def _ensure_user_id(client: MisskeyLikeClient, raw: dict) -> dict:
    """Return raw with an `id` field even if create_admin fell through to signin.

    create_admin returns either:
    - admin/accounts/create response (fresh): `{id, username, token, ...}`
    - signin response (existing): `{i: token, finished: true}` (no id)

    Phase 13-2 の swap シナリオでは setup → swap → verify と pytest セッションが
    分かれるため、後段 (verify) は必ず signin パスに落ちる。`alice["id"]` を
    使う test_swap_verify.py が動くように /api/i で hydrate する。
    """
    if "id" in raw:
        return raw
    me = client._api("i")
    return {**me, **raw}


@pytest.fixture(scope="session")
def alice(instance_a: MisskeyLikeClient) -> dict:
    """Root user on instance A."""
    return _ensure_user_id(instance_a, instance_a.create_admin("alice", "password1234"))


@pytest.fixture(scope="session")
def bob(instance_b: MisskeyLikeClient) -> dict:
    """Root user on instance B."""
    return _ensure_user_id(instance_b, instance_b.create_admin("bob", "password1234"))
