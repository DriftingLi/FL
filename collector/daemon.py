"""增量 daemon 的节奏循环（#2678）。

第一性原理：spec #2671 user story 20 要求「历史回补与增量采集是同一组件的两种模式，
so that 清洗与匹配逻辑共用一份、不漂移」。所以 daemon **不是**第二条管线——它就是
「把回补那一轮按周重复」。本模块只负责**节奏**与**失败隔离**，采集/抽取/清洗/匹配
一行都不复制（由调用方传入的 `cycle` 走既有 `run_backfill`）。

两条设计约束来自「周更任务」这个场景本身：
  - **周节奏不漂移**：下一轮从「本轮开始」算满一周，而不是「本轮结束」再等一周，
    否则抓取耗时（千条量级≈1 小时）会逐轮累积推后。
  - **失败不杀 daemon**：周更任务挂一次就永久停摆是不可接受的。单轮异常记进
    `CycleResult.error`，照常等下一周期。连续失败也不额外熔断——限速与退避已经
    在采集层处理了「站点不健康」，这里再加熔断只会制造静默停摆。
"""

from __future__ import annotations

import sys
import time
from dataclasses import dataclass
from typing import Callable
from urllib.parse import urlparse

WEEK_SECONDS = 7 * 24 * 3600

# 一轮的产出：落库行数 + 得率报告（失败时 report 为 None）。
Cycle = Callable[[], tuple[int, object]]


@dataclass(frozen=True)
class CycleResult:
    """一轮增量采集的结果。失败轮也留一条，便于日志审计「哪一轮挂了」。

    `failed_items` 是**轮内部分失败**（某几条公告抓不到，但整轮没抛错）——
    与 `error`（整轮失败）是两件事：一轮可以既落库若干行、又有几条抓取失败。
    """

    stored: int
    report: object
    error: BaseException | None
    failed_items: int = 0

    @property
    def failed(self) -> bool:
        return self.error is not None

    def render(self) -> str:
        if self.failed:
            head = f"本轮失败：{self.error}（不终止 daemon，按周期重试）"
            if self.stored:
                head += f"；已落库 {self.stored} 行"
            return head
        head = f"本轮落库 {self.stored} 行"
        if self.failed_items:
            head += f"，{self.failed_items} 条公告抓取失败（不阻塞后续）"
        if self.report is None:
            return head
        # 优先用报告自己的 render()（有得率/剔除原因分节、URL 有上限）；
        # 直接 f-string 会把整份 dataclass repr 打出来，日志既更长又更少信息。
        render = getattr(self.report, "render", None)
        body = render() if callable(render) else str(self.report)
        return f"{head}\n{body}"


def _validate_list_urls(list_urls: list[str] | None) -> None:
    """常驻服务启动前校验发现入口（fail-loud）。

    实测过的坑：`COLLECTOR_LIST_URL` 未设置时 compose 传空串，daemon 会带着空 URL
    起来，每轮被 robots 拒绝一次、记一条失败，然后**永远跑下去**——日志里只有周期性的
    「本轮失败」，外部看不出这是个配错的常驻服务。宁可启动时直接报错退出。

    注意：真正的守卫是这里，不是 argparse——空串是合法的参数值，argparse 拦不住。
    """
    if list_urls is None:
        return  # 调用方自己管发现入口（如测试注入的 cycle），不校验
    if not list_urls:
        raise ValueError("daemon 需要至少一个 --list-url（发现入口不能为空）")
    for url in list_urls:
        if not url or not url.strip():
            raise ValueError(
                "daemon 的 --list-url 不能为空：COLLECTOR_LIST_URL 未设置时 "
                "compose 会传空串，服务会变成「每周期空转失败」的假健康服务"
            )
        parsed = urlparse(url.strip())
        # 只查「有没有可用的 host」——真正的站点级合规判定在 crawl 层
        # （robots.py 按 netloc 绑定，未知 host fail-closed），这里不重复它的逻辑。
        if parsed.scheme not in ("http", "https") or not parsed.netloc:
            raise ValueError(
                f"daemon 的 --list-url 必须带 host 的 http(s) URL，收到：{url!r}"
            )


def validate_startup(
    *,
    list_urls: list[str] | None,
    dsn_fn: Callable[[], str] = lambda: "",
    probe: Callable[[], None] | None = None,
) -> None:
    """启动前把「必然每轮回失败」的配置错一次性查掉，别让服务跑到第一轮才炸。

    为什么要在启动时查：常驻服务 + `restart: unless-stopped` 的组合下，「每轮都失败
    但进程活着」是最难发现的故障形态——容器 healthy、日志有输出、却一周只写一条失败。
    启动即退出至少能让 Docker 的重启策略和 `docker compose ps` 把问题暴露出来。

    `probe` 由调用方提供（连一次 PG、解析一次 AI 配置），因为本模块不认识 PG/AI 配置。
    """
    _validate_list_urls(list_urls)
    if not dsn_fn():
        raise ValueError(
            "缺少 DATABASE_URL（或 --dsn）：daemon 没有数据库就没法落库，"
            "不该带着这个配置常驻"
        )
    if probe is not None:
        probe()


def run_daemon(
    *,
    cycle: Cycle,
    interval_seconds: float = WEEK_SECONDS,
    max_cycles: int | None = None,
    list_urls: list[str] | None = None,
    clock: Callable[[], float] = time.monotonic,
    sleep: Callable[[float], None] = time.sleep,
    on_result: Callable[[CycleResult], None] | None = None,
) -> list[CycleResult]:
    """按固定节奏重复跑 `cycle`，返回每轮结果（`max_cycles=None` = 永不停止）。

    `list_urls` 给定时先校验发现入口（空/非 URL 直接抛错，不留空转服务）；
    `clock` / `sleep` 可注入：测试用假时钟推进虚拟时间，不真的等一周。
    """
    if interval_seconds < 0:
        raise ValueError("interval_seconds 不得为负")
    if max_cycles is not None and max_cycles < 0:
        raise ValueError("max_cycles 不得为负")
    _validate_list_urls(list_urls)

    results: list[CycleResult] = []
    completed = 0
    while max_cycles is None or completed < max_cycles:
        started = clock()
        try:
            stored, report = cycle()
            result = CycleResult(
                stored=stored,
                report=report,
                error=None,
                # 轮内部分失败（毒丸隔离后不再抛错，改由报告携带）。
                failed_items=getattr(report, "failed_items", 0) if report is not None else 0,
            )
        except Exception as exc:  # 失败隔离：任何单轮异常都不许终止 daemon
            result = CycleResult(stored=0, report=None, error=exc)
        results.append(result)
        completed += 1
        # on_result 也在隔离范围内：日志管道断了（如 docker compose logs 关掉 stdout）
        # 不该杀掉常驻服务。它只是可观测性，不是业务步骤。
        try:
            if on_result is not None:
                on_result(result)
        except Exception as exc:  # noqa: BLE001 —— 可观测性失败不得终止服务
            print(f"[daemon] on_result 回调失败（已忽略）：{exc}", file=sys.stderr)

        # 最后一轮之后不再等待（max_cycles 到点就该返回）。
        if max_cycles is not None and completed >= max_cycles:
            break
        # 周节奏从「本轮开始」算：扣掉本轮耗时，超时则立即开始下一轮。
        elapsed = clock() - started
        sleep(max(0.0, interval_seconds - elapsed))
    return results
