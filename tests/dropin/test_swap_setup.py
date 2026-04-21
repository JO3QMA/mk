"""Phase 13-2 (#367) drop-in swap シナリオ: setup 段階。

`run-swap-test.sh` から `pytest test_swap_setup.py` の形で実行され、TS-A の
backend が動いている状態で alice/bob/follow/note を作る。後段の `pytest
test_swap_verify.py` は backend が mk-go に差し替わった後で同じ instance A
URL に対してこの state が残存していることを確認する。

ここで作る state は DB-A / Redis-A に persist される。docker compose stop で
backend だけ落として overlay で mk-go を立て直しても DB / Redis は無事。
"""

from __future__ import annotations

from conftest import A_DOMAIN, B_DOMAIN  # type: ignore[import-not-found]
from conftest_base import MisskeyLikeClient, poll_until  # type: ignore[import-not-found]

# verify 段で使う marker。run-swap-test.sh が同じ pytest コマンドを 2 回呼ぶので
# テストファイル間で値を共有する必要があり、定数として定義する。
BASELINE_NOTE_TEXT = "dropin-baseline-pre-swap"


def test_setup_alice_follows_bob(
    instance_a: MisskeyLikeClient,
    instance_b: MisskeyLikeClient,
    alice: dict,
    bob: dict,
) -> None:
    """alice@a が bob@b を AP 経由で resolve して follow する。"""
    remote_bob = poll_until(
        lambda: instance_a.users_show("bob", host=B_DOMAIN),
        timeout=60,
        desc="alice resolves bob via AP",
    )
    try:
        instance_a.follow(remote_bob["id"])
    except RuntimeError as e:
        if "ALREADY_FOLLOWING" not in str(e):
            raise

    def _followed() -> bool:
        followers = instance_b._api("users/followers", {"userId": bob["id"]})
        for f in followers:
            follower = f.get("follower") or f
            host = follower.get("host") or follower.get("followerHost")
            if host == A_DOMAIN:
                return True
        return False

    assert poll_until(_followed, timeout=60, desc="bob registers alice as follower")


def test_setup_baseline_note(
    instance_a: MisskeyLikeClient,
    instance_b: MisskeyLikeClient,
    alice: dict,
    bob: dict,
) -> None:
    """bob が baseline note を投稿し alice の home に届くことを確認する。"""
    instance_b.create_note(BASELINE_NOTE_TEXT)

    def _arrived() -> bool:
        tl = instance_a._api("notes/timeline", {"limit": 40})
        return any(n.get("text") == BASELINE_NOTE_TEXT for n in tl)

    assert poll_until(_arrived, timeout=90, desc="alice receives baseline note")
