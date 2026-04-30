"""Queue depth probes for the three drivers under test.

Each driver stores queue state under different Redis key conventions:

- BullMQ (Misskey TS):     bull:<queue>:wait, bull:<queue>:active (LIST)
- asynq:                   asynq:{<queue>}:pending, asynq:{<queue>}:active (LIST)
- mkq (BullMQ-compatible): bull:<queue>:wait, bull:<queue>:active (LIST)

The probe abstracts these so the bench drivers can poll a uniform
"pending + active" depth across the three stacks.
"""
from __future__ import annotations

from dataclasses import dataclass
from typing import Literal

import redis

DriverKind = Literal["bullmq", "asynq", "mkq"]


@dataclass
class QueueProbe:
    name: str  # human label e.g. "ts" / "asynq" / "mkq"
    kind: DriverKind
    client: redis.Redis
    queue_name: str  # "deliver" / "inbox" / etc.

    def depth(self) -> int:
        """Pending + active job count for the configured queue."""
        if self.kind == "asynq":
            wait = f"asynq:{{{self.queue_name}}}:pending"
            active = f"asynq:{{{self.queue_name}}}:active"
            scheduled = f"asynq:{{{self.queue_name}}}:scheduled"
            retry = f"asynq:{{{self.queue_name}}}:retry"
            return (
                self.client.llen(wait)
                + self.client.llen(active)
                + self.client.zcard(scheduled)
                + self.client.zcard(retry)
            )
        # bullmq / mkq are wire-compatible
        wait = f"bull:{self.queue_name}:wait"
        active = f"bull:{self.queue_name}:active"
        delayed = f"bull:{self.queue_name}:delayed"
        return (
            self.client.llen(wait)
            + self.client.llen(active)
            + self.client.zcard(delayed)
        )

    def completed(self) -> int:
        """Approximate completed job count (driver-specific best effort)."""
        if self.kind == "asynq":
            # asynq tracks per-day counters: asynq:{<q>}:processed:<YYYY-MM-DD>
            # 簡易には "completed - failed" を厳密に追えないので 0 を返す。
            # drain time の判定は depth() == 0 で十分。
            return 0
        completed = f"bull:{self.queue_name}:completed"
        return int(self.client.zcard(completed))


def make_probe(name: str, kind: DriverKind, host: str, queue_name: str, port: int = 6379, db: int = 0) -> QueueProbe:
    client = redis.Redis(host=host, port=port, db=db, decode_responses=False)
    return QueueProbe(name=name, kind=kind, client=client, queue_name=queue_name)
