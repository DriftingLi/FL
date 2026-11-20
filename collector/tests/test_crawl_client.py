"""CrawlClient 的单测（#2677 判据 2 的可验证落点）。

钉住三道闸的**交互**（各自单测已覆盖，这里验组合）：
  - robots 禁止 ⇒ 不抓、抛 RobotsDisallowed、记 robots_disallowed；
  - 显式 override ⇒ 抓，但日志标 robots_override（审计可见）；
  - 每次成功抓取都过限速（间隔 ≥2s）；
  - 抓取失败记 failed 并向上抛（不静默吞）。
"""

import unittest

from collector.crawl.client import CrawlClient, load_robots
from collector.crawl.limiter import RateLimiter
from collector.crawl.robots import RobotsDisallowed, parse_robots

DENY_ALL = "User-agent: *\nDisallow: /\n"
ALLOW_ALL = "User-agent: *\nDisallow:\n"

TARGET = "https://www.rmfysszc.gov.cn/statichtml/rm_xmdetail/13184763.shtml"


class FakeClock:
    def __init__(self) -> None:
        self.now = 0.0
        self.sleeps: list[float] = []

    def time(self) -> float:
        return self.now

    def sleep(self, seconds: float) -> None:
        self.sleeps.append(seconds)
        self.now += seconds


def _client(robots_text, *, fetcher=None, allow_override=False, clock=None):
    clock = clock or FakeClock()
    calls: list[str] = []

    def default_fetcher(url):
        calls.append(url)
        return "<html>公告</html>"

    client = CrawlClient(
        robots=parse_robots(robots_text, user_agent="ForkliftValuationCollector/1.0"),
        fetcher=fetcher or default_fetcher,
        limiter=RateLimiter(min_interval=2.0, clock=clock.time, sleep=clock.sleep),
        allow_robots_override=allow_override,
    )
    return client, clock, calls


class RobotsGatingTest(unittest.TestCase):
    def test_disallowed_url_is_not_fetched(self):
        client, _, calls = _client(DENY_ALL)
        with self.assertRaises(RobotsDisallowed):
            client.fetch(TARGET)
        self.assertEqual(calls, [], "robots 禁止时不得发出请求")
        self.assertEqual(client.log.entries[0]["status"], "robots_disallowed")

    def test_override_fetches_but_is_logged_as_bypass(self):
        client, _, calls = _client(DENY_ALL, allow_override=True)
        client.fetch(TARGET)
        self.assertEqual(len(calls), 1)
        self.assertEqual(client.log.entries[0]["status"], "robots_override")
        self.assertIn("robots_override", client.log.summary())

    def test_allowed_url_fetches_normally(self):
        client, _, calls = _client(ALLOW_ALL)
        html = client.fetch(TARGET)
        self.assertEqual(html, "<html>公告</html>")
        self.assertEqual(client.log.entries[0]["status"], "ok")


class RateLimitIntegrationTest(unittest.TestCase):
    def test_successive_fetches_are_spaced(self):
        client, clock, _ = _client(ALLOW_ALL)
        client.fetch(TARGET)
        client.fetch(TARGET)
        client.fetch(TARGET)
        self.assertEqual(clock.sleeps, [2.0, 2.0], "连续抓取必须按 ≤1 req/2s 间隔")

    def test_waited_time_is_recorded(self):
        client, _, _ = _client(ALLOW_ALL)
        client.fetch(TARGET)
        client.fetch(TARGET)
        self.assertEqual(client.log.entries[1]["waited"], 2.0)


class FailureHandlingTest(unittest.TestCase):
    def test_fetch_failure_is_logged_and_reraised(self):
        def boom(url):
            raise ValueError("确定性失败，不重试")

        client, _, _ = _client(ALLOW_ALL, fetcher=boom)
        with self.assertRaises(ValueError):
            client.fetch(TARGET)
        self.assertEqual(client.log.entries[0]["status"], "failed")


class LoadRobotsTest(unittest.TestCase):
    def test_reads_site_robots_and_denies(self):
        def fetcher(url):
            assert url.endswith("/robots.txt"), url
            return DENY_ALL

        policy = load_robots(
            base_url="https://www.rmfysszc.gov.cn", fetcher=fetcher
        )
        self.assertFalse(policy.allowed(TARGET))

    def test_unreadable_robots_fails_closed(self):
        def fetcher(url):
            raise OSError("network down")

        policy = load_robots(
            base_url="https://www.rmfysszc.gov.cn", fetcher=fetcher
        )
        self.assertFalse(
            policy.allowed(TARGET),
            "读不到 robots 必须按禁止处理（fail-closed），不能当作没限制",
        )


if __name__ == "__main__":
    unittest.main()
