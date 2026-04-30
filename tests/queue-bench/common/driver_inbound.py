"""Inbound inbox-throughput driver (#563).

Drives the faker control API to blast pre-signed AP Create activities
at each receiver's `/inbox`, while polling the receiver's inbox queue
depth until drained. Compares pure receiver inbox processing throughput
across the three drivers.
"""
from __future__ import annotations

import json
import os
import sys
import time
from typing import Any

import httpx

from queue_probe import DriverKind, make_probe

INBOUND_COUNT = int(os.environ.get("INBOUND_COUNT", "10000"))
INBOUND_CONCURRENCY = int(os.environ.get("INBOUND_CONCURRENCY", "128"))
POLL_INTERVAL_S = 0.1
DRAIN_TIMEOUT_S = 600.0
FAKER_URL = os.environ["FAKER_URL"]

STACK_PROBES: dict[str, tuple[DriverKind, str]] = {
    "asynq": ("asynq", os.environ["REDIS_ASYNQ_HOST"]),
    "mkq": ("mkq", os.environ["REDIS_MKQ_HOST"]),
    "ts": ("bullmq", os.environ["REDIS_TS_HOST"]),
}

# Receiver hostname → inbox URL に変換するためのマップ。faker は `target` に
# 受信側 inbox URL をそのまま受け取る。
INBOX_URLS = {
    "asynq": "https://mk-asynq/inbox",
    "mkq": "https://mk-mkq/inbox",
    "ts": "https://ts/inbox",
}


def faker_send(targets: list[str], count: int, concurrency: int) -> dict[str, Any]:
    payload = {"targets": targets, "count": count, "concurrency": concurrency}
    # send は count*len(targets) 件を blasting するので timeout は十分長く。
    r = httpx.post(f"{FAKER_URL}/send", json=payload, timeout=DRAIN_TIMEOUT_S)
    r.raise_for_status()
    return r.json()


def drain_one(stack: str, info: dict[str, Any], expected: int) -> dict[str, Any]:
    kind, redis_host = STACK_PROBES[stack]
    probe = make_probe(stack, kind, redis_host, "inbox")

    samples: list[dict[str, float]] = []
    deadline = time.monotonic() + DRAIN_TIMEOUT_S
    start = time.monotonic()
    drained_at: float | None = None

    while time.monotonic() < deadline:
        depth = probe.depth()
        samples.append({"t": time.monotonic() - start, "depth": depth})
        if depth == 0 and len(samples) > 1:
            drained_at = samples[-1]["t"]
            break
        time.sleep(POLL_INTERVAL_S)

    drain_time = drained_at if drained_at is not None else (time.monotonic() - start)
    timed_out = drained_at is None
    throughput = expected / drain_time if drain_time > 0 else 0.0
    peak = max((s["depth"] for s in samples), default=0)
    return {
        "stack": stack,
        "expected_jobs": expected,
        "drain_seconds": drain_time,
        "timed_out": timed_out,
        "throughput_jobs_per_sec": throughput,
        "peak_queue_depth": peak,
        "samples": samples[-200:],
    }


def main() -> int:
    with open("/state/seed.json") as f:
        seed = json.load(f)

    targets = [INBOX_URLS[stack] for stack in seed["stacks"].keys()]

    print(
        f"firing faker: {len(targets)} targets x {INBOUND_COUNT} count "
        f"({INBOUND_COUNT*len(targets)} total) @ concurrency={INBOUND_CONCURRENCY}",
        flush=True,
    )

    # faker.send は同期的に blast 完了まで待つ。一方で receiver の inbox
    # ジョブは送出と同時にエンキューされ始めるので、receiver 側の drain
    # 監視は faker 呼び出しと並行で行う必要がある。並行 thread で立ち
    # 上げる。
    import threading

    drain_results: dict[str, dict[str, Any]] = {}

    def drain_thread(stack: str, info: dict[str, Any]) -> None:
        drain_results[stack] = drain_one(stack, info, INBOUND_COUNT)

    threads = [
        threading.Thread(target=drain_thread, args=(stack, info))
        for stack, info in seed["stacks"].items()
    ]
    for t in threads:
        t.start()

    # send 開始
    send_start = time.monotonic()
    send_resp = faker_send(targets, INBOUND_COUNT, INBOUND_CONCURRENCY)
    send_elapsed = time.monotonic() - send_start
    print(
        f"faker send done in {send_elapsed:.2f}s "
        f"(presign {send_resp['preSignMs']:.0f}ms, total {send_resp['totalMs']:.0f}ms)",
        flush=True,
    )

    for t in threads:
        t.join()

    out = {
        "send": send_resp,
        "send_elapsed_s": send_elapsed,
        "drain": drain_results,
    }
    out_path = "/results/inbound.json"
    with open(out_path, "w") as f:
        json.dump(out, f, indent=2)
    print(f"inbound.json written to {out_path}", flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
