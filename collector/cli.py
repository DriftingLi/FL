"""collector batch CLI：fixture 进 → 抽取 → market_transactions 行 + 抽检队列出。

用法：
    python3 -m collector.cli batch <fixture.html> [--dsn DATABASE_URL] [--search-path SCHEMA]

`--dsn` 缺省取环境变量 DATABASE_URL（与后端 / CI 同源），`SECRET_KEY` 用于解密
ai_configs.api_key（与后端同源）。`--search-path` 用于契约测试把整条链路收窄在随机
schema；生产不传，走迁移建表的默认 schema。抽取 seam 可注入，测试用录制 fixture 打桩。
"""

from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path
from typing import Callable

from .extract.config import FEATURE_KEY, resolve_config
from .extract.llm import extract
from .extract.service import apply_extraction
from .parsers.rmfysszc import parse_announcement
from .store import enqueue_review, insert_sample


def run_batch(
    fixture_path: str,
    dsn: str | None = None,
    search_path: str = "",
    *,
    extractor: Callable | None = None,
    secret_key: str | None = None,
) -> int:
    """处理一份 fixture：解析 → 直读配置 → LLM 抽取 → 写回同一行 + 抽检入队。

    返回落库行数。extractor 为可注入 seam：测试用录制 fixture 打桩（确定性、不测
    prompt 质量）；生产不传则走真实 OpenAI 兼容 LLM 调用。
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
                sample_id = insert_sample(conn, updated)
                for reason, detail in review:
                    enqueue_review(conn, sample_id, reason, detail)
    return len(samples)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="collector")
    sub = parser.add_subparsers(dest="command", required=True)
    batch = sub.add_parser("batch", help="从单份 fixture 公告落 raw 样本行")
    batch.add_argument("fixture", help="fixture HTML 路径")
    batch.add_argument("--dsn", default=None, help="PG 连接串（缺省取 DATABASE_URL）")
    batch.add_argument(
        "--search-path", default="", help="可选：把本次写入收窄到指定 schema（契约测试用）"
    )
    args = parser.parse_args(argv)

    try:
        count = run_batch(args.fixture, args.dsn, args.search_path)
    except (OSError, RuntimeError) as exc:
        print(f"collector batch 失败: {exc}", file=sys.stderr)
        return 1
    print(f"collector batch: 写入 {count} 行")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
