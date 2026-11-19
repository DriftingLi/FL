"""market_transactions 写入层。

psycopg 在需要真正连接时才 import：本机无 DATABASE_URL 跑单测时不必装驱动。
jsonb 列用 psycopg.types.json.Jsonb 包一层；dict 列退回 JSON 字符串由服务端 jsonb CAST。
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .models import RawSample

if TYPE_CHECKING:
    import psycopg

# INSERT 列顺序（与迁移 000063 保持一致）。SQL 生成与取值组装共用这一处，
# 新增字段只需改这里 + RawSample + 迁移，不再两处逐字同步。
_COLUMNS = (
    "channel",
    "source_url",
    "raw_description",
    "structured_specs",
    "raw_price",
    "raw_price_kind",
    "appraisal_price",
    "auction_date",
    "clean_status",
    "rejection_reason",
    "field_confidence",
    "extraction_version",
    "match_level",
    "matched_original_range",
    "premium_anomaly",
    "residual_rate_label",
    "matched_original_price",
)

# 由 Python dict 写入、列类型为 jsonb 的列。与迁移 000063 保持一致。
_JSONB_COLUMNS = (
    "raw_description",
    "structured_specs",
    "field_confidence",
    "matched_original_range",
)


def _insert_sql() -> str:
    placeholders = ", ".join("%s" for _ in _COLUMNS)
    return (
        "INSERT INTO market_transactions ("
        + ", ".join(_COLUMNS)
        + f") VALUES ({placeholders})"
    )


def insert_sample(conn: "psycopg.Connection", sample: RawSample) -> None:
    """把一条 raw 样本落进 market_transactions（调用方负责事务）。"""
    from psycopg.types.json import Jsonb

    row = sample.to_row()
    values: list[Any] = []
    for col in _COLUMNS:
        value = row[col]
        if col in _JSONB_COLUMNS and value is not None:
            value = Jsonb(value)
        values.append(value)
    conn.execute(_insert_sql(), values)
