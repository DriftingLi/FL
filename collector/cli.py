"""collector CLI：fixture 进 → 抽取 → 清洗 → 匹配 → market_transactions 行 + 抽检队列出。

三段分离（spec #2671）：采集层只 fetch + 原始留底，抽取 / 清洗 / 匹配是独立的纯函数
阶段，入库只做写。清洗与匹配不依赖抓取，故可重跑：

    python3 -m collector.cli batch <fixture.html> [--dsn URL] [--search-path SCHEMA]
    python3 -m collector.cli reclean   [--dsn URL] [--search-path SCHEMA]
    python3 -m collector.cli rematch   [--dsn URL] [--search-path SCHEMA]
    python3 -m collector.cli unmatched [--dsn URL] [--search-path SCHEMA]
    python3 -m collector.cli backfill  --list-url <列表页> [...]      # #2677 历史回补
    python3 -m collector.cli daemon    --list-url <列表页> [...]      # #2678 周更增量

`backfill`（一次性）与 `daemon`（常驻周更）是**同一组件的两种模式**：daemon 每一轮
直接复用 backfill，清洗/匹配只有一份实现（spec user story 20）。

`--dsn` 缺省取环境变量 DATABASE_URL，`SECRET_KEY` 用于解密 ai_configs.api_key
（均与后端同源）。`--search-path` 用于契约测试把整条链路收窄在随机 schema。
抽取 / 清洗 / 匹配均可注入 seam：测试用录制 fixture + 纯函数打桩（确定性、不测 prompt 质量）。
"""

from __future__ import annotations

import argparse
import os
import sys
import time
import unicodedata
from pathlib import Path
from typing import Callable

from .backfill import (
    BACKFILL_END_YEAR,
    BACKFILL_START_YEAR,
    DiscoveredAnnouncement,
    build_yield_report,
    daemon_end_year,
    filter_by_year,
    select_new_announcements,
)
from .backfill import BASE as BASE_URL
from .clean import CLEAN_REVIEW_REASONS, clean
from .daemon import WEEK_SECONDS, run_daemon, validate_startup
from .extract.config import FEATURE_KEY, resolve_config
from .extract.llm import extract
from .extract.service import apply_extraction, apply_structured
from .match import match
from .sources import Fetch, SourceSpec, resolve_source
from .store import (
    enqueue_review,
    fetch_samples,
    insert_sample,
    load_price_candidates,
    load_unmatched_queue,
    update_clean_result,
    update_match_result,
)


def _process_samples(
    conn,
    samples: list,
    *,
    spec: SourceSpec,
    source_url: str = "",
    config,
    candidates: list[dict],
    extractor: Callable | None,
    clean_fn: Callable,
    match_fn: Callable,
    existing_samples: list,
) -> tuple[int, list]:
    """把已解析出的 raw 样本走完 抽取/直取 → 清洗 → 匹配 → 落库。

    这是 batch 与 backfill 共用的**唯一**管线（ADR-0101 决策四：回补不另写一份）。
    抽取阶段按采集面的 `structured` 分流：
      - `structured=False`（rmfysszc 公告正文）⇒ LLM 抽取 + `apply_extraction`；
      - `structured=True`（京东 JSONP / 公拍网与产权交易所表格）⇒ 字段已是来源事实，
        跳过 LLM，只做判定与盖版本（`apply_structured`）。
    分流点只有这一个，清洗与匹配对两类采集面**完全同一份代码**——否则「结构化源
    不走清洗」这种漂移迟早发生。
    """
    if not samples:
        return 0, []
    # 去重信号 = 已入库样本 + 本批已落样本；两者都由调用方跨 URL 累积传入。
    existing = list(existing_samples)
    added: list = []
    for sample in samples:
        if spec.structured:
            updated, review = apply_structured(sample)
        else:
            extraction = extract(sample.raw_description["body"], config, extractor)
            updated, review = apply_extraction(sample, extraction)
        # 清洗是独立纯函数阶段：只吃已抽取样本 + 既有样本（去重信号）。
        cleaned, clean_review = clean_fn(updated, existing=existing)
        # 匹配是独立纯函数阶段：从 PG 直读候选原价，取中位数、不取最高价。
        matched = match_fn(cleaned, candidates)
        sample_id = insert_sample(conn, matched)
        for reason, detail in review + clean_review:
            enqueue_review(conn, sample_id, reason, detail)
        existing.append(matched)
        added.append(matched)
    return len(samples), added


