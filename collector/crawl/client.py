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
from .robots import RobotsBook, RobotsDisallowed, RobotsPolicy, parse_robots
from .ua import user_agent

# fetch(url) -> html。生产实现用 urllib；测试注入假函数。
Fetcher = Callable[[str], str]

DEFAULT_TIMEOUT = 30.0


def urllib_fetcher(
    timeout: float = DEFAULT_TIMEOUT,
    *,
    headers: dict[str, str] | None = None,
) -> Fetcher:
    """标准库 HTTP 抓取：诚实 UA，非 2xx 抛 HTTPError（4xx 不重试）。

    `headers` 用于按采集面补平台特定头（如京东 JSONP 需要 Accept: */* 与 Referer
    才能稳定拿到 code:0）；不给就是通用的「文档型」头。
    """

    extra = dict(headers or {})

    def fetch(url: str) -> str:
        request = urllib.request.Request(
            url,
            headers={
                "User-Agent": user_agent(),
                "Accept": "text/html,application/xhtml+xml",
                "Accept-Language": "zh-CN,zh;q=0.9",
                **extra,
            },
        )
        with urllib.request.urlopen(request, timeout=timeout) as response:
            charset = response.headers.get_content_charset() or "utf-8"
            return response.read().decode(charset, errors="replace")

    return fetch


class CrawlClient:
    """受 robots + 限速 + 退避约束的抓取客户端。所有等待与绕过都记进 RequestLog。

    `robots` 可以是单个 `RobotsPolicy`（单 host 采集面）或 `RobotsBook`（多 host
    采集面，按 URL host 分派）。两者都提供 `ensure_allowed`，抓取路径不因此分叉。
    """

    def __init__(
        self,
        *,
        robots: RobotsPolicy | RobotsBook,
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


def _looks_like_robots(text: str) -> bool:
    """判断响应体是否真的是 robots.txt（而不是软 200 的错误页 / 首页 / JSON）。

    为什么必须判：实测三个平台都存在「robots.txt 返回 200 但不是 robots」的软 200 陷阱
    ——`paimai.jd.com/robots.txt` 302 到 `www.jd.com/error.html`、`api.m.jd.com/robots.txt`
    返回 `{"code":"1",...}`、`zc.gpai.net/robots.txt` 返回 SPA 首页 HTML。
    这些文本里没有任何 `Disallow` 行，直接交给 `parse_robots` 会解析成「无规则 ⇒ 全部允许」
    ——把「站点没给我们 robots」误判成「站点允许一切」，是合规上的反向错误。

    判据刻意宽松（只排除明显不是 robots 的形态），因为 robots.txt 语法本身很自由：
    真正的 robots 是纯文本行（`User-agent:` / `Allow:` / `Disallow:` / 注释 / 空行）。
    """
    stripped = (text or "").lstrip()
    if not stripped:
        # 空响应 = 没读到内容，等同于读不到。
        return False
    head = stripped[:200].lower()
    if head.startswith(("<!doctype", "<html", "<?xml", "<")):
        return False
    if head.startswith(("{", "[")):
        # JSON 错误体（实测 api.m.jd.com 的形态）。
        return False
    # 至少要有一条 robots 指令，否则无从遵从。
    lowered = stripped.lower()
    return any(token in lowered for token in ("user-agent", "disallow", "allow:", "sitemap"))


def load_robots(
    *,
    base_url: str,
    fetcher: Fetcher,
    user_agent_string: str | None = None,
) -> RobotsPolicy:
    """读站点 robots.txt 并解析成对本 UA 生效的策略。

    读不到 robots（网络错误 / 404 / 软 200 错误页）时**不放行**：fail-closed 返回全禁
    策略——「没读到」不能当作「没有限制」。
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
    if not _looks_like_robots(text):
        # 软 200：拿到了 HTTP 200，但内容不是 robots.txt。同样按「读不到」处理。
        return parse_robots("User-agent: *\nDisallow: /\n", user_agent=ua, host=host)
    return parse_robots(text, user_agent=ua, host=host)


def load_robots_book(
    *,
    origins: tuple[str, ...],
    fetcher: Fetcher,
    user_agent_string: str | None = None,
) -> RobotsBook:
    """按 origin 各读一份 robots，组成多 host 采集面的策略集（#2679）。

    每个 origin 独立 fail-closed：某个 host 读不到 robots 只让**那个 host** 的 URL
    需要人工放行，不会连坐其它 host。这正是多 host 采集面该有的粒度。
    """
    policies = [
        load_robots(
            base_url=origin, fetcher=fetcher, user_agent_string=user_agent_string
        )
        for origin in origins
    ]
    return RobotsBook(tuple(policies))
