"""增量 daemon 的节奏循环单测（#2678 判据 1/4）。

第一性原理：spec #2671 user story 20 要求「历史回补与增量采集是同一组件的两种模式，
so that 清洗与匹配逻辑共用一份、不漂移」。所以 daemon **不是**第二条管线——它就是
「把回补那一轮按周重复」。本模块只负责节奏与失败隔离，管线本身一行不复制。

TDD 红：先于实现写下，钉住：
  - 周节奏：一轮跑完到下一轮开始之间补足一周（周期耗时不能累积漂移）；
  - 失败隔离：某一轮抛错不许杀死 daemon（周更任务挂一次就永久停摆是不可接受的）；
  - 循环可终止：max_cycles 供测试与「只跑一轮」运维场景；
  - 时钟/睡眠可注入，测试不真的等一周。
"""

import unittest

from collector.daemon import WEEK_SECONDS, CycleResult, run_daemon


class FakeClock:
    """可注入时钟：sleep 只推进虚拟时间，测试不真的等一周。"""

    def __init__(self, cycle_cost: float = 0.0) -> None:
        self.now = 0.0
        self.sleeps: list[float] = []
        self._cycle_cost = cycle_cost

    def time(self) -> float:
        return self.now

    def sleep(self, seconds: float) -> None:
        self.sleeps.append(seconds)
        self.now += seconds

    def charge_cycle(self) -> None:
        """模拟一轮真实耗时（抓取/抽取要花时间）。"""
        self.now += self._cycle_cost


def _cycle(clock: FakeClock, *, fail_times: int = 0):
    state = {"calls": 0}

    def cycle() -> tuple[int, object]:
        state["calls"] += 1
        clock.charge_cycle()
        if state["calls"] <= fail_times:
            raise ConnectionError(f"第 {state['calls']} 轮失败")
        return 3, "report"

    cycle.calls = lambda: state["calls"]  # type: ignore[attr-defined]
    return cycle


class CadenceTest(unittest.TestCase):
    def test_week_is_seven_days(self):
        self.assertEqual(WEEK_SECONDS, 7 * 24 * 3600)

    def test_runs_max_cycles_then_stops(self):
        clock = FakeClock()
        cycle = _cycle(clock)
        results = run_daemon(
            cycle=cycle, max_cycles=2, clock=clock.time, sleep=clock.sleep
        )
        self.assertEqual(len(results), 2)
        self.assertEqual(cycle.calls(), 2)

    def test_sleeps_until_next_week_boundary(self):
        clock = FakeClock()
        cycle = _cycle(clock)
        run_daemon(cycle=cycle, max_cycles=2, clock=clock.time, sleep=clock.sleep)
        # 一轮之间只睡一次，且补足一整周（该轮耗时为 0）。
        self.assertEqual(clock.sleeps, [WEEK_SECONDS])

    def test_cycle_duration_does_not_drift_cadence(self):
        """周节奏是从「上一轮开始」算的，抓取耗时不能把周期推长。"""
        clock = FakeClock(cycle_cost=3600.0)
        cycle = _cycle(clock)
        run_daemon(cycle=cycle, max_cycles=2, clock=clock.time, sleep=clock.sleep)
        self.assertEqual(
            clock.sleeps, [WEEK_SECONDS - 3600.0], "应扣掉本轮耗时，保持周期不漂移"
        )

    def test_overlong_cycle_does_not_sleep_negative(self):
        clock = FakeClock(cycle_cost=WEEK_SECONDS * 2)
        cycle = _cycle(clock)
        run_daemon(cycle=cycle, max_cycles=2, clock=clock.time, sleep=clock.sleep)
        self.assertEqual(clock.sleeps, [0.0], "本轮超时就不再等，立即开始下一轮")

    def test_custom_interval_is_honoured(self):
        clock = FakeClock()
        cycle = _cycle(clock)
        run_daemon(
            cycle=cycle,
            interval_seconds=60.0,
            max_cycles=2,
            clock=clock.time,
            sleep=clock.sleep,
        )
        self.assertEqual(clock.sleeps, [60.0])

    def test_negative_interval_is_rejected(self):
        with self.assertRaises(ValueError):
            run_daemon(
                cycle=lambda: (0, None),
                interval_seconds=-1.0,
                max_cycles=1,
                clock=FakeClock().time,
                sleep=FakeClock().sleep,
            )


class FailureIsolationTest(unittest.TestCase):
    def test_failed_cycle_does_not_kill_daemon(self):
        """周更任务挂一次就永久停摆是不可接受的——必须记错并继续。"""
        clock = FakeClock()
        cycle = _cycle(clock, fail_times=1)
        results = run_daemon(
            cycle=cycle, max_cycles=3, clock=clock.time, sleep=clock.sleep
        )
        self.assertEqual(len(results), 3, "三轮都应有结果记录")
        self.assertIsNotNone(results[0].error)
        self.assertIsNone(results[1].error)
        self.assertIsNone(results[2].error)
        self.assertEqual(cycle.calls(), 3, "失败后仍要继续跑后续轮次")

    def test_failure_still_waits_full_cadence(self):
        clock = FakeClock()
        cycle = _cycle(clock, fail_times=1)
        run_daemon(cycle=cycle, max_cycles=2, clock=clock.time, sleep=clock.sleep)
        self.assertEqual(clock.sleeps, [WEEK_SECONDS], "失败轮也要按周期等待")

    def test_error_is_recorded_with_message(self):
        clock = FakeClock()

        def boom():
            raise RuntimeError("抓取挂了")

        results = run_daemon(
            cycle=boom, max_cycles=1, clock=clock.time, sleep=clock.sleep
        )
        self.assertIn("抓取挂了", str(results[0].error))