def _process_url(
    conn,
    url: str,
    *,
    spec: SourceSpec,
    fetch: Fetch,
    config,
    candidates: list[dict],
    extractor: Callable | None,
    clean_fn: Callable,
    match_fn: Callable,
    existing_samples: list,
) -> tuple[int, list]:
    """抓一个详情 URL（或其 JSONP 接口）→ 走完整条管线（调用方负责连接与事务）。

    `spec.fetch_detail` 是采集面唯一的「取样本」入口：HTML 详情页的适配器用
    `html_detail` 包一层（抓一次页面），京东这种需要打多个 JSONP 接口的自定义实现。
    所以这里不关心是 HTML 还是 JSON——那是适配器的事。

    `fetch` 必须是过了 robots + 限速 + 退避的抓取函数（生产是 `CrawlClient.fetch`）。
    适配器内部的所有请求都走它，采集面无法绕过合规三道闸。
    """
    samples = spec.fetch_detail(url, fetch)
    return _process_samples(
        conn,
        samples,
        spec=spec,
        config=config,
        candidates=candidates,
        extractor=extractor,
        clean_fn=clean_fn,
        match_fn=match_fn,
        existing_samples=existing_samples,
    )


def _process_html(
    conn,
    html: str,
    *,
    source_url: str = "",
    spec: SourceSpec | None = None,
    config,
    candidates: list[dict],
    extractor: Callable | None,
    clean_fn: Callable,
    match_fn: Callable,
    existing_samples: list,
) -> tuple[int, list]:
    """把一份**已在内存里的**详情页 HTML 走完整条管线（batch / fixture 路径）。

    `spec` 缺省按 `source_url` 识别；识别不出则按 rmfysszc 处理（batch 的 fixture
    就是 rmfysszc 公告，且 source_url 常为空）。生产回补不走这里——它走
    `_process_url`，让适配器自己决定抓什么。
    """
    if spec is None:
        spec = resolve_source(source_url) if source_url else _default_spec()
    samples = spec.fetch_detail(source_url, lambda _url: html)
    return _process_samples(
        conn,
        samples,
        spec=spec,
        config=config,
        candidates=candidates,
        extractor=extractor,
        clean_fn=clean_fn,
        match_fn=match_fn,
        existing_samples=existing_samples,
    )


def _default_spec() -> SourceSpec:
    """batch 无 URL 时的缺省采集面：rmfysszc（tracer bullet 的 fixture 就是它）。"""
    from .models import CHANNEL_RMFYSSZC
    from .sources import source_for_channel

    return source_for_channel(CHANNEL_RMFYSSZC)


def run_batch(
    fixture_path: str,
    dsn: str | None = None,
    search_path: str = "",
    *,
    extractor: Callable | None = None,
    secret_key: str | None = None,
    cleaner: Callable | None = None,
    matcher: Callable | None = None,
    price_candidates: list[dict] | None = None,
) -> int:
    """处理一份 fixture：解析 → 配置 → 抽取 → 清洗 → 匹配 → 写回同一行 + 抽检入队。

    返回落库行数。extractor / cleaner / matcher 为可注入 seam：测试用录制 fixture
    打桩；生产不传则走真实 LLM + 版本化清洗规则 + 校准专用分层匹配器。
    price_candidates 可注入以避免测试里真建 original_prices 表。
    """
    import psycopg
    from psycopg import sql

    dsn = dsn or os.environ.get("DATABASE_URL") or ""
    if not dsn:
        raise RuntimeError("缺少 DATABASE_URL（或 --dsn）")

    html = Path(fixture_path).read_text(encoding="utf-8")
    # batch 子命令只服务 rmfysszc 公告正文 fixture（结构化采集面走 backfill/daemon，
    # 它们的样本来自接口或表格，没有「一份公告 HTML」这种 fixture 形态）。
    spec = _default_spec()
    samples = spec.fetch_detail("", lambda _url: html)
    if not samples:
        return 0

    secret_key = secret_key if secret_key is not None else os.environ.get("SECRET_KEY", "")
    clean_fn = cleaner or clean
    match_fn = matcher or match

    with psycopg.connect(dsn) as conn:
        if search_path:
            conn.execute(
                sql.SQL("SET search_path TO {}").format(sql.Identifier(search_path))
            )
        # 配置直读与写回同在一个连接 / 事务：要么整批样本+队列行都在，要么都不在。
        config = resolve_config(conn, FEATURE_KEY, secret_key)
        candidates = (
            price_candidates if price_candidates is not None
            else load_price_candidates(conn)
        )
        with conn.transaction():
            count, _ = _process_html(
                conn,
                html,
                spec=spec,
                config=config,
                candidates=candidates,
                extractor=extractor,
                clean_fn=clean_fn,
                match_fn=match_fn,
                existing_samples=[],
            )
            return count


