"""采集层（#2677）：robots 遵从 + 限速 + 诚实 UA + 指数退避。

三段分离（spec #2671）：本包只负责「把 URL 变成 HTML」，不做任何语义判断；
抽取 / 清洗 / 匹配在 collector 的其它包里，回补与增量共用同一份。
"""

from .client import CrawlClient, Fetcher, load_robots, urllib_fetcher
from .limiter import MIN_INTERVAL_SECONDS, RateLimiter, RequestLog
from .retry import backoff_delays, with_retry
from .robots import RobotsDecision, RobotsDisallowed, RobotsPolicy, parse_robots
from .ua import user_agent

__all__ = [
    "CrawlClient",
    "Fetcher",
    "MIN_INTERVAL_SECONDS",
    "RateLimiter",
    "RequestLog",
    "RobotsDecision",
    "RobotsDisallowed",
    "RobotsPolicy",
    "backoff_delays",
    "load_robots",
    "parse_robots",
    "urllib_fetcher",
    "user_agent",
    "with_retry",
]
