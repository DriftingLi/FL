"""指数退避重试（#2677 判据 2）。

只重试「暂时性」失败（网络 / 超时 / 5xx）；4xx 这类确定性拒绝不重试——重试不会
变好，只会把不合规的访问量放大。睡眠函数可注入，测试不真的等。
"""

from __future__ import annotations

import time
import urllib.error
from typing import Callable, TypeVar

from .limiter import DEFAULT_ATTEMPTS, DEFAULT_BACKOFF_BASE, DEFAULT_BACKOFF_CAP

T = TypeVar("T")

# 可重试的异常：连接/超时属暂时性；其余（ValueError 等）不重试。
# 注意：urllib.error.HTTPError 是 OSError 的子类，会落进这里，所以要在
# _is_retryable 里单独把 4xx 排除掉——不能只靠异常类型元组。
RETRYABLE_EXCEPTIONS: tuple[type[BaseException], ...] = (
    ConnectionError,
    TimeoutError,
    OSError,
)


def _is_retryable(exc: BaseException) -> bool:
    """HTTP 4xx 是确定性拒绝（请求再多次也不会变），不重试；5xx 与网络错误重试。"""
    if isinstance(exc, urllib.error.HTTPError):
        return exc.code >= 500
    return isinstance(exc, RETRYABLE_EXCEPTIONS)


def backoff_delays(
    attempts: int,
    *,
    base: float = DEFAULT_BACKOFF_BASE,
    cap: float = DEFAULT_BACKOFF_CAP,
) -> list[float]:
    """第 n 次失败后的等待：base * 2^(n-1)，封顶 cap。返回 attempts-1 个等待。"""
    delays: list[float] = []
    for index in range(max(0, attempts - 1)):
        delays.append(min(cap, base * (2 ** index)))
    return delays


def with_retry(
    operation: Callable[[], T],
    *,
    attempts: int = DEFAULT_ATTEMPTS,
    base: float = DEFAULT_BACKOFF_BASE,
    cap: float = DEFAULT_BACKOFF_CAP,
    sleep: Callable[[float], None] = time.sleep,
) -> T:
    """执行 operation，暂时性失败按指数退避重试；用尽后抛最后一次异常。"""
    if attempts < 1:
        raise ValueError("attempts 必须 >= 1")
    delays = backoff_delays(attempts, base=base, cap=cap)
    last_error: BaseException | None = None
    for index in range(attempts):
        try:
            return operation()
        except BaseException as exc:
            if not _is_retryable(exc):
                raise
            last_error = exc
            if index >= len(delays):
                break
            sleep(delays[index])
    assert last_error is not None
    raise last_error