def run_reclean(
    dsn: str | None = None,
    search_path: str = "",
    *,
    cleaner: Callable | None = None,
) -> int:
    """重跑清洗：从 PG 读回已入库行，按当前规则版本重算清洗列，不重爬公告。

    重跑只动 clean_status/rejection_reason/premium_anomaly/label_eligible/结构字段
    与原文留底（身份剥离），抽取版本 / 匹配 / 标签列不动。清洗产生的 review 行先按
    CLEAN_REVIEW_REASONS 清除再重入，保证幂等。返回重算的行数。
    """
    import psycopg
    from psycopg import sql

    dsn = dsn or os.environ.get("DATABASE_URL") or ""
    if not dsn:
        raise RuntimeError("缺少 DATABASE_URL（或 --dsn）")

    clean_fn = cleaner or clean
    with psycopg.connect(dsn) as conn:
        if search_path:
            conn.execute(
                sql.SQL("SET search_path TO {}").format(sql.Identifier(search_path))
            )
        rows = fetch_samples(conn)
        # 去重信号用读回的全量样本（同一 schema 内的已入库行）。
        all_samples = [sample for _, sample in rows]
        with conn.transaction():
            for sample_id, sample in rows:
                cleaned, review = clean_fn(sample, existing=all_samples)
                conn.execute(
                    "DELETE FROM market_sample_review_queue "
                    "WHERE sample_id = %s AND reason = ANY(%s)",
                    (sample_id, list(CLEAN_REVIEW_REASONS)),
                )
                update_clean_result(conn, sample_id, cleaned)
                for reason, detail in review:
                    enqueue_review(conn, sample_id, reason, detail)
    return len(rows)


def run_rematch(
    dsn: str | None = None,
    search_path: str = "",
    *,
    matcher: Callable | None = None,
    price_candidates: list[dict] | None = None,
) -> int:
    """补原价后重跑匹配：读回已入库行，重算匹配/标签列，不重爬、不动抽取/清洗。

    未匹配样本 label_eligible 保留、residual_rate_label 留空，补入原价后重跑可回收标签。
    返回重算行数。
    """
    import psycopg
    from psycopg import sql

    dsn = dsn or os.environ.get("DATABASE_URL") or ""
    if not dsn:
        raise RuntimeError("缺少 DATABASE_URL（或 --dsn）")

    match_fn = matcher or match
    with psycopg.connect(dsn) as conn:
        if search_path:
            conn.execute(
                sql.SQL("SET search_path TO {}").format(sql.Identifier(search_path))
            )
        candidates = (
            price_candidates if price_candidates is not None
            else load_price_candidates(conn)
        )
        rows = fetch_samples(conn)
        with conn.transaction():
            for sample_id, sample in rows:
                matched = match_fn(sample, candidates)
                update_match_result(conn, sample_id, matched)
    return len(rows)


def run_unmatched(dsn: str | None = None, search_path: str = "") -> list[dict]:
    """查未匹配待审队列（迁移 000067 的聚合视图），返回聚合组列表（#2676）。

    只读：人工据聚合结果补原价条目，补入后 `collector rematch` 重跑回收标签。
    """
    import psycopg
    from psycopg import sql

    dsn = dsn or os.environ.get("DATABASE_URL") or ""
    if not dsn:
        raise RuntimeError("缺少 DATABASE_URL（或 --dsn）")

    with psycopg.connect(dsn) as conn:
        if search_path:
            conn.execute(
                sql.SQL("SET search_path TO {}").format(sql.Identifier(search_path))
            )
        return load_unmatched_queue(conn)


