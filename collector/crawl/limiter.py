"""限速 / 诚实 UA / 指数退避（#2677 判据 2）。

三条判据都要「测试/日志可验证」，所以时间来源全部可注入：测试用假时钟推进虚拟
时间，生产用 `time.time` / `time.sleep`。限速间隔的**下限写死 2 秒**——判据是
≤1 req/2s 的合规窗口，不能靠配置把它调小绕过。
"""

from __future__ import annotations

import time
from dataclasses import dataclass, field
from typing import Callable

# 判据硬下限：≤1 req/2s。低于它的配置直接拒绝（合规窗口不是可调参数）。
MIN_INTERVAL_SECONDS = 2.0

# 指数退避默认值：2s → 4s → 8s → …，封顶 30s（避免一次故障拖死整轮回补）。
DEFAULT_BACKOFF_BASE = 2.0
DEFAULT_BACKOFF_CAP = 30.0
DEFAULT_ATTEMPTS = 4


class RateLimiter:
    """同一平台的最小请求间隔门。首请求不等，之后每次至少隔 min_interval 秒。"""

    def __init__(
        self,
        min_interval: float = MIN_INTERVAL_SECONDS,
        *,
        clock: Callable[[], float] = time.monotonic,
        sleep: Callable[[float], None] = time.sleep,
    ) -> None:
        if min_interval < MIN_INTERVAL_SECONDS:
            raise ValueError(
                f"限速间隔不得低于 {MIN_INTERVAL_SECONDS}s（判据 ≤1 req/2s，合规窗口不可调）"
            )
        self._min_interval = min_interval
        self._clock = clock
        self._sleep = sleep
        self._last: float | None = None

    def wait(self) -> float:
        """等到可发下一次请求；返回本次实际等待秒数（供日志/断言）。"""
        if self._last is None:
            self._last = self._clock()
            return 0.0
        elapsed = self._clock() - self._last
        waited = 0.0
        if elapsed < self._min_interval:
            waited = self._min_interval - elapsed
            self._sleep(waited)
        self._last = self._clock()
        return waited


@dataclass
class RequestLog:
    """请求审计记录：URL、等待时长、状态（ok / robots_override / retry / failed）。

    「测试/日志可验证」的落点：限速等待与 robots 绕过都留痕，回补结束可整体打印。
    """

    entries: list[dict] = field(default_factory=list)

    def record(self, *, url: str, waited: float, status: str, detail: str = "") -> None:
        self.entries.append(
            {"url": url, "waited": round(waited, 3), "status": status, "detail": detail}
        )

    def summary(self) -> str:
        if not self.entries:
            return "请求日志：空"
        by_status: dict[str, int] = {}
        total_wait = 0.0
        for entry in self.entries:
            by_status[entry["status"]] = by_status.get(entry["status"], 0) + 1
            total_wait += entry["waited"]
        parts = ", ".join(f"{k}={v}" for k, v in sorted(by_status.items()))
        return f"请求日志：{len(self.entries)} 次（{parts}），累计等待 {total_wait:.1f}s"
