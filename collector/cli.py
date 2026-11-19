"""collector CLI：fixture 进 → 抽取 → 清洗 → market_transactions 行 + 抽检队列出。

三段分离（spec #2671）：采集层只 fetch + 原始留底，抽取与清洗是独立的纯函数阶段，
入库只做写。清洗不依赖抓取，故可重跑：

    python3 -m collector.cli batch <fixture.html> [--dsn URL] [--search-path SCHEMA]
    python3 -m collector.cli reclean [--dsn URL] [--search-path SCHEMA]

`--dsn` 缺省取环境变量 DATABASE_URL，`SECRET_KEY` 用于解密 ai_configs.api_key
（均与后端同源）。`--search-path` 用于契约测试把整条链路收窄在随机 schema。
抽取 / 清洗均可注入 seam：测试用录制 fixture + 纯函数打桩（确定性、不测 prompt 质量）。
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
from .parsers.rmfysszc import parse_announcement
from .store import (
    enqueue_review,
    fetch_samples,
    insert_sample,
    update_clean_result,
)


def run_batch(
    fixture_path: str,
    dsn: str | None = None,
    search_path: str = "",
    *,
    extractor: Callable | None = None,
    secret_key: str | None = None,
    cleaner: Callable | None = None,
) -> int:
    """处理一份 fixture：解析 → 直读配置 → 抽取 → 清洗 → 写回同一行 + 抽检入队。

    返回落库行数。extractor / cleaner 为可注入 seam：测试用录制 fixture 打桩；
    生产不传则走真实 LLM 调用 + 版本化清洗规则模块。
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

    with psycopg.connect(dsn) as conn:
        if search_path:
            conn.execute(
                sql.SQL("SET search_path TO {}").format(sql.Identifier(search_path))
            )
        # 配置直读与写回同在一个连接 / 事务：要么整批样本+队列行都在，要么都不在。
        config = resolve_config(conn, FEATURE_KEY, secret_key)
        with conn.transaction():
            for sample in samples:
                extraction = extract(sample.raw_description["body"], config, extractor)
                updated, review = apply_extraction(sample, extraction)
                # 清洗是独立纯函数阶段：只吃已抽取样本 + 同批样本（去重信号）。
                cleaned, clean_review = clean_fn(updated, existing=samples)
                sample_id = insert_sample(conn, cleaned)
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

    args = parser.parse_args(argv)

    try:
        if args.command == "batch":
            count = run_batch(args.fixture, args.dsn, args.search_path)
        else:
            count = run_reclean(args.dsn, args.search_path)
    except (OSError, RuntimeError) as exc:
        print(f"collector {args.command} 失败: {exc}", file=sys.stderr)
        return 1
    print(f"collector {args.command}: 处理 {count} 行")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