def run_backfill(
    list_urls: list[str],
    dsn: str | None = None,
    search_path: str = "",
    *,
    start_year: int = BACKFILL_START_YEAR,
    end_year: int = BACKFILL_END_YEAR,
    crawler=None,
    extractor: Callable | None = None,
    secret_key: str | None = None,
    cleaner: Callable | None = None,
    matcher: Callable | None = None,
    price_candidates: list[dict] | None = None,
) -> tuple[int, object]:
    """历史回补：发现 → 过滤年份 → 跳过已入库 → 逐个抓详情 → 落库 → 得率报告。

    一条命令完成（判据 1）：`collector backfill --list-url <URL> [--list-url ...]`。
    抓取过 robots + 限速 + 退避（判据 2，`crawler` 可注入以便测试不碰网络）。
    样本走既有全管线（判据 3：`_process_html` 与 batch 共用同一段）。
    已入库 URL 跳过（判据 5：`select_new_announcements` + `existing_urls`）。

    返回 (落库行数, YieldReport)。
    """
    import psycopg
    from psycopg import sql

    dsn = dsn or os.environ.get("DATABASE_URL") or ""
    if not dsn:
        raise RuntimeError("缺少 DATABASE_URL（或 --dsn）")
    if crawler is None:
        raise RuntimeError("backfill 需要 crawler（CLI 会构造真实抓取客户端）")

    secret_key = secret_key if secret_key is not None else os.environ.get("SECRET_KEY", "")
    clean_fn = cleaner or clean
    match_fn = matcher or match

    # ① 发现：抓列表页 → 解析 → 年份过滤。采集面按 list_url 识别（多源可混在同一轮），
    # 识别不出即报错——不能把别的平台列表页当 rmfysszc 解析（会静默产出 0 条）。
    discovered: list[DiscoveredAnnouncement] = []
    kept_undated: list[DiscoveredAnnouncement] = []
    for list_url in list_urls:
        spec = resolve_source(list_url)
        page = crawler.fetch(list_url)
        items = spec.parse_listing(page, list_url)
        if spec.keep_undated:
            # 结构化采集面（京东/公拍网/产权交易所）没有发布日期字段：年份窗口对它们
            # 不适用，全部放行。否则「按年份过滤」会把它们的样本 100% 丢光，
            # 表现为「回补成功但 0 条」——静默失败，最难查。
            kept_undated.extend(items)
        else:
            discovered.extend(items)
    in_window = filter_by_year(discovered, start_year=start_year, end_year=end_year)
    in_window.extend(kept_undated)

    with psycopg.connect(dsn) as conn:
        if search_path:
            conn.execute(
                sql.SQL("SET search_path TO {}").format(sql.Identifier(search_path))
            )
        config = resolve_config(conn, FEATURE_KEY, secret_key)
        candidates = (
            price_candidates if price_candidates is not None
            else load_price_candidates(conn)
        )
        # ② 幂等：已入库 URL 跳过（重跑不重复落行）。
        existing_urls = _existing_source_urls(conn)
        new_items = select_new_announcements(in_window, existing_urls=existing_urls)
        # 上面的只读 SELECT 在 psycopg（autocommit=False）里已隐式开启一个事务。
        # 必须先提交结束它，否则下面每条 conn.transaction() 只是在已有事务里开
        # savepoint，单条失败会让外层 connect 上下文 rollback 掉整轮已落的行。
        conn.commit()

        # ③ 逐条回补。网络抓取在事务外；每条一个独立小事务——千条回补按 ≤1 req/2s
        # 要跑约 1 小时，不能全程开着同一个事务（idle-in-transaction / 锁）；且单条
        # 失败不该把前面已落的行全回滚。
        #
        # **毒丸隔离**：单条 URL 抓取/落库失败只记进 failed_urls 并继续下一条。
        # 常驻 daemon 尤其需要——一条永久 404 若能让整轮抛错，它就会永远卡在队头，
        # 后面所有新公告每周都抓不到（实测过：8 周后只落 1 行）。
        stored = 0
        stored_samples: list = []
        yield_rows: list[dict] = []
        failed_urls: list[str] = []
        for item in new_items:
            try:
                # 网络 I/O 全部在事务外；适配器自己决定抓一次 HTML 还是多次 JSONP。
                spec = resolve_source(item.url)
                with conn.transaction():
                    count, added = _process_url(
                        conn,
                        item.url,
                        spec=spec,
                        fetch=crawler.fetch,
                        config=config,
                        candidates=candidates,
                        extractor=extractor,
                        clean_fn=clean_fn,
                        match_fn=match_fn,
                        # 跨 URL 去重信号：同轮前面所有 URL 的落库样本（车商批量帖去重⑥）。
                        existing_samples=stored_samples,
                    )
            except Exception as exc:  # 单条隔离：坏一条不影响其余
                print(f"[backfill] 跳过抓取失败的公告 {item.url}: {exc}", file=sys.stderr)
                failed_urls.append(item.url)
                continue
            stored += count
            stored_samples.extend(added)
            yield_rows.extend(
                {"clean_status": s.clean_status, "rejection_reason": s.rejection_reason}
                for s in added
            )
        report = build_yield_report(yield_rows, failed_urls=failed_urls)
    return stored, report


