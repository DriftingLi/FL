"""限速 + 诚实 UA + 指数退避的单测（#2677 判据 2）。

判据原文：「每平台限速 ≤1 req/2s + 诚实 UA + 指数退避（测试/日志可验证）」。
三条都要可验证，且都要可注入时钟/睡眠，否则测试会真的等 2 秒。

TDD 红：先于实现写下，钉住：
  - 两次请求间隔 ≥ 2s（同一平台）；
  - UA 含产品标识 + 联系方式（诚实 UA 的形态：能被站长找到人）；
  - 失败按指数退避重试（2s → 4s → 8s…），上限后放弃并抛错；
  - 退避与限速的等待都进日志/记录，可被断言。
"""

import unittest

from collector.crawl.limiter import RateLimiter, RequestLog
from collector.crawl.retry import backoff_delays, with_retry
from collector.crawl.ua import user_agent


class FakeClock:
    """可注入时钟：sleep 只推进虚拟时间，测试不真的等。"""

    def __init__(self) -> None:
        self.now = 0.0
        self.sleeps: list[float] = []

    def time(self) -> float:
        return self.now

    def sleep(self, seconds: float) -> None:
        self.sleeps.append(seconds)
        self.now += seconds


class UserAgentTest(unittest.TestCase):
    def test_ua_is_honest(self):
        ua = user_agent()
        # 诚实 UA：有产品名/版本 + 可联系的邮箱或 URL，不伪装浏览器。
        self.assertIn("ForkliftValuationCollector", ua)
        self.assertRegex(ua, r"[+\w.]+@[\w.]+|https?://")
        for masquerade in ("Mozilla", "Chrome", "Safari", "AppleWebKit"):
            self.assertNotIn(masquerade, ua, f"诚实 UA 不得伪装成浏览器：{masquerade}")


class RateLimiterTest(unittest.TestCase):
    def test_enforces_two_second_gap(self):
        clock = FakeClock()
        limiter = RateLimiter(min_interval=2.0, clock=clock.time, sleep=clock.sleep)
        limiter.wait()
        limiter.wait()
        limiter.wait()
        self.assertEqual(clock.sleeps, [2.0, 2.0])
        self.assertEqual(clock.now, 4.0)

    def test_first_request_does_not_wait(self):
        clock = FakeClock()
        limiter = RateLimiter(min_interval=2.0, clock=clock.time, sleep=clock.sleep)
        limiter.wait()
        self.assertEqual(clock.sleeps, [])

    def test_no_wait_when_enough_time_already_passed(self):
        clock = FakeClock()
        limiter = RateLimiter(min_interval=2.0, clock=clock.time, sleep=clock.sleep)
        limiter.wait()
        clock.now += 5.0
        limiter.wait()
        self.assertEqual(clock.sleeps, [])

    def test_interval_never_below_two_seconds(self):
        """判据写死 ≤1 req/2s：配置更小的间隔必须被拒（不能靠配置绕过合规窗口）。"""
        with self.assertRaises(ValueError):
            RateLimiter(min_interval=0.5)


class BackoffTest(unittest.TestCase):
    def test_delays_are_exponential_and_capped(self):
        # attempts = 总尝试次数，故等待数 = attempts - 1。
        self.assertEqual(backoff_delays(attempts=5, base=2.0, cap=30.0), [2.0, 4.0, 8.0, 16.0])
        self.assertEqual(backoff_delays(attempts=6, base=2.0, cap=10.0), [2.0, 4.0, 8.0, 10.0, 10.0])

    def test_single_attempt_has_no_delay(self):
        self.assertEqual(backoff_delays(attempts=1, base=2.0, cap=30.0), [])


class RetryTest(unittest.TestCase):
    def test_retries_until_success_and_records_sleeps(self):
        clock = FakeClock()
        calls = {"n": 0}

        def flaky():
            calls["n"] += 1
            if calls["n"] < 3:
                raise ConnectionError("boom")
            return "ok"

        result = with_retry(
            flaky,
            attempts=4,
            base=2.0,
            cap=30.0,
            sleep=clock.sleep,
        )
        self.assertEqual(result, "ok")
        self.assertEqual(calls["n"], 3)
        self.assertEqual(clock.sleeps, [2.0, 4.0])

    def test_gives_up_after_attempts_and_raises(self):
        clock = FakeClock()

        def always_fail():
            raise ConnectionError("boom")

        with self.assertRaises(ConnectionError):
            with_retry(always_fail, attempts=3, base=2.0, cap=30.0, sleep=clock.sleep)
        self.assertEqual(clock.sleeps, [2.0, 4.0])

    def test_zero_attempts_is_rejected(self):
        with self.assertRaises(ValueError):
            with_retry(lambda: "ok", attempts=0, sleep=FakeClock().sleep)

    def test_does_not_retry_non_retryable(self):
        clock = FakeClock()

        def bad():
            raise ValueError("not transient")

        with self.assertRaises(ValueError):
            with_retry(bad, attempts=3, base=2.0, cap=30.0, sleep=clock.sleep)
        self.assertEqual(clock.sleeps, [])

    def test_does_not_retry_http_4xx_but_retries_5xx(self):
        """4xx 是确定性拒绝，重试只会放大不合规请求量；5xx 才是暂时性失败。"""
        import urllib.error

        clock = FakeClock()
        calls = {"n": 0}

        def forbidden():
            calls["n"] += 1
            raise urllib.error.HTTPError("https://x.test", 403, "Forbidden", {}, None)

        with self.assertRaises(urllib.error.HTTPError):
            with_retry(forbidden, attempts=3, base=2.0, cap=30.0, sleep=clock.sleep)
        self.assertEqual(calls["n"], 1, "403 不得重试")
        self.assertEqual(clock.sleeps, [])

        calls2 = {"n": 0}

        def server_error():
            calls2["n"] += 1
            if calls2["n"] < 2:
                raise urllib.error.HTTPError("https://x.test", 503, "Unavailable", {}, None)
            return "ok"

        self.assertEqual(
            with_retry(server_error, attempts=3, base=2.0, cap=30.0, sleep=clock.sleep),
            "ok",
        )
        self.assertEqual(calls2["n"], 2, "503 应重试")
        self.assertEqual(clock.sleeps, [2.0])


class RequestLogTest(unittest.TestCase):
    def test_log_records_url_and_wait_for_audit(self):
        log = RequestLog()
        log.record(url="https://x.test/a", waited=2.0, status="ok")
        log.record(url="https://x.test/b", waited=0.0, status="robots_override")
        self.assertEqual(len(log.entries), 2)
        self.assertEqual(log.entries[0]["url"], "https://x.test/a")
        self.assertEqual(log.entries[1]["status"], "robots_override")
        self.assertIn("robots_override", log.summary())


if __name__ == "__main__":
    unittest.main()
