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
    # #2680：挂牌价格（与成交金额配对）。列顺序与迁移 000068 的语义一致，
    # 且必须与 _FETCH_COLUMNS / fetch_samples 解包顺序同步。
    "listing_price",
    "appraisal_price",
    "auction_date",
    "clean_status",
    "clean_version",
    "rejection_reason",
    "field_confidence",
    "extraction_version",
    "match_level",
    "match_version",
    "matched_original_range",
    "premium_anomaly",
    "residual_rate_label",
    "matched_original_price",
    "label_eligible",
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


def _jsonb(value: Any, column: str):
    """按列类型决定是否包 Jsonb——INSERT 与 UPDATE 共用，避免两处各记一份规则。"""
    if column in _JSONB_COLUMNS and value is not None:
        from psycopg.types.json import Jsonb

        return Jsonb(value)
    return value


def insert_sample(conn: "psycopg.Connection", sample: RawSample) -> int:
    """把一条 raw 样本落进 market_transactions，返回新行 id（调用方负责事务）。"""
    row = sample.to_row()
    values = [_jsonb(row[col], col) for col in _COLUMNS]
    return conn.execute(_insert_sql() + " RETURNING id", values).fetchone()[0]


def enqueue_review(
    conn: "psycopg.Connection", sample_id: int, reason: str, detail: str | None
) -> None:
    """把需人工复核的样本写进 market_sample_review_queue（调用方负责事务）。"""
    conn.execute(
        "INSERT INTO market_sample_review_queue (sample_id, reason, detail) "
        "VALUES (%s, %s, %s)",
        (sample_id, reason, detail),
    )


# 重跑清洗读回的列：原文留底 + 抽取结果 + 当前清洗状态。不含匹配/标签列（下游 #2675）。
_FETCH_COLUMNS = (
    "id", "channel", "source_url", "raw_description", "structured_specs",
    "raw_price", "raw_price_kind", "listing_price", "appraisal_price", "field_confidence",
    "clean_status", "clean_version", "rejection_reason", "premium_anomaly", "label_eligible",
)


def fetch_samples(conn: "psycopg.Connection") -> list[tuple[int, Any]]:
    """读回已入库样本（id + RawSample），供重跑清洗用；调用方负责连接/schema。"""
    from .models import RawSample

    rows = conn.execute(
        "SELECT " + ", ".join(_FETCH_COLUMNS) + " FROM market_transactions ORDER BY id"
    ).fetchall()
    result: list[tuple[int, RawSample]] = []
    for row in rows:
        (
            sid, channel, source_url, raw_description, structured_specs,
            raw_price, raw_price_kind, listing_price, appraisal_price, field_confidence,
            clean_status, clean_version, rejection_reason, premium_anomaly, label_eligible,
        ) = row
        sample = RawSample(
            channel=channel,
            source_url=source_url,
            raw_description=raw_description,
            structured_specs=structured_specs or {},
            raw_price=raw_price,
            raw_price_kind=raw_price_kind,
            listing_price=listing_price,
            appraisal_price=appraisal_price,
            field_confidence=field_confidence or {},
            clean_status=clean_status,
            clean_version=clean_version,
            rejection_reason=rejection_reason,
            premium_anomaly=premium_anomaly,
            label_eligible=label_eligible,
        )
        result.append((sid, sample))
    return result


_PRICE_CANDIDATE_COLUMNS = (
    "brand", "vehicle_type", "series", "tonnage", "config_type",
    "mast_type", "mast_height_mm", "original_price",
)


def load_price_candidates(conn: "psycopg.Connection") -> list[dict[str, Any]]:
    """从 original_prices 直读全部匹配候选（#2675）。

    与后端 eval 路径读同一张真表，但匹配语义是独立的纯函数（分层 + 中位数），
    不复用取最高价的模糊匹配。候选量级为数百行，全量读入即可。
    """
    rows = conn.execute(
        "SELECT " + ", ".join(_PRICE_CANDIDATE_COLUMNS) + " FROM original_prices"
    ).fetchall()
    return [dict(zip(_PRICE_CANDIDATE_COLUMNS, row)) for row in rows]


def update_match_result(
    conn: "psycopg.Connection", sample_id: int, matched: Any
) -> None:
    """把匹配结果写回匹配/标签列（供补原价后重跑匹配；抽取/清洗列不动）。"""
    conn.execute(
        "UPDATE market_transactions SET "
        "match_level = %s, match_version = %s, matched_original_price = %s, "
        "matched_original_range = %s, residual_rate_label = %s, "
        "field_confidence = %s WHERE id = %s",
        (
            matched.match_level,
            matched.match_version,
            matched.matched_original_price,
            _jsonb(matched.matched_original_range, "matched_original_range"),
            matched.residual_rate_label,
            _jsonb(matched.field_confidence, "field_confidence"),
            sample_id,
        ),
    )


def load_unmatched_queue(conn: "psycopg.Connection") -> list[dict[str, Any]]:
    """读未匹配待审队列聚合视图（迁移 000067），供 CLI 打印 / 运维核查（#2676）。

    样本多的组优先（人工补原价按此优先级）；外层再写一次 ORDER BY——视图内的
    ORDER BY 不保证穿透到外层查询，不重写就会变成任意顺序。
    """
    from .review import UNMATCHED_GROUP_COLUMNS

    rows = conn.execute(
        "SELECT " + ", ".join(UNMATCHED_GROUP_COLUMNS) + " FROM market_unmatched_queue "
        "ORDER BY sample_count DESC, brand ASC, model ASC, tonnage ASC NULLS LAST"
    ).fetchall()
    return [dict(zip(UNMATCHED_GROUP_COLUMNS, row)) for row in rows]


def update_clean_result(
    conn: "psycopg.Connection", sample_id: int, cleaned: Any
) -> None:
    """把重跑清洗的结果写回清洗列 + 结构字段；采集来源 channel/source_url 不动。

    原文留底 jsonb 里被剥离的身份字段与脱敏后的 body 一并写回（PIPL：留底亦不含
    买受人身份）；抽取版本、匹配与标签列均不动——清洗只负责清洗列与结构字段。
    clean_version 记录产出本行结果所用的规则版本，供后续按版本分离重跑。
    """
    conn.execute(
        "UPDATE market_transactions SET "
        "clean_status = %s, clean_version = %s, rejection_reason = %s, "
        "premium_anomaly = %s, label_eligible = %s, structured_specs = %s, "
        "raw_description = %s WHERE id = %s",
        (
            cleaned.clean_status,
            cleaned.clean_version,
            cleaned.rejection_reason,
            cleaned.premium_anomaly,
            cleaned.label_eligible,
            _jsonb(cleaned.structured_specs, "structured_specs"),
            _jsonb(cleaned.raw_description, "raw_description"),
            sample_id,
        ),
    )