def _existing_source_urls(conn) -> set[str]:
    """已入库的 source_url 集合（幂等判据的数据来源）。"""
    rows = conn.execute("SELECT DISTINCT source_url FROM market_transactions").fetchall()
    return {row[0] for row in rows}


def run_daemon_command(
    list_urls: list[str],
    dsn: str | None = None,
    search_path: str = "",
    *,
    interval_seconds: float = WEEK_SECONDS,
    max_cycles: int | None = None,
    start_year: int = BACKFILL_START_YEAR,
    end_year: int | None = None,
    crawler=None,
    extractor: Callable | None = None,
    secret_key: str | None = None,
    cleaner: Callable | None = None,
    matcher: Callable | None = None,
    price_candidates: list[dict] | None = None,
    clock=None,
    sleep=None,
    on_result: Callable | None = None,
) -> list:
    """增量 daemon：按固定节奏重复跑「一轮增量采集」，单轮逻辑就是 run_backfill。

    spec #2671 要求同一组件两种模式、清洗/匹配共用一份——所以 daemon 的每一轮
    **直接复用 run_backfill**（发现→年份过滤→幂等跳过→全管线→得率报告），
    本函数只加节奏与失败隔离，不复制任何采集/抽取/清洗/匹配逻辑。

    周更场景下年份窗口的**上界默认跟随当前年**（`daemon_end_year()`）——写死 2026 会让
    daemon 从 2027-01-01 起每轮「成功但发现 0 条」，日志全绿、库不再增长、无任何报错。
    """

    # end_year 缺省 = 当前年（不是写死的 BACKFILL_END_YEAR，见 daemon_end_year 的说明）。
    resolved_end_year = daemon_end_year() if end_year is None else end_year

    def one_cycle() -> tuple[int, object]:
        return run_backfill(
            list_urls,
            dsn,
            search_path,
            start_year=start_year,
            end_year=resolved_end_year,
            crawler=crawler,
            extractor=extractor,
            secret_key=secret_key,
            cleaner=cleaner,
            matcher=matcher,
            price_candidates=price_candidates,
        )

    return run_daemon(
        cycle=one_cycle,
        interval_seconds=interval_seconds,
        max_cycles=max_cycles,
        # 启动前校验发现入口：空 COLLECTOR_LIST_URL 不许起一个「每周期空转失败」的服务。
        list_urls=list_urls,
        # clock/sleep 缺省用真时钟真睡眠（这里显式取真值，避免传 None 冲掉默认参数）。
        clock=clock if clock is not None else time.monotonic,
        sleep=sleep if sleep is not None else time.sleep,
        on_result=on_result,
    )


def _print_unmatched(groups: list[dict]) -> None:
    """把待审队列聚合组打印成对齐表格（品牌/型号/吨位/样本数/价格区间）。

    表头与 `UNMATCHED_GROUP_COLUMNS` 同序（品牌/型号/吨位/样本数/价格区间），
    避免列名契约与展示顺序两处手抄漂移。宽度按显示宽度算（CJK 占 2 列）。
    """
    if not groups:
        print("未匹配待审队列：空（无待审样本）")
        return
    header = ("品牌", "型号", "吨位", "样本数", "成交价区间")
    rows = [
        (
            str(g.get("brand") or "-"),
            str(g.get("model") or "-"),
            "-" if g.get("tonnage") is None else str(g.get("tonnage")),
            str(g.get("sample_count")),
            _price_range_text(g.get("price_min"), g.get("price_max")),
        )
        for g in groups
    ]
    widths = [
        max(_display_width(header[i]), *(_display_width(r[i]) for r in rows))
        for i in range(len(header))
    ]
    print("  ".join(_pad(h, widths[i]) for i, h in enumerate(header)))
    for row in rows:
        print("  ".join(_pad(c, widths[i]) for i, c in enumerate(row)))
    print(f"\n共 {len(groups)} 组、{sum(g['sample_count'] for g in groups)} 条待审样本")


