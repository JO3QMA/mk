"""Phase 13-2 (#367) drop-in swap シナリオ: verify 段階。

`run-swap-test.sh` が test_swap_setup.py を実行 → backend を mk-go に差し替え
→ この test を実行する流れ。instance A の URL は変わらず https://a/ のまま
だが、その背後で動いている backend が TS から mk-go に切り替わっている。

DB-A / Redis-A は無事なので alice の token / follow 関係 / home timeline 内容
がすべて引き継がれているはず。さらに mk-A 上で alice が新しい操作 (reply,
reaction) を行い、TS-B 側に federation 経由で届くことも確認する。
"""

from __future__ import annotations

import time

import pytest

from conftest import B_DOMAIN  # type: ignore[import-not-found]
from conftest_base import MisskeyLikeClient, poll_until  # type: ignore[import-not-found]
from test_swap_setup import BASELINE_NOTE_TEXT  # type: ignore[import-not-found]


def test_post_swap_baseline_note_preserved(
    instance_a: MisskeyLikeClient,
    alice: dict,
) -> None:
    """切替前に bob が投稿し alice の home に届いていた note が、mk-A でも
    引き続き読めることを確認する。Redis prefix が TS と揃っていなければ
    ここでタイムラインが空になる (まさに #362 の症状)。
    """
    tl = instance_a._api("notes/timeline", {"limit": 40})
    assert any(n.get("text") == BASELINE_NOTE_TEXT for n in tl), \
        "baseline note disappeared from alice's home after backend swap"


@pytest.mark.xfail(
    reason="mk-go の NoteDeliveryHook (#368) は reply target 個別 deliver を未実装。"
    "alice の followers (=0 人) + text mention (=なし) しか対象にしないため、"
    "リモートユーザーへの reply は配信されない。drop-in 切替に固有の問題ではなく、"
    "fresh インストールでも同じ振る舞い。Phase 13-3 で federation 修正後に xfail 解除。",
    strict=True,
)
def test_post_swap_alice_can_reply(
    instance_a: MisskeyLikeClient,
    instance_b: MisskeyLikeClient,
    alice: dict,
    bob: dict,
) -> None:
    """mk-A で alice が新しいリプライを投稿し、TS-B の bob 側に届くことを確認する。

    federation deliver のキュー (asynq) が稼働していれば届く。これが届かない
    場合は mk-go の deliver 処理 / SSL_CERT_FILE バンドル / inbox URL 解決
    のいずれかが壊れている。
    """
    tl = instance_a._api("notes/timeline", {"limit": 40})
    baseline = next((n for n in tl if n.get("text") == BASELINE_NOTE_TEXT), None)
    assert baseline is not None, "baseline note missing — cannot reply"

    reply_text = f"dropin-reply-{int(time.time())}"
    instance_a._api(
        "notes/create",
        {"text": reply_text, "replyId": baseline["id"]},
    )

    def _arrived() -> bool:
        notifications = instance_b.get_notifications(limit=20)
        return any(
            (n.get("note") or {}).get("text") == reply_text
            for n in notifications
        )

    assert poll_until(_arrived, timeout=120, desc="bob receives reply from mk-A alice")


@pytest.mark.xfail(
    reason="reaction の federation deliver も #368 と同じ NoteDeliveryHook 経路を"
    "通るため、リモートユーザーへ reaction が配信されない。Phase 13-3 で連動して"
    "解除予定。",
    strict=True,
)
def test_post_swap_alice_can_react(
    instance_a: MisskeyLikeClient,
    instance_b: MisskeyLikeClient,
    alice: dict,
    bob: dict,
) -> None:
    """mk-A の alice が baseline note にリアクションを付け、TS-B の bob 側に
    届くことを確認する。
    """
    tl = instance_a._api("notes/timeline", {"limit": 40})
    baseline = next((n for n in tl if n.get("text") == BASELINE_NOTE_TEXT), None)
    assert baseline is not None

    try:
        instance_a.react(baseline["id"], "👍")
    except RuntimeError as e:
        # 切替前に既にリアクション済みの場合は許容 (ALREADY_REACTED 等)
        if "ALREADY" not in str(e).upper():
            raise

    def _arrived() -> bool:
        notifications = instance_b.get_notifications(limit=20)
        for n in notifications:
            if n.get("type") != "reaction":
                continue
            from_user = n.get("user") or {}
            if from_user.get("host") in (None, "") or from_user.get("username") == "alice":
                return True
        return False

    assert poll_until(_arrived, timeout=120, desc="bob receives reaction from mk-A alice")
