"""collector CLI：fixture 进 → 抽取 → 清洗 → 匹配 → market_transactions 行 + 抽检队列出。

三段分离（spec #2671）：采集层只 fetch + 原始留底，抽取 / 清洗 / 匹配是独立的纯函数
阶段，入库只做写。清洗与匹配不依赖抓取，故可重跑：

    python3 -m collector.cli batch <fixture.html> [--dsn URL] [--search-path SCHEMA]
    python3 -m collector.cli reclean [--dsn URL] [--search-path SCHEMA]
    python3 -m collector.cli rematch [--dsn URL] [--search-path SCHEMA]

`--dsn` 缺省取环境变量 DATABASE_URL，`SECRET_KEY` 用于解密 ai_configs.api_key
（均与后端同源）。`--search-path` 用于契约测试把整条链路收窄在随机 schema。
抽取 / 清洗 / 匹配均可注入 seam：测试用录制 fixture + 纯函数打桩（确定性、不测 prompt 质量）。
"""

from __future__ import annotations

import argparse
import os
import sys
import unicodedata
from pathlib import Path
from typing import Callable

from .backfill import (
    BACKFILL_END_YEAR,
    BACKFILL_START_YEAR,
    DiscoveredAnnouncement,
    build_yield_report,
    filter_by_year,
    parse_announcement_list,
    select_new_announcements,
)
from .backfill import BASE as BASE_URL
from .clean import CLEAN_REVIEW_REASONS, clean
from .extract.config import FEATURE_KEY, resolve_config
from .extract.llm import extract
from .extract.service import apply_extraction
from .match import match
from .parsers.rmfysszc import parse_announcement
from .store import (
    enqueue_review,
    fetch_samples,
    insert_sample,
    load_price_candidates,
    load_unmatched_queue,
    update_clean_result,
    update_match_result,
)


def _process_html(
    conn,
    html: str,
    *,
    source_url: str = "",
    config,
    candidates: list[dict],
    extractor: Callable | None,
    clean_fn: Callable,
    match_fn: Callable,
    existing_samples: list,
) -> tuple[int, list]:
    """把一份公告 HTML 走完 抽取 → 清洗 → 匹配 → 落库（调用方负责连接与事务）。

    batch（单份 fixture）与 backfill（逐个回补 URL）共用这一段——回补不另写一份
    管线，否则清洗/匹配逻辑必然漂移（spec #2671 的硬约束）。

    返回 (落库行数, 本轮落库的样本列表)。后者是给回补的跨 URL 去重信号：清洗规则⑥
    （车商批量帖去重）要能看见同轮其它 URL 的样本，否则「同一车多城重发」不会被去重。
    """
    samples = parse_announcement(html, source_url)
    if not samples:
        return 0, []
    # 去重信号 = 已入库样本 + 本批已落样本；两者都由调用方跨 URL 累积传入。
    existing = list(existing_samples)
    added: list = []
    for sample in samples:
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
    if not parse_announcement(html):
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

    # ① 发现：抓列表页 → 解析 → 年份过滤。
    discovered: list[DiscoveredAnnouncement] = []
    for list_url in list_urls:
        discovered.extend(parse_announcement_list(crawler.fetch(list_url)))
    in_window = filter_by_year(discovered, start_year=start_year, end_year=end_year)

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
        stored = 0
        stored_samples: list = []
        yield_rows: list[dict] = []
        for item in new_items:
            html = crawler.fetch(item.url)  # 网络 I/O，不在事务内
            with conn.transaction():
                count, added = _process_html(
                    conn,
                    html,
                    source_url=item.url,
                    config=config,
                    candidates=candidates,
                    extractor=extractor,
                    clean_fn=clean_fn,
                    match_fn=match_fn,
                    # 跨 URL 去重信号：同轮前面所有 URL 的落库样本（车商批量帖去重⑥）。
                    existing_samples=stored_samples,
                )
            stored += count
            stored_samples.extend(added)
            yield_rows.extend(
                {"clean_status": s.clean_status, "rejection_reason": s.rejection_reason}
                for s in added
            )
        report = build_yield_report(yield_rows)
    return stored, report


def _existing_source_urls(conn) -> set[str]:
    """已入库的 source_url 集合（幂等判据的数据来源）。"""
    rows = conn.execute("SELECT DISTINCT source_url FROM market_transactions").fetchall()
    return {row[0] for row in rows}


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


def _build_crawler(*, allow_robots_override: bool, base_url: str):
    """构造生产抓取客户端：读站点 robots → 遵从判定 + 限速 + 退避。"""
    from .crawl.client import CrawlClient, load_robots, urllib_fetcher

    fetcher = urllib_fetcher()
    robots = load_robots(base_url=base_url, fetcher=fetcher)
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
        help="公告列表页 URL（可重复；如 .../Agency/pmgg.aspx?fid=100）",
    )
    backfill.add_argument("--start-year", type=int, default=BACKFILL_START_YEAR)
    backfill.add_argument("--end-year", type=int, default=BACKFILL_END_YEAR)
    backfill.add_argument(
        "--allow-robots-override",
        action="store_true",
        help="robots 禁止时仍抓取（会记审计日志；默认遵从 robots）",
    )
    _add_common_args(backfill)

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
                base_url=BASE_URL,
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
        else:
            count = run_rematch(args.dsn, args.search_path)
    except (OSError, RuntimeError) as exc:
        print(f"collector {args.command} 失败: {exc}", file=sys.stderr)
        return 1
    print(f"collector {args.command}: 处理 {count} 行")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