def _display_width(text: str) -> int:
    """终端显示宽度：CJK 全角字符占 2 列，其余占 1 列。"""
    return sum(2 if unicodedata.east_asian_width(ch) in ("W", "F") else 1 for ch in text)


def _pad(text: str, width: int) -> str:
    return text + " " * max(0, width - _display_width(text))


def _price_range_text(low, high) -> str:
    if low is None and high is None:
        return "-"
    if low == high:
        return f"{float(low):.2f}"
    return f"{float(low):.2f} ~ {float(high):.2f}"


def _add_common_args(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("--dsn", default=None, help="PG 连接串（缺省取 DATABASE_URL）")
    parser.add_argument(
        "--search-path", default="", help="可选：把本次操作收窄到指定 schema（契约测试用）"
    )


def _crawl_origins(list_urls: list[str] | None = None) -> tuple[str, ...]:
    """本轮要核验 robots 的 origin 集合：缺省采集面 + 每个 list_url 所属采集面的 origins。

    为什么要按 list_url 收集而不是写死一份：robots 按 host 生效，多 host 采集面
    （京东三 host、公拍网三 host）如果只读一个 origin，其余 host 会被 RobotsPolicy
    判成「未核验 ⇒ 禁止」，采集面直接全废。反过来把所有已知 origin 都读一遍又
    会在无关站点上多发请求。按「本轮真正要抓的采集面」收集最准。
    """
    origins: list[str] = list(_default_spec().origins)
    for list_url in list_urls or ():
        try:
            spec = resolve_source(list_url)
        except ValueError:
            # 识别不出就不猜它的 origins：抓取时 resolve_source 会再报一次（带 URL）。
            continue
        for origin in spec.origins:
            if origin not in origins:
                origins.append(origin)
    return tuple(origins)


def _build_crawler(*, allow_robots_override: bool, list_urls: list[str] | None = None):
    """构造生产抓取客户端：按 origin 各读 robots → 遵从判定 + 限速 + 退避。

    多 origin 用 `load_robots_book`（每 host 一份策略、各自 fail-closed），
    单 origin 退化成原来的单策略路径——两种形态都走同一个 CrawlClient。
    """
    from .crawl.client import (
        CrawlClient,
        load_robots,
        load_robots_book,
        urllib_fetcher,
    )

    fetcher = urllib_fetcher()
    origins = _crawl_origins(list_urls)
    if len(origins) == 1:
        robots = load_robots(base_url=origins[0], fetcher=fetcher)
    else:
        robots = load_robots_book(origins=origins, fetcher=fetcher)
    return CrawlClient(
        robots=robots,
        fetcher=fetcher,
        allow_robots_override=allow_robots_override,
    )


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="collector")
    sub = parser.add_subparsers(dest="command", required=True)

    batch = sub.add_parser("batch", help="从单份 fixture 公告落 raw 样本行")
    batch.add_argument("fixture", help="fixture HTML 路径")
    _add_common_args(batch)

    reclean = sub.add_parser("reclean", help="重跑清洗（读回已入库行，不重爬）")
    _add_common_args(reclean)

    rematch = sub.add_parser("rematch", help="补原价后重跑匹配（读回已入库行，不重爬）")
    _add_common_args(rematch)

    unmatched = sub.add_parser(
        "unmatched", help="查未匹配待审队列（品牌×型号×吨位聚合）"
    )
    _add_common_args(unmatched)

    backfill = sub.add_parser(
        "backfill", help="历史回补：列表页发现 → 全管线 → 得率报告"
    )
    backfill.add_argument(
        "--list-url",
        action="append",
        required=True,
        dest="list_urls",
        help=(
            "发现入口 URL（可重复；按 URL 自动识别采集面）"
            "。rmfysszc 如 .../Agency/pmgg.aspx?fid=100；"
            "京东如 https://paimai.jd.com/notice/11810994；公拍网如 https://www.gpai.net/sf/"
        ),
    )
    backfill.add_argument("--start-year", type=int, default=BACKFILL_START_YEAR)
    backfill.add_argument("--end-year", type=int, default=BACKFILL_END_YEAR)
    backfill.add_argument(
        "--allow-robots-override",
        action="store_true",
        help="robots 禁止时仍抓取（会记审计日志；默认遵从 robots）",
    )
    _add_common_args(backfill)

    daemon = sub.add_parser(
        "daemon", help="增量 daemon：按周节奏重复跑一轮采集（复用 backfill）"
    )
    daemon.add_argument(
        "--list-url",
        action="append",
        required=True,
        dest="list_urls",
        help=(
            "发现入口 URL（可重复；按 URL 自动识别采集面）"
            "。rmfysszc 如 .../Agency/pmgg.aspx?fid=100；"
            "京东如 https://paimai.jd.com/notice/11810994；公拍网如 https://www.gpai.net/sf/"
        ),
    )
    daemon.add_argument(
        "--interval-seconds",
        type=float,
        default=WEEK_SECONDS,
        help=f"两轮之间补足到该周期（默认一周 {WEEK_SECONDS}s）",
    )
    daemon.add_argument(
        "--max-cycles",
        type=int,
        default=None,
        help="只跑 N 轮后退出（缺省常驻；运维手动跑一轮用 1）",
    )
    daemon.add_argument("--start-year", type=int, default=BACKFILL_START_YEAR)
    daemon.add_argument(
        "--end-year",
        type=int,
        default=None,
        help="窗口上界（缺省 = 当前年；常驻服务不应写死年份）",
    )
    daemon.add_argument(
        "--allow-robots-override",
        action="store_true",
        help="robots 禁止时仍抓取（会记审计日志；默认遵从 robots）",
    )
    _add_common_args(daemon)

    args = parser.parse_args(argv)

    try:
        if args.command == "batch":
            count = run_batch(args.fixture, args.dsn, args.search_path)
        elif args.command == "reclean":
            count = run_reclean(args.dsn, args.search_path)
        elif args.command == "unmatched":
            groups = run_unmatched(args.dsn, args.search_path)
            _print_unmatched(groups)
            return 0
        elif args.command == "backfill":
            crawler = _build_crawler(
                allow_robots_override=args.allow_robots_override,
                list_urls=args.list_urls,
            )
            count, report = run_backfill(
                args.list_urls,
                args.dsn,
                args.search_path,
                start_year=args.start_year,
                end_year=args.end_year,
                crawler=crawler,
            )
            print(report.render())
            print(crawler.log.summary())
            return 0
        elif args.command == "daemon":
            crawler = _build_crawler(
                allow_robots_override=args.allow_robots_override,
                list_urls=args.list_urls,
            )

            def _probe() -> None:
                """连一次 PG 并解析一次 AI 配置：把「每轮必失败」的配置错挡在第一轮之前。"""
                import psycopg
                from psycopg import sql as _sql

                dsn = args.dsn or os.environ.get("DATABASE_URL") or ""
                with psycopg.connect(dsn) as conn:
                    if args.search_path:
                        conn.execute(
                            _sql.SQL("SET search_path TO {}").format(
                                _sql.Identifier(args.search_path)
                            )
                        )
                    resolve_config(
                        conn, FEATURE_KEY, os.environ.get("SECRET_KEY", "")
                    )

            validate_startup(
                list_urls=args.list_urls,
                dsn_fn=lambda: args.dsn or os.environ.get("DATABASE_URL") or "",
                probe=_probe,
            )

            def _log(result):
                print(f"[daemon] {result.render()}", flush=True)

            results = run_daemon_command(
                args.list_urls,
                args.dsn,
                args.search_path,
                interval_seconds=args.interval_seconds,
                max_cycles=args.max_cycles,
                start_year=args.start_year,
                end_year=args.end_year,
                crawler=crawler,
                on_result=_log,
            )
            failed = sum(1 for r in results if r.failed)
            print(f"collector daemon: 跑完 {len(results)} 轮，失败 {failed} 轮")
            print(crawler.log.summary())
            return 0 if failed == 0 else 1
        else:
            count = run_rematch(args.dsn, args.search_path)
    except (OSError, RuntimeError, ValueError) as exc:
        print(f"collector {args.command} 失败: {exc}", file=sys.stderr)
        return 1
    except Exception as exc:  # psycopg.Error 等驱动异常：干净报错，不打 traceback
        if type(exc).__module__.startswith(("psycopg", "psycopg2")):
            print(f"collector {args.command} 失败: {exc}", file=sys.stderr)
            return 1
        raise
    print(f"collector {args.command}: 处理 {count} 行")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