class CycleResultTest(unittest.TestCase):
    def test_successful_result_carries_count_and_report(self):
        result = CycleResult(stored=5, report="报告", error=None)
        self.assertEqual(result.stored, 5)
        self.assertEqual(result.report, "报告")
        self.assertIsNone(result.error)
        self.assertFalse(result.failed)

    def test_failed_result_is_marked(self):
        result = CycleResult(stored=0, report=None, error="boom")
        self.assertTrue(result.failed)

    def test_render_is_loggable(self):
        self.assertIn("5", CycleResult(stored=5, report=None, error=None).render())
        self.assertIn("boom", CycleResult(stored=0, report=None, error="boom").render())


class ListUrlValidationTest(unittest.TestCase):
    """常驻服务启动前必须校验发现入口——否则会变成「每周期空转失败」的假健康服务。

    实测过这个坑：`COLLECTOR_LIST_URL` 未设置时 compose 传空串，daemon 会带着空 URL
    起来，每轮 robots 拒绝一次、记一条失败，然后永远跑下去。
    """

    def test_empty_list_urls_rejected_before_any_cycle(self):
        clock = FakeClock()
        called = {"n": 0}

        def cycle():
            called["n"] += 1
            return 0, None

        with self.assertRaises(ValueError):
            run_daemon(
                cycle=cycle,
                list_urls=[],
                clock=clock.time,
                sleep=clock.sleep,
            )
        self.assertEqual(called["n"], 0, "校验失败时不得开始第一轮")

    def test_blank_list_url_rejected(self):
        clock = FakeClock()
        with self.assertRaises(ValueError):
            run_daemon(
                cycle=lambda: (0, None),
                list_urls=["   "],
                clock=clock.time,
                sleep=clock.sleep,
            )

    def test_non_http_list_url_rejected(self):
        clock = FakeClock()
        with self.assertRaises(ValueError):
            run_daemon(
                cycle=lambda: (0, None),
                list_urls=["www1.rmfysszc.gov.cn/Agency/pmgg.aspx"],
                clock=clock.time,
                sleep=clock.sleep,
            )

    def test_url_without_host_rejected(self):
        """`https://` 有 scheme 无 host —— robots 层会 fail-closed，但那时已经常驻了。"""
        clock = FakeClock()
        with self.assertRaises(ValueError):
            run_daemon(
                cycle=lambda: (0, None),
                list_urls=["https://"],
                clock=clock.time,
                sleep=clock.sleep,
            )

    def test_valid_list_url_passes_validation(self):
        clock = FakeClock()
        results = run_daemon(
            cycle=lambda: (1, None),
            list_urls=["https://www1.rmfysszc.gov.cn/Agency/pmgg.aspx?fid=100"],
            max_cycles=1,
            clock=clock.time,
            sleep=clock.sleep,
        )
        self.assertEqual(len(results), 1)

    def test_validation_is_skipped_when_no_list_urls_given(self):
        """不传 list_urls（None）表示「调用方自己管发现入口」，不做校验。"""
        clock = FakeClock()
        results = run_daemon(
            cycle=lambda: (1, None),
            max_cycles=1,
            clock=clock.time,
            sleep=clock.sleep,
        )
        self.assertEqual(len(results), 1)


class StartupValidationTest(unittest.TestCase):
    """启动前把「必然每轮回失败」的配置错查掉。

    常驻服务 + `restart: unless-stopped` 的组合下，「每轮都失败但进程活着」是最难
    发现的故障形态：容器 healthy、日志有输出、一周只写一条失败。启动即退出至少能让
    Docker 重启策略和 `docker compose ps` 把它暴露出来。
    """

    def test_missing_dsn_rejected(self):
        from collector.daemon import validate_startup

        with self.assertRaises(ValueError) as ctx:
            validate_startup(
                list_urls=["https://www1.rmfysszc.gov.cn/Agency/pmgg.aspx?fid=100"],
                dsn_fn=lambda: "",
            )
        self.assertIn("DATABASE_URL", str(ctx.exception))

    def test_probe_failure_propagates(self):
        """AI 配置未绑定 / SECRET_KEY 错这类问题必须启动时就暴露。"""
        from collector.daemon import validate_startup

        def boom():
            raise RuntimeError("功能 market_sample_extract 未绑定有效 AI 配置")

        with self.assertRaises(RuntimeError):
            validate_startup(
                list_urls=["https://x.test/list"],
                dsn_fn=lambda: "postgres://x/y",
                probe=boom,
            )

    def test_valid_config_passes(self):
        from collector.daemon import validate_startup

        called = {"probe": 0}

        validate_startup(
            list_urls=["https://x.test/list"],
            dsn_fn=lambda: "postgres://x/y",
            probe=lambda: called.__setitem__("probe", called["probe"] + 1),
        )
        self.assertEqual(called["probe"], 1, "配置齐备时应真的跑一次 probe")


if __name__ == "__main__":
    unittest.main()
