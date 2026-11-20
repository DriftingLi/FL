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
from pathlib import Path
from typing import Callable

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
    samples = parse_announcement(html)
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
            for sample in samples:
                extraction = extract(sample.raw_description["body"], config, extractor)
                updated, review = apply_extraction(sample, extraction)
                # 清洗是独立纯函数阶段：只吃已抽取样本 + 同批样本（去重信号）。
                cleaned, clean_review = clean_fn(updated, existing=samples)
                # 匹配是独立纯函数阶段：从 PG 直读候选原价，取中位数、不取最高价。
                matched = match_fn(cleaned, candidates)
                sample_id = insert_sample(conn, matched)
                for reason, detail in review + clean_review:
                    enqueue_review(conn, sample_id, reason, detail)
    return len(samples)


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


def _print_unmatched(groups: list[dict]) -> None:
    """把待审队列聚合组打印成对齐表格（品牌/型号/吨位/样本数/价格区间）。"""
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
    widths = [max(len(header[i]), *(len(r[i]) for r in rows)) for i in range(len(header))]
    print("  ".join(h.ljust(widths[i]) for i, h in enumerate(header)))
    for row in rows:
        print("  ".join(c.ljust(widths[i]) for i, c in enumerate(row)))
    print(f"\n共 {len(groups)} 组、{sum(g['sample_count'] for g in groups)} 条待审样本")


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
        else:
            count = run_rematch(args.dsn, args.search_path)
    except (OSError, RuntimeError) as exc:
        print(f"collector {args.command} 失败: {exc}", file=sys.stderr)
        return 1
    print(f"collector {args.command}: 处理 {count} 行")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
