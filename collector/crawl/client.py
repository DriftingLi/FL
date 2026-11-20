"""回补抓取器：把发现到的公告 URL 变成 HTML（#2677 判据 1/2）。

三段分离的「采集层」落点：只 fetch，不做语义判断。每次抓取都要过三道闸：
  1. robots 判定（默认遵从，`allow_robots_override` 才放行并记审计）；
  2. 限速（≤1 req/2s，硬下限）；
  3. 指数退避重试（暂时性失败）。

HTTP 客户端可注入（`fetcher` seam）：单测不碰网络，生产用标准库 urllib。
"""

from __future__ import annotations

import urllib.error
import urllib.request
from typing import Callable

from .limiter import RateLimiter, RequestLog
from .retry import with_retry
from .robots import RobotsDisallowed, RobotsPolicy, parse_robots
from .ua import user_agent

# fetch(url) -> html。生产实现用 urllib；测试注入假函数。
Fetcher = Callable[[str], str]

DEFAULT_TIMEOUT = 30.0


def urllib_fetcher(timeout: float = DEFAULT_TIMEOUT) -> Fetcher:
    """标准库 HTTP 抓取：诚实 UA，非 2xx 抛 HTTPError（4xx 不重试）。"""

    def fetch(url: str) -> str:
        request = urllib.request.Request(
            url,
            headers={
                "User-Agent": user_agent(),
                "Accept": "text/html,application/xhtml+xml",
                "Accept-Language": "zh-CN,zh;q=0.9",
            },
        )
        with urllib.request.urlopen(request, timeout=timeout) as response:
            charset = response.headers.get_content_charset() or "utf-8"
            return response.read().decode(charset, errors="replace")

    return fetch


class CrawlClient:
    """受 robots + 限速 + 退避约束的抓取客户端。所有等待与绕过都记进 RequestLog。"""

    def __init__(
        self,
        *,
        robots: RobotsPolicy,
        fetcher: Fetcher,
        limiter: RateLimiter | None = None,
        log: RequestLog | None = None,
        allow_robots_override: bool = False,
    ) -> None:
        self._robots = robots
        self._fetcher = fetcher
        self._limiter = limiter or RateLimiter()
        self._log = log or RequestLog()
        self._allow_override = allow_robots_override

    @property
    def log(self) -> RequestLog:
        return self._log

    def fetch(self, url: str) -> str:
        # ensure_allowed 复用 RobotsPolicy 的统一错误文案（单一真相），
        # 并同时覆盖「robots 禁止」与「跨 host 未核验」两种拒绝。
        try:
            decision = self._robots.ensure_allowed(
                url, allow_override=self._allow_override
            )
        except RobotsDisallowed:
            self._log.record(url=url, waited=0.0, status="robots_disallowed")
            raise
        waited = self._limiter.wait()
        status = "robots_override" if decision.overridden else "ok"
        try:
            html = with_retry(lambda: self._fetcher(url))
        except Exception:
            self._log.record(url=url, waited=waited, status="failed")
            raise
        self._log.record(url=url, waited=waited, status=status)
        return html


def load_robots(
    *,
    base_url: str,
    fetcher: Fetcher,
    user_agent_string: str | None = None,
) -> RobotsPolicy:
    """读站点 robots.txt 并解析成对本 UA 生效的策略。

    读不到 robots（网络错误 / 404）时**不放行**：fail-closed 返回全禁策略——
    「没读到」不能当作「没有限制」。
    """
    from urllib.parse import urlparse

    ua = user_agent_string or user_agent()
    host = urlparse(base_url).netloc
    url = base_url.rstrip("/") + "/robots.txt"
    try:
        text = fetcher(url)
    except Exception:
        # fail-closed：读不到 robots ⇒ 视为全站禁止（空文本会被解析成「允许全部」，
        # 所以这里显式给一条 Disallow: / 规则）。
        return parse_robots("User-agent: *\nDisallow: /\n", user_agent=ua, host=host)
    return parse_robots(text, user_agent=ua, host=host)
